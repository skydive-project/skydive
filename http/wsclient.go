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

package http

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

const (
	maxMessages    = 1024
	maxMessageSize = 0
	writeWait      = 10 * time.Second
)

// Interface of a message to send over the wire
type Message interface {
	Bytes() []byte
}

// WSRawMessage represents a raw message (array of bytes)
type WSRawMessage []byte

// Return the string representation of the raw message
func (m WSRawMessage) Bytes() []byte {
	return m
}

// WebSocket client interface
type WSClient interface {
	GetHost() string
	GetAddrPort() (string, int)
	GetClientType() common.ServiceType
	IsConnected() bool
	Send(m Message)
	Connect()
	Disconnect()
	AddEventHandler(WSClientEventHandler)
}

// WSAsyncClient describes a WebSocket connection. WSAsyncClient is used
// by the client when connecting to a server, and also by the server when
// a client connects
type WSAsyncClient struct {
	sync.RWMutex
	Host              string
	ClientType        common.ServiceType
	Addr              string
	Port              int
	Path              string
	AuthClient        *AuthenticationClient
	headers           http.Header
	send              chan []byte
	read              chan []byte
	quit              chan bool
	wg                sync.WaitGroup
	wsConn            *websocket.Conn
	eventHandlers     []WSClientEventHandler
	eventHandlersLock sync.RWMutex
	connected         atomic.Value
	running           atomic.Value
	pongWait          time.Duration
	pingPeriod        time.Duration
	ticker            *time.Ticker
}

// Interface to be implement by the client events listeners
type WSClientEventHandler interface {
	OnMessage(c WSClient, m Message)
	OnConnected(c WSClient)
	OnDisconnected(c WSClient)
}

// DefaultWSClientEventHandler implements stubs for the WSClientEventHandler interface
type DefaultWSClientEventHandler struct {
}

// OnMessage is called when a message is received
func (d *DefaultWSClientEventHandler) OnMessage(c WSClient, m Message) {
}

// OnConnected is called when the connection is established
func (d *DefaultWSClientEventHandler) OnConnected(c WSClient) {
}

// OnDisconnected is called when the connection is closed or lost
func (d *DefaultWSClientEventHandler) OnDisconnected(c WSClient) {
}

// Return the hostname of the connection
func (c *WSAsyncClient) GetHost() string {
	return c.Host
}

// Return the address and the port of the remote end
func (c *WSAsyncClient) GetAddrPort() (string, int) {
	return c.Addr, c.Port
}

// Return the connection status
func (c *WSAsyncClient) IsConnected() bool {
	return c.connected.Load() == true
}

// Add a message to the send queue
func (c *WSAsyncClient) Send(m Message) {
	if c.running.Load() == false {
		return
	}

	c.send <- m.Bytes()
}

// Return the client type
func (c *WSAsyncClient) GetClientType() common.ServiceType {
	return c.ClientType
}

// Send a message directly over the wire
func (c *WSAsyncClient) SendMessage(msg []byte) error {
	if !c.IsConnected() {
		return errors.New("Not connected")
	}

	c.wsConn.SetWriteDeadline(time.Now().Add(writeWait))

	w, err := c.wsConn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if _, err = w.Write(msg); err != nil {
		return err
	}

	return w.Close()
}

// Return the URL scheme
func (c *WSAsyncClient) scheme() string {
	if config.IsTLSenabled() == true {
		return "wss://"
	}
	return "ws://"
}

// Connect to the server
func (c *WSAsyncClient) connect() {
	var err error
	host := c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10)
	endpoint := c.scheme() + host + c.Path
	headers := http.Header{
		"X-Host-ID":             {c.Host},
		"Origin":                {endpoint},
		"X-Client-Type":         {c.ClientType.String()},
		"X-Websocket-Namespace": {WilcardNamespace},
	}

	if c.AuthClient != nil {
		if err = c.AuthClient.Authenticate(); err != nil {
			logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
			return
		}
		c.AuthClient.SetHeaders(headers)
	}

	d := websocket.Dialer{
		Proxy:           http.ProxyFromEnvironment,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	certPEM := config.GetConfig().GetString("agent.X509_cert")
	keyPEM := config.GetConfig().GetString("agent.X509_key")
	if certPEM != "" && keyPEM != "" {
		d.TLSClientConfig = common.SetupTLSClientConfig(certPEM, keyPEM)
		checkTLSConfig(d.TLSClientConfig)
	}
	c.wsConn, _, err = d.Dial(endpoint, headers)

	if err != nil {
		logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
		return
	}
	defer c.wsConn.Close()
	c.wsConn.SetPingHandler(nil)

	c.connected.Store(true)
	defer c.connected.Store(false)

	logging.GetLogger().Infof("Connected to %s", endpoint)

	// notify connected
	c.RLock()
	for _, l := range c.eventHandlers {
		l.OnConnected(c)
	}
	c.RUnlock()

	c.wg.Add(1)
	c.run()
}

func (c *WSAsyncClient) start() {
	c.wg.Add(1)
	go c.run()
}

// Client main loop to read and send messages
func (c *WSAsyncClient) run() {
	defer c.wg.Done()

	go func() {
		for c.running.Load() == true {
			_, m, err := c.wsConn.ReadMessage()
			if err != nil {
				if c.running.Load() != false {
					c.quit <- true
				}
				break
			}

			c.read <- m
		}
	}()

	defer func() {
		c.connected.Store(false)
		c.RLock()
		for _, l := range c.eventHandlers {
			l.OnDisconnected(c)
		}
		c.RUnlock()
	}()

	defer c.ticker.Stop()

	for {
		select {
		case <-c.quit:
			return
		case m := <-c.send:
			err := c.SendMessage(m)
			if err != nil {
				logging.GetLogger().Errorf("Error while writing to the WebSocket: %s", err.Error())
			}
		case m := <-c.read:
			c.RLock()
			for _, l := range c.eventHandlers {
				l.OnMessage(c, WSRawMessage(m))
			}
			c.RUnlock()
		case <-c.ticker.C:
			c.SendPing()
		}
	}
}

func (c *WSAsyncClient) SendPing() {
	c.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.wsConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
		logging.GetLogger().Warningf("Error while sending ping to the websocket: %s", err.Error())
	}
}

// Connect to the server - and reconnect if necessary
func (c *WSAsyncClient) Connect() {
	go func() {
		for c.running.Load() == true {
			c.connect()
			time.Sleep(1 * time.Second)
		}
	}()
}

// Register a new event handler
func (c *WSAsyncClient) AddEventHandler(h WSClientEventHandler) {
	c.Lock()
	c.eventHandlers = append(c.eventHandlers, h)
	c.Unlock()
}

// Disconnect the client without waiting for termination
func (c *WSAsyncClient) Disconnect() {
	c.running.Store(false)
	if c.connected.Load() == true {
		c.quit <- true
		c.wsConn.Close()
		close(c.send)
		close(c.read)
	}
}

// NewWSAsyncClient returns a client with a new connection
func NewWSAsyncClient(host string, clientType common.ServiceType, addr string, port int, path string, authClient *AuthenticationClient) *WSAsyncClient {
	pongTimeout := time.Duration(config.GetConfig().GetInt("ws_pong_timeout")) * time.Second
	c := &WSAsyncClient{
		Host:       host,
		ClientType: clientType,
		Addr:       addr,
		Port:       port,
		Path:       path,
		AuthClient: authClient,
		send:       make(chan []byte, maxMessages),
		read:       make(chan []byte, maxMessages),
		quit:       make(chan bool, 2),
		pongWait:   pongTimeout,
		pingPeriod: pongTimeout * 8 / 10,
		ticker:     &time.Ticker{},
	}
	c.connected.Store(false)
	c.running.Store(true)
	return c
}

// NewWSAsyncClientFromConnection creates a client from an existing connection
func NewWSAsyncClientFromConnection(host string, clientType common.ServiceType, conn *websocket.Conn) *WSAsyncClient {
	svc, _ := common.ServiceAddressFromString(conn.RemoteAddr().String())
	c := NewWSAsyncClient(host, clientType, svc.Addr, svc.Port, "", nil)

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(100 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(c.pongWait))
		return nil
	})
	c.wsConn = conn
	c.ticker = time.NewTicker(c.pingPeriod)

	// send a first ping to help firefox and some other client which wait for a
	// first ping before doing something
	c.SendPing()

	c.connected.Store(true)
	c.running.Store(true)
	return c
}

// NewWSAsyncClientFromConnection creates a client based on the configuration
func NewWSAsyncClientFromConfig(clientType common.ServiceType, addr string, port int, path string, authClient *AuthenticationClient) *WSAsyncClient {
	host := config.GetConfig().GetString("host_id")
	return NewWSAsyncClient(host, clientType, addr, port, path, authClient)
}
