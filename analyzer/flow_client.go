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
	"errors"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

// FlowClientPool describes a flow client pool.
type FlowClientPool struct {
	sync.RWMutex
	shttp.DefaultWSClientEventHandler
	flowClients []*FlowClient
}

// FlowClient descibes a flow client connection
type FlowClient struct {
	Addr string
	Port int

	connection *FlowClientConn
}

func (c *FlowClient) connect() {
	strAddr := c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10)
	srv, err := net.ResolveUDPAddr("udp", strAddr)
	if err != nil {
		logging.GetLogger().Errorf("Can't resolv address to %s", strAddr)
		time.Sleep(200 * time.Millisecond)
		return
	}
	connection, err := NewFlowClientConn(srv)
	if err != nil {
		logging.GetLogger().Errorf("Connection error to %s : %s", strAddr, err.Error())
		time.Sleep(200 * time.Millisecond)
		return
	}
	c.connection = connection
}

func (c *FlowClient) close() {
	if c.connection != nil {
		c.connection.Close()
	}
}

// SendFlow sends a flow to the server
func (c *FlowClient) SendFlow(f *flow.Flow) error {
	if c.connection == nil {
		return errors.New("Not connected")
	}

	data, err := f.GetData()
	if err != nil {
		return err
	}

retry:
	_, err = c.connection.Write(data)
	if err != nil {
		logging.GetLogger().Errorf("flows connection to analyzer error %s : try to reconnect", err.Error())
		c.connection.Close()
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
func NewFlowClient(addr string, port int) *FlowClient {
	FlowClient := &FlowClient{Addr: addr, Port: port}
	FlowClient.connect()
	return FlowClient
}

// OnConnected websocket event handler
func (p *FlowClientPool) OnConnected(c *shttp.WSAsyncClient) {
	p.Lock()
	defer p.Unlock()

	for i, fc := range p.flowClients {
		if fc.Addr == c.Addr && fc.Port == c.Port {
			logging.GetLogger().Warningf("Got a connected event on already connected client: %s:%d", c.Addr, c.Port)
			fc.close()

			p.flowClients = append(p.flowClients[:i], p.flowClients[i+1:]...)
		}
	}

	p.flowClients = append(p.flowClients, NewFlowClient(c.Addr, c.Port))
}

// OnDisconnected websocket event handler
func (p *FlowClientPool) OnDisconnected(c *shttp.WSAsyncClient) {
	p.Lock()
	defer p.Unlock()

	for i, fc := range p.flowClients {
		if fc.Addr == c.Addr && fc.Port == c.Port {
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
func NewFlowClientPool(wspool *shttp.WSAsyncClientPool) *FlowClientPool {
	p := &FlowClientPool{
		flowClients: make([]*FlowClient, 0),
	}

	wspool.AddEventHandler(p, []string{})

	return p
}
