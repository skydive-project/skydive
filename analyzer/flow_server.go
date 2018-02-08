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
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pmylund/go-cache"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/enhancers"
	"github.com/skydive-project/skydive/flow/storage"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

// FlowServerConn describes a flow server connection
type FlowServerConn interface {
	Serve(ch chan *flow.Flow, quit chan struct{}, wg *sync.WaitGroup)
}

// FlowServerConn describes a UDP flow server connection
type FlowServerUDPConn struct {
	conn *net.UDPConn
}

// FlowServerConn describes a WebSocket flow server connection
type FlowServerWebSocketConn struct {
	shttp.DefaultWSSpeakerEventHandler
	server *shttp.Server
	ch     chan *flow.Flow
}

// FlowServer describes a flow server with pipeline enhancers mechanism
type FlowServer struct {
	storage                storage.Storage
	enhancerPipeline       *flow.EnhancerPipeline
	enhancerPipelineConfig *flow.EnhancerPipelineConfig
	conn                   FlowServerConn
	state                  int64
	wgServer               sync.WaitGroup
	bulkInsert             int
	bulkInsertDeadline     time.Duration
	ch                     chan *flow.Flow
	quit                   chan struct{}
}

// OnMessage event
func (c *FlowServerWebSocketConn) OnMessage(client shttp.WSSpeaker, m shttp.WSMessage) {
	f, err := flow.FromData(m.Bytes())
	if err != nil {
		logging.GetLogger().Errorf("Error while parsing flow: %s", err.Error())
		return
	}
	logging.GetLogger().Debugf("New flow from Websocket connection: %+v", f)
	c.ch <- f
}

// Start a WebSocket flow server
func (c *FlowServerWebSocketConn) Serve(ch chan *flow.Flow, quit chan struct{}, wg *sync.WaitGroup) {
	c.ch = ch
	server := shttp.NewWSServer(c.server, "/ws/flow")
	server.AddEventHandler(c)
	go func() {
		server.Start()
		<-quit
		server.Stop()
	}()
}

// NewFlowServerWebSocketConn returns a new WebSocket flow server
func NewFlowServerWebSocketConn(server *shttp.Server) (*FlowServerWebSocketConn, error) {
	return &FlowServerWebSocketConn{server: server}, nil
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
	return &FlowServerUDPConn{conn: conn}, err
}

func (s *FlowServer) storeFlows(flows []*flow.Flow) {
	if s.storage != nil && len(flows) > 0 {
		s.enhancerPipeline.Enhance(s.enhancerPipelineConfig, flows)
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

// NewFlowServer creates a new flow server listening at address/port, based on configuration
func NewFlowServer(s *shttp.Server, g *graph.Graph, store storage.Storage, probe *probe.ProbeBundle) (*FlowServer, error) {
	cache := cache.New(time.Duration(600)*time.Second, time.Duration(600)*time.Second)
	pipeline := flow.NewEnhancerPipeline(enhancers.NewGraphFlowEnhancer(g, cache))

	// check that the neutron probe is loaded if so add the neutron flow enhancer
	if probe.GetProbe("neutron") != nil {
		pipeline.AddEnhancer(enhancers.NewNeutronFlowEnhancer(g, cache))
	}

	bulk := config.GetInt("analyzer.storage.bulk_insert")
	bulkDeadLine := config.GetInt("analyzer.storage.bulk_insert_deadline")
	if bulkDeadLine < 1 {
		return nil, fmt.Errorf("bulk_insert_deadline has to be >= 1")
	}

	var err error
	var conn FlowServerConn
	protocol := strings.ToLower(config.GetString("flow.protocol"))
	switch protocol {
	case "udp":
		conn, err = NewFlowServerUDPConn(s.Addr, s.Port)
	case "websocket":
		conn, err = NewFlowServerWebSocketConn(s)
	default:
		err = fmt.Errorf("Invalid protocol %s", protocol)
	}

	if err != nil {
		return nil, err
	}

	return &FlowServer{
		storage:                store,
		enhancerPipeline:       pipeline,
		enhancerPipelineConfig: flow.NewEnhancerPipelineConfig(),
		bulkInsert:             bulk,
		bulkInsertDeadline:     time.Duration(time.Duration(bulkDeadLine) * time.Second),
		conn:                   conn,
		quit:                   make(chan struct{}, 2),
		ch:                     make(chan *flow.Flow, bulk*2),
	}, nil
}
