/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package client

import (
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/logging"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// FlowClientPool describes a flow client pool.
type FlowClientPool struct {
	insanelock.RWMutex
	ws.DefaultSpeakerEventHandler
	flowClients []*FlowClient
	wsOpts      *ws.ClientOpts
}

// FlowClient describes a flow client connection
type FlowClient struct {
	addr           string
	port           int
	protocol       string
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
	ws.DefaultSpeakerEventHandler
	url      *url.URL
	wsClient *ws.Client
	opts     *ws.ClientOpts
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
	c.wsClient.Stop()
	return nil
}

// Connect to the WebSocket flow server
func (c *FlowClientWebSocketConn) Connect() (err error) {
	if c.wsClient, err = config.NewWSClient(config.AgentService, c.url, *c.opts); err != nil {
		return err
	}

	c.wsClient.Start()
	c.wsClient.AddEventHandler(c)

	return nil
}

// Send data over the wire
func (c *FlowClientWebSocketConn) Send(data []byte) error {
	c.wsClient.SendRaw(data)
	return nil
}

// NewFlowClientWebSocketConn returns a new WebSocket flow client
func NewFlowClientWebSocketConn(url *url.URL, wsOpts *ws.ClientOpts) *FlowClientWebSocketConn {
	return &FlowClientWebSocketConn{url: url, opts: wsOpts}
}

func (c *FlowClient) connect() {
	if err := c.flowClientConn.Connect(); err != nil {
		logging.GetLogger().Errorf("Connection error to %s:%d : %s", c.addr, c.port, err)
		time.Sleep(200 * time.Millisecond)
	}
}

func (c *FlowClient) close() {
	if err := c.flowClientConn.Close(); err != nil {
		logging.GetLogger().Errorf("Error while closing flow connection: %s", err)
	}
}

// SendMessage sends a flow to the server
func (c *FlowClient) SendMessage(m *flow.Message) error {
	data, err := m.Marshal()
	if err != nil {
		return err
	}

retry:
	err = c.flowClientConn.Send(data)
	if err != nil {
		logging.GetLogger().Errorf("flows connection to analyzer error %s : try to reconnect", err)
		c.close()
		c.connect()
		goto retry
	}

	return nil
}

func (c *FlowClient) sendFlows(flows []*flow.Flow) {
	// NOTE: set it to 1 for udp to ensure that the flow can be sent
	// even with rawpackets. Set it a bit bigger for websocket to
	// improve performances.
	bulkSize := 1
	if c.protocol == "websocket" {
		bulkSize = 10
	}

	var msg flow.Message
	for i := 0; i < len(flows); i += bulkSize {
		e := i + bulkSize

		if e > len(flows) {
			e = len(flows)
		}

		msg.Flows = flows[i:e]

		if err := c.SendMessage(&msg); err != nil {
			logging.GetLogger().Errorf("Unable to send flow: %s", err)
		}
	}
}

func (c *FlowClient) sendStats(stats flow.Stats) {
	msg := flow.Message{
		Stats: &stats,
	}

	if err := c.SendMessage(&msg); err != nil {
		logging.GetLogger().Errorf("Unable to send stats: %s", err)
	}
}

// normalizeAddrForURL format the given address to be used in URL. For IPV6
// addresses the brackets will be added.
func normalizeAddrForURL(addr string) string {
	ip := net.ParseIP(addr)
	if ip != nil && len(ip) == net.IPv6len {
		return "[" + addr + "]"
	}
	return addr
}

// NewFlowClient creates a flow client and creates a new connection to the server
func NewFlowClient(addr string, port int, wsOpts *ws.ClientOpts) (*FlowClient, error) {
	var (
		connection FlowClientConn
		err        error
	)
	protocol := strings.ToLower(config.GetString("flow.protocol"))
	switch protocol {
	case "udp":
		connection, err = NewFlowClientUDPConn(normalizeAddrForURL(addr), port)
		if err != nil {
			return nil, err
		}
	case "websocket":
		endpoint := config.GetURL("ws", normalizeAddrForURL(addr), port, "/ws/agent/flow")
		connection = NewFlowClientWebSocketConn(endpoint, wsOpts)
	default:
		return nil, fmt.Errorf("Invalid protocol %s", protocol)
	}

	fc := &FlowClient{addr: addr, port: port, protocol: protocol, flowClientConn: connection}
	fc.connect()

	return fc, nil
}

// OnConnected websocket event handler
func (p *FlowClientPool) OnConnected(c ws.Speaker) error {
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

	flowClient, err := NewFlowClient(addr, port, p.wsOpts)
	if err != nil {
		logging.GetLogger().Error(err)
		return err
	}
	p.flowClients = append(p.flowClients, flowClient)

	return nil
}

// OnDisconnected websocket event handler
func (p *FlowClientPool) OnDisconnected(c ws.Speaker) {
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

// SendFlows implements the flow Sender interface
func (p *FlowClientPool) SendFlows(flows []*flow.Flow) {
	p.RLock()
	defer p.RUnlock()

	if len(p.flowClients) == 0 {
		return
	}

	fc := p.flowClients[rand.Intn(len(p.flowClients))]
	fc.sendFlows(flows)
}

// SendStats implements the flow Sender interface
func (p *FlowClientPool) SendStats(stats flow.Stats) {
	p.RLock()
	defer p.RUnlock()

	if len(p.flowClients) == 0 {
		return
	}

	fc := p.flowClients[rand.Intn(len(p.flowClients))]
	fc.sendStats(stats)
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
func NewFlowClientPool(pool ws.SpeakerPool, opts *ws.ClientOpts) *FlowClientPool {
	p := &FlowClientPool{
		flowClients: make([]*FlowClient, 0),
		wsOpts:      opts,
	}
	pool.AddEventHandler(p)
	return p
}
