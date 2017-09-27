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
type WSMessage interface {
	Bytes() []byte
}

// WSRawMessage represents a raw message (array of bytes)
type WSRawMessage []byte

// Return the string representation of the raw message
func (m WSRawMessage) Bytes() []byte {
	return m
}

type WSSpeaker interface {
	GetHost() string
	GetAddrPort() (string, int)
	GetClientType() common.ServiceType
	IsConnected() bool
	Send(m WSMessage)
	Connect()
	Disconnect()
	AddEventHandler(WSSpeakerEventHandler)
}

// WebSocket client interface
type WSConn struct {
	sync.RWMutex
	Host          string
	ClientType    common.ServiceType
	Addr          string
	Port          int
	send          chan []byte
	read          chan []byte
	quit          chan bool
	wg            sync.WaitGroup
	conn          *websocket.Conn
	running       atomic.Value
	pingTicker    *time.Ticker // only used by incoming connections
	connected     atomic.Value
	eventHandlers []WSSpeakerEventHandler
	wsSpeaker     WSSpeaker // speaker owning the connection
}

// wsIncomingClient describes a WebSocket connection. wsIncomingClient is used
// by the client when connecting to a server, and also by the server when
// a client connects
type wsIncomingClient struct {
	*WSConn
}

type WSClient struct {
	*WSConn
	Path       string
	AuthClient *AuthenticationClient
}

// Interface to be implement by the client events listeners
type WSSpeakerEventHandler interface {
	OnMessage(c WSSpeaker, m WSMessage)
	OnConnected(c WSSpeaker)
	OnDisconnected(c WSSpeaker)
}

// DefaultWSClientEventHandler implements stubs for the wsIncomingClientEventHandler interface
type DefaultWSSpeakerEventHandler struct {
}

// OnMessage is called when a message is received
func (d *DefaultWSSpeakerEventHandler) OnMessage(c WSSpeaker, m WSMessage) {
}

// OnConnected is called when the connection is established
func (d *DefaultWSSpeakerEventHandler) OnConnected(c WSSpeaker) {
}

// OnDisconnected is called when the connection is closed or lost
func (d *DefaultWSSpeakerEventHandler) OnDisconnected(c WSSpeaker) {
}

// GetHost returns the hostname of the connection
func (c *WSConn) GetHost() string {
	return c.Host
}

// GetAddrPort returns the address and the port of the remote end
func (c *WSConn) GetAddrPort() (string, int) {
	return c.Addr, c.Port
}

// IsConnected returns the connection status
func (c *WSConn) IsConnected() bool {
	return c.connected.Load() == true
}

// Send adds a message to the send queue
func (c *WSConn) Send(m WSMessage) {
	if c.running.Load() == false {
		return
	}
	c.send <- m.Bytes()
}

// GetClientType returns the client type
func (c *WSConn) GetClientType() common.ServiceType {
	return c.ClientType
}

// SendMessage sends a message directly over the wire
func (c *WSConn) SendMessage(msg []byte) error {
	if !c.IsConnected() {
		return errors.New("Not connected")
	}

	c.conn.SetWriteDeadline(time.Now().Add(writeWait))

	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if _, err = w.Write(msg); err != nil {
		return err
	}

	return w.Close()
}

// Return the URL scheme
func (c *WSClient) scheme() string {
	if config.IsTLSenabled() == true {
		return "wss://"
	}
	return "ws://"
}

// Connect to the server
func (c *WSClient) connect() {
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
	c.conn, _, err = d.Dial(endpoint, headers)

	if err != nil {
		logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
		return
	}
	defer c.conn.Close()
	c.conn.SetPingHandler(nil)

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

func (c *WSConn) start() {
	c.wg.Add(1)
	go c.run()
}

// Client main loop to read and send messages
func (c *WSConn) run() {
	defer c.wg.Done()

	go func() {
		for c.running.Load() == true {
			_, m, err := c.conn.ReadMessage()
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
			l.OnDisconnected(c.wsSpeaker)
		}
		c.RUnlock()
	}()

	defer c.pingTicker.Stop()

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
				l.OnMessage(c.wsSpeaker, WSRawMessage(m))
			}
			c.RUnlock()
		case <-c.pingTicker.C:
			c.sendPing()
		}
	}
}

// sendPing is used for remote connections by the server to send PingMessage
// to remote client.
func (c *WSConn) sendPing() {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
		logging.GetLogger().Warningf("Error while sending ping to the websocket: %s", err.Error())
	}
}

// Connect to the server - and reconnect if necessary
func (c *WSClient) Connect() {
	go func() {
		for c.running.Load() == true {
			c.connect()
			time.Sleep(1 * time.Second)
		}
	}()
}

// AddEventHandler registers a new event handler
func (c *WSConn) AddEventHandler(h WSSpeakerEventHandler) {
	c.Lock()
	c.eventHandlers = append(c.eventHandlers, h)
	c.Unlock()
}

func (c *WSConn) Connect() {
}

// Disconnect the client without waiting for termination
func (c *WSConn) Disconnect() {
	c.running.Store(false)
	if c.connected.Load() == true {
		c.quit <- true
		c.conn.Close()
		close(c.send)
		close(c.read)
	}
}

func newWSCon(host string, clientType common.ServiceType, addr string, port int) *WSConn {
	c := &WSConn{
		Host:       host,
		ClientType: clientType,
		Addr:       addr,
		Port:       port,
		send:       make(chan []byte, maxMessages),
		read:       make(chan []byte, maxMessages),
		quit:       make(chan bool, 2),
		pingTicker: &time.Ticker{},
	}
	c.connected.Store(false)
	c.running.Store(true)
	return c
}

// NewwsIncomingClient returns a client with a new connection
func NewWSClient(host string, clientType common.ServiceType, addr string, port int, path string, authClient *AuthenticationClient) *WSClient {
	wsconn := newWSCon(host, clientType, addr, port)
	c := &WSClient{
		WSConn:     wsconn,
		Path:       path,
		AuthClient: authClient,
	}
	wsconn.wsSpeaker = c
	return c
}

// NewWSAsyncClientFromConnection creates a client based on the configuration
func NewWSClientFromConfig(clientType common.ServiceType, addr string, port int, path string, authClient *AuthenticationClient) *WSClient {
	host := config.GetConfig().GetString("host_id")
	return NewWSClient(host, clientType, addr, port, path, authClient)
}

// newIncomingWSConn is called by the server for incoming connections
func newIncomingWSClient(host string, clientType common.ServiceType, conn *websocket.Conn) *wsIncomingClient {
	svc, _ := common.ServiceAddressFromString(conn.RemoteAddr().String())
	wsconn := newWSCon(host, clientType, svc.Addr, svc.Port)
	wsconn.conn = conn

	pongTimeout := time.Duration(config.GetConfig().GetInt("ws_pong_timeout")) * time.Second

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(100 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	c := &wsIncomingClient{
		WSConn: wsconn,
	}
	wsconn.wsSpeaker = c

	c.connected.Store(true)

	// send a first ping to help firefox and some other client which wait for a
	// first ping before doing something
	c.sendPing()

	wsconn.pingTicker = time.NewTicker(pongTimeout * 8 / 10)

	return c
}
