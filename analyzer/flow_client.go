/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

// FlowClientPool describes a flow client pool.
type FlowClientPool struct {
	sync.RWMutex
	shttp.DefaultWSSpeakerEventHandler
	flowClients []*FlowClient
}

// FlowClient describes a flow client connection
type FlowClient struct {
	addr           string
	port           int
	flowClientConn FlowClientConn
}

// FlowClientConn is the interface to be implemented by the flow clients
type FlowClientConn interface {
	Connect() error
	Close() error
	Send(data []byte) error
}

// FlowClientUDPConn describes UDP client connection
type FlowClientUDPConn struct {
	addr *net.UDPAddr
	conn *net.UDPConn
}

// FlowClientWebSocketConn describes WebSocket client connection
type FlowClientWebSocketConn struct {
	shttp.DefaultWSSpeakerEventHandler
	url      *url.URL
	wsClient *shttp.WSClient
}

// Close the connection
func (c *FlowClientUDPConn) Close() error {
	return c.conn.Close()
}

// Connect to the UDP flow server
func (c *FlowClientUDPConn) Connect() (err error) {
	logging.GetLogger().Debugf("UDP client dialup done for %s", c.addr.String())
	c.conn, err = net.DialUDP("udp", nil, c.addr)
	return err
}

// Send data over the wire
func (c *FlowClientUDPConn) Send(data []byte) error {
	_, err := c.conn.Write(data)
	return err
}

// NewFlowClientUDPConn returns a new UDP flow client
func NewFlowClientUDPConn(addr string, port int) (*FlowClientUDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}

	return &FlowClientUDPConn{addr: udpAddr}, nil
}

// Close the connection
func (c *FlowClientWebSocketConn) Close() error {
	c.wsClient.Disconnect()
	return nil
}

// Connect to the WebSocket flow server
func (c *FlowClientWebSocketConn) Connect() error {
	authOptions := NewAnalyzerAuthenticationOpts()
	authAddr := common.NormalizeAddrForURL(c.url.Hostname())
	authPort, _ := strconv.Atoi(c.url.Port())
	authClient := shttp.NewAuthenticationClient(config.GetURL("http", authAddr, authPort, ""), authOptions)
	c.wsClient = shttp.NewWSClientFromConfig(common.AgentService, c.url, authClient, nil)
	c.wsClient.Connect()
	c.wsClient.AddEventHandler(c)

	return nil
}

// Send data over the wire
func (c *FlowClientWebSocketConn) Send(data []byte) error {
	c.wsClient.SendRaw(data)
	return nil
}

// NewFlowClientWebSocketConn returns a new WebSocket flow client
func NewFlowClientWebSocketConn(url *url.URL) (*FlowClientWebSocketConn, error) {
	return &FlowClientWebSocketConn{url: url}, nil
}

func (c *FlowClient) connect() {
	if err := c.flowClientConn.Connect(); err != nil {
		logging.GetLogger().Errorf("Connection error to %s:%d : %s", c.addr, c.port, err.Error())
		time.Sleep(200 * time.Millisecond)
	}
}

func (c *FlowClient) close() {
	if err := c.flowClientConn.Close(); err != nil {
		logging.GetLogger().Errorf("Error while closing flow connection: %s", err.Error())
	}
}

// SendFlow sends a flow to the server
func (c *FlowClient) SendFlow(f *flow.Flow) error {
	data, err := f.GetData()
	if err != nil {
		return err
	}

retry:
	err = c.flowClientConn.Send(data)
	if err != nil {
		logging.GetLogger().Errorf("flows connection to analyzer error %s : try to reconnect", err.Error())
		c.close()
		c.connect()
		goto retry
	}

	return nil
}

// SendFlows sends flows to the server
func (c *FlowClient) SendFlows(flows []*flow.Flow) {
	for _, flow := range flows {
		err := c.SendFlow(flow)
		if err != nil {
			logging.GetLogger().Errorf("Unable to send flow: %s", err.Error())
		}
	}
}

// NewFlowClient creates a flow client and creates a new connection to the server
func NewFlowClient(addr string, port int) (*FlowClient, error) {
	var (
		connection FlowClientConn
		err        error
	)
	protocol := strings.ToLower(config.GetString("flow.protocol"))
	switch protocol {
	case "udp":
		connection, err = NewFlowClientUDPConn(common.NormalizeAddrForURL(addr), port)
	case "websocket":
		connection, err = NewFlowClientWebSocketConn(config.GetURL("ws", common.NormalizeAddrForURL(addr), port, "/ws/flow"))
	default:
		return nil, fmt.Errorf("Invalid protocol %s", protocol)
	}

	if err != nil {
		return nil, err
	}

	fc := &FlowClient{addr: addr, port: port, flowClientConn: connection}
	fc.connect()

	return fc, nil
}

// OnConnected websocket event handler
func (p *FlowClientPool) OnConnected(c shttp.WSSpeaker) {
	p.Lock()
	defer p.Unlock()

	addr, port := c.GetAddrPort()
	for i, fc := range p.flowClients {
		if fc.addr == addr && fc.port == port {
			logging.GetLogger().Warningf("Got a connected event on already connected client: %s:%d", addr, port)
			fc.close()

			p.flowClients = append(p.flowClients[:i], p.flowClients[i+1:]...)
		}
	}

	flowClient, err := NewFlowClient(addr, port)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}

	p.flowClients = append(p.flowClients, flowClient)
}

// OnDisconnected websocket event handler
func (p *FlowClientPool) OnDisconnected(c shttp.WSSpeaker) {
	p.Lock()
	defer p.Unlock()

	addr, port := c.GetAddrPort()
	for i, fc := range p.flowClients {
		if fc.addr == addr && fc.port == port {
			fc.close()

			p.flowClients = append(p.flowClients[:i], p.flowClients[i+1:]...)
		}
	}
}

// SendFlows sends flows using a random connection
func (p *FlowClientPool) SendFlows(flows []*flow.Flow) {
	p.RLock()
	defer p.RUnlock()

	if len(p.flowClients) == 0 {
		return
	}

	fc := p.flowClients[rand.Intn(len(p.flowClients))]
	fc.SendFlows(flows)
}

// Close all connections
func (p *FlowClientPool) Close() {
	for _, fc := range p.flowClients {
		fc.close()
	}
}

// NewFlowClientPool returns a new FlowClientPool using the websocket connections
// to maintain the pool of client up to date according to the websocket connections
// status.
func NewFlowClientPool(pool shttp.WSSpeakerPool) *FlowClientPool {
	p := &FlowClientPool{
		flowClients: make([]*FlowClient, 0),
	}
	pool.AddEventHandler(p)
	return p
}
