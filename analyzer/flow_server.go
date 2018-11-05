/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package analyzer

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/storage"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
	ws "github.com/skydive-project/skydive/websocket"
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
	Serve(ch chan *flow.Flow, quit chan struct{}, wg *sync.WaitGroup)
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
	ch                     chan *flow.Flow
	timeOfLastLostFlowsLog time.Time
	numOfLostFlows         int
	maxFlowBufferSize      int
	auth                   shttp.AuthenticationBackend
}

// FlowServer describes a flow server
type FlowServer struct {
	storage            storage.Storage
	conn               FlowServerConn
	state              int64
	wgServer           sync.WaitGroup
	bulkInsert         int
	bulkInsertDeadline time.Duration
	ch                 chan *flow.Flow
	quit               chan struct{}
	auth               shttp.AuthenticationBackend
}

// OnMessage event
func (c *FlowServerWebSocketConn) OnMessage(client ws.Speaker, m ws.Message) {
	f, err := flow.FromData(m.Bytes(client.GetClientProtocol()))
	if err != nil {
		logging.GetLogger().Errorf("Error while parsing flow: %s", err.Error())
		return
	}
	logging.GetLogger().Debugf("New flow from Websocket connection: %+v", f)
	if len(c.ch) >= c.maxFlowBufferSize {
		c.numOfLostFlows++
		if c.timeOfLastLostFlowsLog.IsZero() ||
			(time.Now().Sub(c.timeOfLastLostFlowsLog) >= time.Second) {
			logging.GetLogger().Errorf("Buffer overflow - too many flow updates, removing and not storing flows: %d", c.numOfLostFlows)
			c.timeOfLastLostFlowsLog = time.Now()
			c.numOfLostFlows = 0
		}
		return
	}
	c.ch <- f
}

// Serve starts a WebSocket flow server
func (c *FlowServerWebSocketConn) Serve(ch chan *flow.Flow, quit chan struct{}, wg *sync.WaitGroup) {
	c.ch = ch
	server := config.NewWSServer(c.server, "/ws/flow", c.auth)
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
func (c *FlowServerUDPConn) Serve(ch chan *flow.Flow, quit chan struct{}, wg *sync.WaitGroup) {
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
					logging.GetLogger().Errorf("Error while reading: %s", err.Error())
				}

				f, err := flow.FromData(data[0:n])
				if err != nil {
					logging.GetLogger().Errorf("Error while parsing flow: %s", err.Error())
					continue
				}

				logging.GetLogger().Debugf("New flow from UDP connection: %+v", f)
				if len(ch) >= c.maxFlowBufferSize {
					c.numOfLostFlows++
					if c.timeOfLastLostFlowsLog.IsZero() ||
						(time.Now().Sub(c.timeOfLastLostFlowsLog) >= time.Second) {
						logging.GetLogger().Errorf("Buffer overflow - too many flow updates, removing and not storing flows: %d", c.numOfLostFlows)
						c.timeOfLastLostFlowsLog = time.Now()
						c.numOfLostFlows = 0
					}
					return
				}
				ch <- f
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

func (s *FlowServer) storeFlows(flows []*flow.Flow) {
	if s.storage != nil && len(flows) > 0 {
		s.storage.StoreFlows(flows)

		logging.GetLogger().Debugf("%d flows stored", len(flows))
	}
}

// Start the flow server
func (s *FlowServer) Start() {
	atomic.StoreInt64(&s.state, common.RunningState)
	s.wgServer.Add(1)

	s.conn.Serve(s.ch, s.quit, &s.wgServer)
	go func() {
		defer s.wgServer.Done()

		dlTimer := time.NewTicker(s.bulkInsertDeadline)
		defer dlTimer.Stop()

		var flowBuffer []*flow.Flow
		defer s.storeFlows(flowBuffer)

		for {
			select {
			case <-s.quit:
				return
			case <-dlTimer.C:
				s.storeFlows(flowBuffer)
				flowBuffer = flowBuffer[:0]
			case f := <-s.ch:
				flowBuffer = append(flowBuffer, f)
				if len(flowBuffer) >= s.bulkInsert {
					s.storeFlows(flowBuffer)
					flowBuffer = flowBuffer[:0]
				}
			}
		}
	}()
}

// Stop the server
func (s *FlowServer) Stop() {
	if atomic.CompareAndSwapInt64(&s.state, common.RunningState, common.StoppingState) {
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
	s.ch = make(chan *flow.Flow, max(flowsMax, s.bulkInsert*2))

	return nil
}

// NewFlowServer creates a new flow server listening at address/port, based on configuration
func NewFlowServer(s *shttp.Server, g *graph.Graph, store storage.Storage, probe *probe.Bundle, auth shttp.AuthenticationBackend) (*FlowServer, error) {
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
		storage: store,
		conn:    conn,
		quit:    make(chan struct{}, 2),
		auth:    auth,
	}
	err = fs.setupBulkConfigFromBackend()
	if err != nil {
		return nil, err
	}
	return fs, nil
}
