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
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/websocket"
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
	server *shttp.Server
}

// FlowServer describes a flow server with pipeline enhancers mechanism
type FlowServer struct {
	Storage          storage.Storage
	EnhancerPipeline *flow.EnhancerPipeline
	Server           *shttp.Server
	conn             FlowServerConn
	state            int64
	wgServer         sync.WaitGroup
	wgFlowsHandlers  sync.WaitGroup
	bulkInsert       int
	bulkDeadline     int
	ch               chan *flow.Flow
	quit             chan struct{}
}

// Serve WebSocket connection and push flows to chan
func (c *FlowServerWebSocketConn) Serve(ch chan *flow.Flow, quit chan struct{}, wg *sync.WaitGroup) {
	serveMessages := func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
		pongWait := time.Duration(config.GetConfig().GetInt("ws_pong_timeout")) * time.Second
		pingPeriod := (pongWait * 8) / 10

		conn, err := websocket.Upgrade(w, &r.Request, nil, 1024, 1024)
		if err != nil {
			return
		}

		wg.Add(2)

		go func() {
			ticker := time.NewTicker(pingPeriod)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
						logging.GetLogger().Warningf("Error while sending ping to the websocket: %s", err.Error())
					}
				case <-quit:
					conn.Close()
					wg.Done()
					return
				}
			}
		}()

		go func() {
			// conn.SetReadLimit(maxMessageSize)
			conn.SetReadDeadline(time.Now().Add(pongWait))
			conn.SetPongHandler(func(string) error {
				conn.SetReadDeadline(time.Now().Add(pongWait))
				return nil
			})

			for {
				_, m, err := conn.ReadMessage()
				if err != nil {
					if e, ok := err.(*websocket.CloseError); ok && e.Code == websocket.CloseAbnormalClosure {
						logging.GetLogger().Infof("Read on closed connection from %s: %s", r.Host, err.Error())
					} else {
						logging.GetLogger().Errorf("Error while reading websocket from %s: %s", r.Host, err.Error())
					}
					break
				}
				f, err := flow.FromData(m)
				if err != nil {
					logging.GetLogger().Errorf("Error while parsing flow: %s", err.Error())
					continue
				}
				ch <- f
			}

			wg.Done()
		}()
	}

	c.server.HandleFunc("/ws/flow", serveMessages)
}

func NewFlowServerWebSocketConn(server *shttp.Server) (*FlowServerWebSocketConn, error) {
	return &FlowServerWebSocketConn{server: server}, nil
}

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

				ch <- f
			}
		}
	}()
}

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
	if s.Storage != nil && len(flows) > 0 {
		s.EnhancerPipeline.Enhance(flows)
		s.Storage.StoreFlows(flows)

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

		dlTimer := time.NewTicker(time.Duration(10) * time.Second)
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

	bulk := config.GetConfig().GetInt("analyzer.storage.bulk_insert")
	deadline := config.GetConfig().GetInt("analyzer.storage.bulk_insert_deadline")

	var err error
	var conn FlowServerConn
	protocol := strings.ToLower(config.GetConfig().GetString("flow.protocol"))
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
		Server:           s,
		Storage:          store,
		EnhancerPipeline: pipeline,
		bulkInsert:       bulk,
		bulkDeadline:     deadline,
		conn:             conn,
		quit:             make(chan struct{}, 1),
		ch:               make(chan *flow.Flow, 1000),
	}, nil
}
