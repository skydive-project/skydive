/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package server

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/storage"
	"github.com/skydive-project/skydive/graffiti/graph"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
)

const (
	// FlowBulkInsertDefault maximum number of flows aggregated between two data store inserts
	FlowBulkInsertDefault int = 100

	// FlowBulkInsertDeadlineDefault deadline of each bulk insert in second
	FlowBulkInsertDeadlineDefault int = 5

	// FlowBulkMaxDelayDefault delay between two bulk
	FlowBulkMaxDelayDefault int = 5
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FlowServerConn describes a flow server connection
type FlowServerConn interface {
	Serve(flowChan chan *flow.Flow, statsChan chan *flow.Stats, quit chan struct{}, wg *sync.WaitGroup)
}

// FlowServerUDPConn describes a UDP flow server connection
type FlowServerUDPConn struct {
	conn                   *net.UDPConn
	timeOfLastLostFlowsLog time.Time
	numOfLostFlows         int
	maxFlowBufferSize      int
}

// FlowServerWebSocketConn describes a WebSocket flow server connection
type FlowServerWebSocketConn struct {
	ws.DefaultSpeakerEventHandler
	server                 *shttp.Server
	flowChan               chan *flow.Flow
	statsChan              chan *flow.Stats
	timeOfLastLostFlowsLog time.Time
	numOfLostFlows         int
	maxFlowBufferSize      int
	auth                   shttp.AuthenticationBackend
}

// FlowServer describes a flow server
type FlowServer struct {
	storage            storage.Storage
	conn               FlowServerConn
	state              common.ServiceState
	wgServer           sync.WaitGroup
	bulkInsert         int
	bulkInsertDeadline time.Duration
	flowChan           chan *flow.Flow
	statsChan          chan *flow.Stats
	quit               chan struct{}
	auth               shttp.AuthenticationBackend
	subscriberEndpoint *FlowSubscriberEndpoint
}

// OnMessage event
func (c *FlowServerWebSocketConn) OnMessage(client ws.Speaker, m ws.Message) {
	// rawmessage at this point
	b, _ := m.Bytes(ws.RawProtocol)

	var msg flow.Message
	if err := msg.Unmarshal(b); err != nil {
		logging.GetLogger().Errorf("Error while parsing flow: %s", err)
		return
	}

	logging.GetLogger().Debugf("New flow message from Websocket connection: %+v", msg)

	// TODO(safchain) handle multiple type of message
	for _, f := range msg.Flows {
		if len(c.flowChan) >= c.maxFlowBufferSize {
			c.numOfLostFlows++
			if c.timeOfLastLostFlowsLog.IsZero() ||
				(time.Now().Sub(c.timeOfLastLostFlowsLog) >= time.Second) {
				logging.GetLogger().Errorf("Buffer overflow - too many flow updates, removing and not storing flows: %d", c.numOfLostFlows)
				c.timeOfLastLostFlowsLog = time.Now()
				c.numOfLostFlows = 0
			}
			return
		}

		c.flowChan <- f
	}

	if msg.Stats != nil {
		c.statsChan <- msg.Stats
	}
}

// Serve starts a WebSocket flow server
func (c *FlowServerWebSocketConn) Serve(flowChan chan *flow.Flow, statsChan chan *flow.Stats, quit chan struct{}, wg *sync.WaitGroup) {
	c.flowChan = flowChan
	c.statsChan = statsChan

	server := config.NewWSServer(c.server, "/ws/agent/flow", c.auth)
	server.AddEventHandler(c)
	go func() {
		server.Start()
		<-quit
		server.Stop()
	}()
}

// NewFlowServerWebSocketConn returns a new WebSocket flow server
func NewFlowServerWebSocketConn(server *shttp.Server, auth shttp.AuthenticationBackend) (*FlowServerWebSocketConn, error) {
	flowsMax := config.GetConfig().GetInt("analyzer.flow.max_buffer_size")
	return &FlowServerWebSocketConn{server: server, maxFlowBufferSize: flowsMax, auth: auth}, nil
}

// Serve UDP connections
func (c *FlowServerUDPConn) Serve(flowChan chan *flow.Flow, statsChan chan *flow.Stats, quit chan struct{}, wg *sync.WaitGroup) {
	go func() {
		defer wg.Done()
		// each flow can be HeaderSize * RawPackets + flow size (~500)
		data := make([]byte, flow.MaxCaptureLength*flow.MaxRawPacketLimit+flow.DefaultProtobufFlowSize)
		for {
			select {
			case <-quit:
				return
			default:
				n, err := c.conn.Read(data)
				if err != nil {
					if netErr, ok := err.(*net.OpError); ok {
						if netErr.Timeout() {
							c.conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
							continue
						}
					}
					logging.GetLogger().Errorf("Error while reading: %s", err)
				}

				var msg flow.Message
				if err := msg.Unmarshal(data[0:n]); err != nil {
					logging.GetLogger().Errorf("Error while parsing flow: %s", err)
					continue
				}

				logging.GetLogger().Debugf("New flow message from UDP connection: %+v", msg)

				for _, f := range msg.Flows {
					if len(flowChan) >= c.maxFlowBufferSize {
						c.numOfLostFlows++
						if c.timeOfLastLostFlowsLog.IsZero() ||
							(time.Now().Sub(c.timeOfLastLostFlowsLog) >= time.Second) {
							logging.GetLogger().Errorf("Buffer overflow - too many flow updates, removing and not storing flows: %d", c.numOfLostFlows)
							c.timeOfLastLostFlowsLog = time.Now()
							c.numOfLostFlows = 0
						}
						return
					}
					flowChan <- f
				}

				if msg.Stats != nil {
					statsChan <- msg.Stats
				}
			}
		}
	}()
}

// NewFlowServerUDPConn return a new UDP flow server
func NewFlowServerUDPConn(addr string, port int) (*FlowServerUDPConn, error) {
	host := addr + ":" + strconv.FormatInt(int64(port), 10)
	udpAddr, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	logging.GetLogger().Info("Analyzer listen agents on UDP socket")
	flowsMax := config.GetConfig().GetInt("analyzer.flow.max_buffer_size")
	return &FlowServerUDPConn{conn: conn, maxFlowBufferSize: flowsMax}, err
}

func (s *FlowServer) handleFlows(flows []*flow.Flow) {
	if len(flows) > 0 {
		if s.storage != nil {
			if err := s.storage.StoreFlows(flows); err != nil {
				logging.GetLogger().Error(err)
			} else {
				logging.GetLogger().Debugf("%d flows stored", len(flows))
			}
		}

		s.subscriberEndpoint.SendFlows(flows)
	}
}

func (s *FlowServer) handleStats(stats *flow.Stats) {
	s.subscriberEndpoint.SendStats(stats)
}

// Start the flow server
func (s *FlowServer) Start() {
	s.state.Store(common.RunningState)
	s.wgServer.Add(1)

	s.conn.Serve(s.flowChan, s.statsChan, s.quit, &s.wgServer)
	go func() {
		defer s.wgServer.Done()

		dlTimer := time.NewTicker(s.bulkInsertDeadline)
		defer dlTimer.Stop()

		var flows []*flow.Flow
		defer s.handleFlows(flows)

		for {
			select {
			case <-s.quit:
				return
			case <-dlTimer.C:
				s.handleFlows(flows)
				flows = flows[:0]
			case f := <-s.flowChan:
				flows = append(flows, f)
				if len(flows) >= s.bulkInsert {
					s.handleFlows(flows)
					flows = flows[:0]
				}
			case stats := <-s.statsChan:
				s.handleStats(stats)
			}
		}
	}()
}

// Stop the server
func (s *FlowServer) Stop() {
	if s.state.CompareAndSwap(common.RunningState, common.StoppingState) {
		s.quit <- struct{}{}
		s.quit <- struct{}{}
		s.wgServer.Wait()
	}
}

func (s *FlowServer) setupBulkConfigFromBackend() error {
	s.bulkInsert = FlowBulkInsertDefault
	s.bulkInsertDeadline = time.Duration(FlowBulkInsertDeadlineDefault) * time.Second

	storage := fmt.Sprintf("storage.%s.", config.GetString("analyzer.flow.backend"))
	if config.IsSet(storage + "driver") {
		bulkMaxDelay := config.GetInt(storage + "bulk_maxdelay")
		if bulkMaxDelay < 0 {
			return errors.New("bulk_maxdelay must be positive values")
		}
		if bulkMaxDelay == 0 {
			bulkMaxDelay = FlowBulkMaxDelayDefault
		}
		s.bulkInsertDeadline = time.Duration(bulkMaxDelay) * time.Second
	}

	flowsMax := config.GetConfig().GetInt("analyzer.flow.max_buffer_size")

	s.flowChan = make(chan *flow.Flow, max(flowsMax, s.bulkInsert*2))
	s.statsChan = make(chan *flow.Stats)

	return nil
}

// NewFlowServer creates a new flow server listening at address/port, based on configuration
func NewFlowServer(s *shttp.Server, g *graph.Graph, store storage.Storage, endpoint *FlowSubscriberEndpoint, probe *probe.Bundle, auth shttp.AuthenticationBackend) (*FlowServer, error) {
	var conn FlowServerConn
	protocol := strings.ToLower(config.GetString("flow.protocol"))

	var err error
	switch protocol {
	case "udp":
		conn, err = NewFlowServerUDPConn(s.Addr, s.Port)
	case "websocket":
		conn, err = NewFlowServerWebSocketConn(s, auth)
	default:
		err = fmt.Errorf("Invalid protocol %s", protocol)
	}

	if err != nil {
		return nil, err
	}

	fs := &FlowServer{
		storage:            store,
		conn:               conn,
		quit:               make(chan struct{}, 2),
		auth:               auth,
		subscriberEndpoint: endpoint,
	}
	err = fs.setupBulkConfigFromBackend()
	if err != nil {
		return nil, err
	}
	return fs, nil
}
