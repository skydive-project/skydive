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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
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
	shttp.DefaultWSClientEventHandler
	flowClients []*FlowClient
}

// FlowClient descibes a flow client connection
type FlowClient struct {
	addr           string
	port           int
	flowClientConn FlowClientConn
	conn           net.Conn
}

type FlowClientConn interface {
	Connect() (net.Conn, error)
}

type FlowClientUDPConn struct {
	addr *net.UDPAddr
}

type FlowClientTCPConn struct {
	addr      *net.TCPAddr
	tlsConfig *tls.Config
}

func (c *FlowClientUDPConn) Connect() (net.Conn, error) {
	logging.GetLogger().Debugf("UDP client dialup done for %s", c.addr.String())
	return net.DialUDP("udp", nil, c.addr)
}

func NewFlowClientUDPConn(addr string, port int) (*FlowClientUDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}

	return &FlowClientUDPConn{addr: udpAddr}, nil
}

func (c *FlowClientTCPConn) Connect() (net.Conn, error) {
	ip := common.IPToString(c.addr.IP)
	port := c.addr.Port + 1

	logging.GetLogger().Debugf("Dialing TLS client connection %s:%d", ip, port)
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), c.tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("TLS error %s:%d : %s", ip, port, err.Error())
	}

	state := conn.ConnectionState()
	if state.HandshakeComplete == false {
		return nil, fmt.Errorf("TLS Handshake is not complete %s:%d : %+v", ip, port, state)
	}

	logging.GetLogger().Debugf("TLS v%d Handshake is complete on %s:%d", state.Version, ip, port)
	return conn, nil
}

func NewFlowClientTCPConn(addr string, port int) (*FlowClientTCPConn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}

	certPEM := config.GetConfig().GetString("agent.X509_cert")
	keyPEM := config.GetConfig().GetString("agent.X509_key")
	serverCertPEM := config.GetConfig().GetString("analyzer.X509_cert")
	serverNamePEM := config.GetConfig().GetString("agent.X509_servername")

	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, errors.New("Certificates are required to use TCP for flows")
	}

	cert, err := tls.LoadX509KeyPair(certPEM, keyPEM)
	if err != nil {
		logging.GetLogger().Fatalf("Can't read X509 key pair set in config : cert '%s' key '%s'", certPEM, keyPEM)
		return nil, err
	}

	rootPEM, err := ioutil.ReadFile(serverCertPEM)
	if err != nil {
		logging.GetLogger().Fatalf("Failed to open root certificate '%s' : %s", serverCertPEM, err.Error())
	}

	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM(rootPEM); !ok {
		logging.GetLogger().Fatalf("Failed to parse root certificate '%s'", serverCertPEM)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      roots,
	}
	if len(serverNamePEM) > 0 {
		tlsConfig.ServerName = serverNamePEM
	} else {
		serverNamePEM = "unspecified"
	}
	tlsConfig.BuildNameToCertificate()

	return &FlowClientTCPConn{addr: tcpAddr, tlsConfig: tlsConfig}, nil
}

func (c *FlowClient) connect() {
	conn, err := c.flowClientConn.Connect()
	if err != nil {
		logging.GetLogger().Errorf("Connection error to %s:%d : %s", c.addr, c.port, err.Error())
		time.Sleep(200 * time.Millisecond)
		return
	}
	c.conn = conn
}

func (c *FlowClient) close() {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			logging.GetLogger().Errorf("Error while closing flow connection: %s", err.Error())
		}
	}
}

// SendFlow sends a flow to the server
func (c *FlowClient) SendFlow(f *flow.Flow) error {
	if c.conn == nil {
		return errors.New("Not connected")
	}

	data, err := f.GetData()
	if err != nil {
		return err
	}

retry:
	_, err = c.conn.Write(data)
	if err != nil {
		logging.GetLogger().Errorf("flows connection to analyzer error %s : try to reconnect", err.Error())
		c.conn.Close()
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
	protocol := strings.ToLower(config.GetConfig().GetString("flow.protocol"))
	switch protocol {
	case "udp":
		connection, err = NewFlowClientUDPConn(addr, port)
	case "tcp":
		connection, err = NewFlowClientTCPConn(addr, port)
	default:
		return nil, fmt.Errorf("Invalid protocol %s", protocol)
	}

	if err != nil {
		return nil, err
	}

	fc := &FlowClient{flowClientConn: connection}
	fc.connect()

	return fc, nil
}

// OnConnected websocket event handler
func (p *FlowClientPool) OnConnected(c *shttp.WSAsyncClient) {
	p.Lock()
	defer p.Unlock()

	for i, fc := range p.flowClients {
		if fc.addr == c.Addr && fc.port == c.Port {
			logging.GetLogger().Warningf("Got a connected event on already connected client: %s:%d", c.Addr, c.Port)
			fc.close()

			p.flowClients = append(p.flowClients[:i], p.flowClients[i+1:]...)
		}
	}

	flowClient, err := NewFlowClient(c.Addr, c.Port)
	if err != nil {
		logging.GetLogger().Error(err)
	}

	p.flowClients = append(p.flowClients, flowClient)
}

// OnDisconnected websocket event handler
func (p *FlowClientPool) OnDisconnected(c *shttp.WSAsyncClient) {
	p.Lock()
	defer p.Unlock()

	for i, fc := range p.flowClients {
		if fc.addr == c.Addr && fc.port == c.Port {
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
