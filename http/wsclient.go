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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/websocket"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

const (
	maxMessageSize = 0
	writeWait      = 10 * time.Second
)

// WSMessage is the interface of a message to send over the wire
type WSMessage interface {
	Bytes() []byte
}

// WSRawMessage represents a raw message (array of bytes)
type WSRawMessage []byte

// Bytes returns the string representation of the raw message
func (m WSRawMessage) Bytes() []byte {
	return m
}

// WSSpeaker is the interface for a websocket speaking client. It is used for outgoing
// or incoming connections.
type WSSpeaker interface {
	GetStatus() WSConnStatus
	GetHost() string
	GetAddrPort() (string, int)
	GetServiceType() common.ServiceType
	GetHeaders() http.Header
	GetURL() *url.URL
	IsConnected() bool
	SendMessage(m WSMessage) error
	Connect()
	Disconnect()
	AddEventHandler(WSSpeakerEventHandler)
}

// WSConnState describes the connection state
type WSConnState int32

// WSConnStatus describes the status of a WebSocket connection
type WSConnStatus struct {
	ServiceType common.ServiceType
	Addr        string
	Port        int
	Host        string       `json:"-"`
	State       *WSConnState `json:"IsConnected"`
	Url         *url.URL     `json:"-"`
	headers     http.Header
	ConnectTime time.Time
}

func (s *WSConnState) MarshalJSON() ([]byte, error) {
	switch *s {
	case common.RunningState:
		return []byte("true"), nil
	case common.StoppedState:
		return []byte("false"), nil
	}
	return nil, fmt.Errorf("Invalid state: %d", s)
}

// UnmarshalJSON deserialize a connection state
func (s *WSConnState) UnmarshalJSON(b []byte) error {
	var state bool
	if err := json.Unmarshal(b, &state); err != nil {
		return err
	}

	if state {
		*s = common.RunningState
	} else {
		*s = common.StoppedState
	}

	return nil
}

// WSConn is the connection object of a WSSpeaker
type WSConn struct {
	sync.RWMutex
	WSConnStatus
	send          chan []byte
	read          chan []byte
	quit          chan bool
	wg            sync.WaitGroup
	conn          *websocket.Conn
	running       atomic.Value
	pingTicker    *time.Ticker // only used by incoming connections
	eventHandlers []WSSpeakerEventHandler
	wsSpeaker     WSSpeaker // speaker owning the connection
}

// wsIncomingClient is only used internally to handle incoming client. It embeds a WSConn.
type wsIncomingClient struct {
	*WSConn
}

// WSClient is a outgoint client meaning a client connected to a remote websocket server.
// It embeds a WSConn.
type WSClient struct {
	*WSConn
	Path       string
	AuthClient *AuthenticationClient
}

// WSSpeakerEventHandler is the interface to be implement by the client events listeners.
type WSSpeakerEventHandler interface {
	OnMessage(c WSSpeaker, m WSMessage)
	OnConnected(c WSSpeaker)
	OnDisconnected(c WSSpeaker)
}

// DefaultWSSpeakerEventHandler implements stubs for the wsIncomingClientEventHandler interface
type DefaultWSSpeakerEventHandler struct {
}

// OnMessage is called when a message is received.
func (d *DefaultWSSpeakerEventHandler) OnMessage(c WSSpeaker, m WSMessage) {
}

// OnConnected is called when the connection is established.
func (d *DefaultWSSpeakerEventHandler) OnConnected(c WSSpeaker) {
}

// OnDisconnected is called when the connection is closed or lost.
func (d *DefaultWSSpeakerEventHandler) OnDisconnected(c WSSpeaker) {
}

// GetHost returns the hostname/host-id of the connection.
func (c *WSConn) GetHost() string {
	return c.Host
}

// GetAddrPort returns the address and the port of the remote end.
func (c *WSConn) GetAddrPort() (string, int) {
	return c.Addr, c.Port
}

// GetURL returns the URL of the connection
func (c *WSConn) GetURL() *url.URL {
	return c.Url
}

// IsConnected returns the connection status.
func (c *WSConn) IsConnected() bool {
	return atomic.LoadInt32((*int32)(c.State)) == common.RunningState
}

// GetStatus returns the status of a WebSocket connection
func (c *WSConn) GetStatus() WSConnStatus {
	c.RLock()
	defer c.RUnlock()

	status := c.WSConnStatus
	status.State = new(WSConnState)
	*status.State = WSConnState(atomic.LoadInt32((*int32)(c.State)))
	return c.WSConnStatus
}

// SendMessage adds a message to sending queue.
func (c *WSConn) SendMessage(m WSMessage) error {
	if !c.IsConnected() {
		return errors.New("Not connected")
	}

	c.send <- m.Bytes()

	return nil
}

// SendRaw adds raw bytes to sending queue.
func (c *WSConn) SendRaw(b []byte) error {
	if !c.IsConnected() {
		return errors.New("Not connected")
	}

	c.send <- b

	return nil
}

// GetServiceType returns the client type.
func (c *WSConn) GetServiceType() common.ServiceType {
	return c.ServiceType
}

// GetHeaders returns the client HTTP headers.
func (c *WSConn) GetHeaders() http.Header {
	return c.headers
}

// SendMessage sends a message directly over the wire.
func (c *WSConn) write(msg []byte) error {
	if !c.IsConnected() {
		return errors.New("Not connected")
	}

	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	c.conn.EnableWriteCompression(config.GetBool("http.ws.enable_write_compression"))
	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if _, err = w.Write(msg); err != nil {
		return err
	}

	return w.Close()
}

func (c *WSConn) start() {
	c.wg.Add(1)
	go c.run()
}

// main loop to read and send messages
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
		atomic.StoreInt32((*int32)(c.State), common.StoppedState)
		c.conn.Close()

		c.RLock()
		for _, l := range c.eventHandlers {
			l.OnDisconnected(c.wsSpeaker)
		}
		c.RUnlock()
	}()

	done := make(chan bool, 2)
	go func() {
		for {
			select {
			case m := <-c.send:
				if err := c.write(m); err != nil {
					logging.GetLogger().Errorf("Error while writing to the WebSocket: %s", err)
				}
			case <-c.pingTicker.C:
				if err := c.sendPing(); err != nil {
					logging.GetLogger().Errorf("Error while sending ping to %+v: %s", c, err)

					// stop the ticker and request a quit
					c.pingTicker.Stop()
					c.quit <- true
				}
			case <-done:
				return
			}
		}
	}()
	defer func() {
		done <- true
	}()

	for {
		select {
		case <-c.quit:
			return
		case m := <-c.read:
			c.RLock()
			for _, l := range c.eventHandlers {
				l.OnMessage(c.wsSpeaker, WSRawMessage(m))
			}
			c.RUnlock()
		}
	}
}

// sendPing is used for remote connections by the server to send PingMessage
// to remote client.
func (c *WSConn) sendPing() error {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(websocket.PingMessage, []byte{})
}

// AddEventHandler registers a new event handler
func (c *WSConn) AddEventHandler(h WSSpeakerEventHandler) {
	c.Lock()
	c.eventHandlers = append(c.eventHandlers, h)
	c.Unlock()
}

// Connect default implementation doing nothing as for incoming connection it is not used.
func (c *WSConn) Connect() {
}

// Disconnect the WSSpeakers without waiting for termination.
func (c *WSConn) Disconnect() {
	c.running.Store(false)
	if atomic.LoadInt32((*int32)(c.State)) == common.RunningState {
		c.quit <- true
	}
}

func newWSConn(host string, clientType common.ServiceType, url *url.URL, headers http.Header, queueSize int) *WSConn {
	if headers == nil {
		headers = http.Header{}
	}

	port, _ := strconv.Atoi(url.Port())
	c := &WSConn{
		WSConnStatus: WSConnStatus{
			Host:        host,
			ServiceType: clientType,
			Addr:        url.Hostname(),
			Port:        port,
			State:       new(WSConnState),
			Url:         url,
			headers:     headers,
			ConnectTime: time.Now(),
		},
		send:       make(chan []byte, queueSize),
		read:       make(chan []byte, queueSize),
		quit:       make(chan bool, 2),
		pingTicker: &time.Ticker{},
	}
	*c.State = common.StoppedState
	c.running.Store(true)
	return c
}

func (c *WSClient) scheme() string {
	if config.IsTLSenabled() == true {
		return "wss://"
	}
	return "ws://"
}

func (c *WSClient) connect() {
	var err error
	endpoint := c.Url.String()
	headers := http.Header{
		"X-Host-ID":             {c.Host},
		"Origin":                {endpoint},
		"X-Client-Type":         {c.ServiceType.String()},
		"X-Websocket-Namespace": {WilcardNamespace},
	}

	if c.AuthClient != nil {
		if err = c.AuthClient.Authenticate(); err != nil {
			logging.GetLogger().Errorf("Unable to authenticate %s : %s", endpoint, err.Error())
			return
		}
	}

	setCookies(&headers, c.AuthClient)

	d := websocket.Dialer{
		Proxy:           http.ProxyFromEnvironment,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	d.TLSClientConfig = getTLSConfig(false)
	c.conn, _, err = d.Dial(endpoint, headers)

	if err != nil {
		logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
		return
	}
	c.conn.SetPingHandler(nil)
	c.conn.EnableWriteCompression(config.GetBool("http.ws.enable_write_compression"))

	atomic.StoreInt32((*int32)(c.State), common.RunningState)
	defer atomic.StoreInt32((*int32)(c.State), common.StoppedState)

	logging.GetLogger().Infof("Connected to %s", endpoint)

	// notify connected
	c.RLock()
	var eventHandlers []WSSpeakerEventHandler
	eventHandlers = append(eventHandlers, c.eventHandlers...)
	c.RUnlock()

	for _, l := range eventHandlers {
		l.OnConnected(c)
		if !c.IsConnected() {
			return
		}
	}

	c.wg.Add(1)
	c.run()
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

// NewWSClient returns a WSClient with a new connection.
func NewWSClient(host string, clientType common.ServiceType, url *url.URL, authClient *AuthenticationClient, headers http.Header, queueSize int) *WSClient {
	wsconn := newWSConn(host, clientType, url, headers, queueSize)
	c := &WSClient{
		WSConn:     wsconn,
		AuthClient: authClient,
	}
	wsconn.wsSpeaker = c
	return c
}

// NewWSClientFromConfig creates a WSClient based on the configuration
func NewWSClientFromConfig(clientType common.ServiceType, url *url.URL, authClient *AuthenticationClient, headers http.Header) *WSClient {
	host := config.GetString("host_id")
	queueSize := config.GetInt("http.ws.queue_size")
	return NewWSClient(host, clientType, url, authClient, headers, queueSize)
}

// newIncomingWSClient is called by the server for incoming connections
func newIncomingWSClient(conn *websocket.Conn, r *auth.AuthenticatedRequest) *wsIncomingClient {
	host := getRequestParameter(r, "X-Host-ID")
	if host == "" {
		host = r.RemoteAddr
	}

	clientType := common.ServiceType(getRequestParameter(r, "X-Client-Type"))
	if clientType == "" {
		clientType = common.UnknownService
	}

	queueSize := config.GetInt("http.ws.queue_size")

	svc, _ := common.ServiceAddressFromString(conn.RemoteAddr().String())
	url := config.GetURL("http", svc.Addr, svc.Port, r.URL.Path+"?"+r.URL.RawQuery)
	wsconn := newWSConn(host, clientType, url, r.Header, queueSize)
	wsconn.conn = conn

	pingDelay := time.Duration(config.GetInt("http.ws.ping_delay")) * time.Second
	pongTimeout := time.Duration(config.GetInt("http.ws.pong_timeout"))*time.Second + pingDelay

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	c := &wsIncomingClient{
		WSConn: wsconn,
	}
	wsconn.wsSpeaker = c

	atomic.StoreInt32((*int32)(c.State), common.RunningState)

	// send a first ping to help firefox and some other client which wait for a
	// first ping before doing something
	c.sendPing()

	wsconn.pingTicker = time.NewTicker(pingDelay)

	return c
}
