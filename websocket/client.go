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

package websocket

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	fmt "fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

const (
	maxMessageSize = 0
	writeWait      = 10 * time.Second
)

// ConnState describes the connection state
type ConnState int32

// ConnStatus describes the status of a WebSocket connection
type ConnStatus struct {
	ServiceType       common.ServiceType
	ClientProtocol    string
	Addr              string
	Port              int
	Host              string      `json:"-"`
	State             *ConnState  `json:"IsConnected"`
	URL               *url.URL    `json:"-"`
	Headers           http.Header `json:"-"`
	ConnectTime       time.Time
	RemoteHost        string             `json:",omitempty"`
	RemoteServiceType common.ServiceType `json:",omitempty"`
}

// MarshalJSON marshal the connexion state to JSON
func (s *ConnState) MarshalJSON() ([]byte, error) {
	switch *s {
	case common.RunningState:
		return []byte("true"), nil
	case common.StoppedState:
		return []byte("false"), nil
	}
	return nil, fmt.Errorf("Invalid state: %d", s)
}

// UnmarshalJSON deserialize a connection state
func (s *ConnState) UnmarshalJSON(b []byte) error {
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

// Message is the interface of a message to send over the wire
type Message interface {
	Bytes(protocol string) []byte
}

// RawMessage represents a raw message (array of bytes)
type RawMessage []byte

// Bytes returns the string representation of the raw message
func (m RawMessage) Bytes(protocol string) []byte {
	return m
}

// Speaker is the interface for a websocket speaking client. It is used for outgoing
// or incoming connections.
type Speaker interface {
	GetStatus() ConnStatus
	GetHost() string
	GetAddrPort() (string, int)
	GetServiceType() common.ServiceType
	GetClientProtocol() string
	GetHeaders() http.Header
	GetURL() *url.URL
	IsConnected() bool
	SendMessage(m Message) error
	SendRaw(r []byte) error
	Connect()
	Disconnect()
	AddEventHandler(SpeakerEventHandler)
	GetRemoteHost() string
	GetRemoteServiceType() common.ServiceType
}

// Conn is the connection object of a Speaker
type Conn struct {
	common.RWMutex
	ConnStatus
	send             chan []byte
	read             chan []byte
	quit             chan bool
	wg               sync.WaitGroup
	conn             *websocket.Conn
	running          atomic.Value
	pingTicker       *time.Ticker // only used by incoming connections
	eventHandlers    []SpeakerEventHandler
	wsSpeaker        Speaker // speaker owning the connection
	writeCompression bool
}

// wsIncomingClient is only used internally to handle incoming client. It embeds a Conn.
type wsIncomingClient struct {
	*Conn
}

// Client is a outgoint client meaning a client connected to a remote websocket server.
// It embeds a Conn.
type Client struct {
	*Conn
	Path      string
	AuthOpts  *shttp.AuthenticationOpts
	tlsConfig *tls.Config
}

// SpeakerEventHandler is the interface to be implement by the client events listeners.
type SpeakerEventHandler interface {
	OnMessage(c Speaker, m Message)
	OnConnected(c Speaker)
	OnDisconnected(c Speaker)
}

// DefaultSpeakerEventHandler implements stubs for the wsIncomingClientEventHandler interface
type DefaultSpeakerEventHandler struct {
}

// OnMessage is called when a message is received.
func (d *DefaultSpeakerEventHandler) OnMessage(c Speaker, m Message) {
}

// OnConnected is called when the connection is established.
func (d *DefaultSpeakerEventHandler) OnConnected(c Speaker) {
}

// OnDisconnected is called when the connection is closed or lost.
func (d *DefaultSpeakerEventHandler) OnDisconnected(c Speaker) {
}

// GetHost returns the hostname/host-id of the connection.
func (c *Conn) GetHost() string {
	return c.Host
}

// GetAddrPort returns the address and the port of the remote end.
func (c *Conn) GetAddrPort() (string, int) {
	return c.Addr, c.Port
}

// GetURL returns the URL of the connection
func (c *Conn) GetURL() *url.URL {
	return c.URL
}

// IsConnected returns the connection status.
func (c *Conn) IsConnected() bool {
	return atomic.LoadInt32((*int32)(c.State)) == common.RunningState
}

// GetStatus returns the status of a WebSocket connection
func (c *Conn) GetStatus() ConnStatus {
	c.RLock()
	defer c.RUnlock()

	status := c.ConnStatus
	status.State = new(ConnState)
	*status.State = ConnState(atomic.LoadInt32((*int32)(c.State)))
	return c.ConnStatus
}

// SpeakerStructMessageHandler interface used to receive Struct messages.
type SpeakerStructMessageHandler interface {
	OnStructMessage(c Speaker, m *StructMessage)
}

// SendMessage adds a message to sending queue.
func (c *Conn) SendMessage(m Message) error {
	if !c.IsConnected() {
		return errors.New("Not connected")
	}

	c.send <- m.Bytes(c.GetClientProtocol())

	return nil
}

// SendRaw adds raw bytes to sending queue.
func (c *Conn) SendRaw(b []byte) error {
	if !c.IsConnected() {
		return errors.New("Not connected")
	}

	c.send <- b

	return nil
}

// GetServiceType returns the client type.
func (c *Conn) GetServiceType() common.ServiceType {
	return c.ServiceType
}

// GetClientProtocol returns the websocket protocol.
func (c *Conn) GetClientProtocol() string {
	return c.ClientProtocol
}

// GetHeaders returns the client HTTP headers.
func (c *Conn) GetHeaders() http.Header {
	return c.Headers
}

// GetRemoteHost returns the hostname/host-id of the remote side of the connection.
func (c *Conn) GetRemoteHost() string {
	return c.RemoteHost
}

// GetRemoteServiceType returns the remote service type.
func (c *Conn) GetRemoteServiceType() common.ServiceType {
	return c.RemoteServiceType
}

// SendMessage sends a message directly over the wire.
func (c *Conn) write(msg []byte) error {
	if !c.IsConnected() {
		return errors.New("Not connected")
	}

	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	c.conn.EnableWriteCompression(c.writeCompression)
	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if _, err = w.Write(msg); err != nil {
		return err
	}

	return w.Close()
}

func (c *Conn) start() {
	c.wg.Add(1)
	go c.run()
}

// main loop to read and send messages
func (c *Conn) run() {
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
				l.OnMessage(c.wsSpeaker, RawMessage(m))
			}
			c.RUnlock()
		}
	}
}

// sendPing is used for remote connections by the server to send PingMessage
// to remote client.
func (c *Conn) sendPing() error {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(websocket.PingMessage, []byte{})
}

// AddEventHandler registers a new event handler
func (c *Conn) AddEventHandler(h SpeakerEventHandler) {
	c.Lock()
	c.eventHandlers = append(c.eventHandlers, h)
	c.Unlock()
}

// Connect default implementation doing nothing as for incoming connection it is not used.
func (c *Conn) Connect() {
}

// Disconnect the Speakers without waiting for termination.
func (c *Conn) Disconnect() {
	c.running.Store(false)
	if atomic.CompareAndSwapInt32((*int32)(c.State), common.RunningState, common.StoppingState) {
		c.quit <- true
	}
}

func newConn(host string, clientType common.ServiceType, clientProtocol string, url *url.URL, headers http.Header, queueSize int, writeCompression bool) *Conn {
	if headers == nil {
		headers = http.Header{}
	}

	port, _ := strconv.Atoi(url.Port())
	c := &Conn{
		ConnStatus: ConnStatus{
			Host:           host,
			ServiceType:    clientType,
			ClientProtocol: clientProtocol,
			Addr:           url.Hostname(),
			Port:           port,
			State:          new(ConnState),
			URL:            url,
			Headers:        headers,
			ConnectTime:    time.Now(),
		},
		send:             make(chan []byte, queueSize),
		read:             make(chan []byte, queueSize),
		quit:             make(chan bool, 2),
		pingTicker:       &time.Ticker{},
		writeCompression: writeCompression,
	}
	*c.State = common.StoppedState
	c.running.Store(true)
	return c
}

func (c *Client) scheme() string {
	if c.tlsConfig != nil {
		return "wss://"
	}
	return "ws://"
}

func (c *Client) connect() {
	var err error
	endpoint := c.URL.String()
	headers := http.Header{
		"X-Host-ID":             {c.Host},
		"Origin":                {endpoint},
		"X-Client-Type":         {c.ServiceType.String()},
		"X-Client-Protocol":     {ProtobufProtocol},
		"X-Websocket-Namespace": {WildcardNamespace},
	}

	if c.AuthOpts != nil {
		shttp.SetAuthHeaders(&headers, c.AuthOpts)
	}

	d := websocket.Dialer{
		Proxy:           http.ProxyFromEnvironment,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	d.TLSClientConfig = c.tlsConfig
	if err != nil {
		logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err)
		return
	}

	var resp *http.Response
	c.conn, resp, err = d.Dial(endpoint, headers)

	if err != nil {
		logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err)
		return
	}
	c.conn.SetPingHandler(nil)
	c.conn.EnableWriteCompression(c.writeCompression)

	atomic.StoreInt32((*int32)(c.State), common.RunningState)
	defer atomic.StoreInt32((*int32)(c.State), common.StoppedState)

	logging.GetLogger().Infof("Connected to %s", endpoint)

	c.RemoteHost = resp.Header.Get("X-Host-ID")

	// NOTE(safchain): fallback to remote addr if host id not provided
	// should be removed, connection should be refused if host id not provided
	if c.RemoteHost == "" {
		c.RemoteHost = c.conn.RemoteAddr().String()
	}

	c.RemoteServiceType = common.ServiceType(resp.Header.Get("X-Service-Type"))
	if c.RemoteServiceType == "" {
		c.RemoteServiceType = common.UnknownService
	}

	// notify connected
	c.RLock()
	var eventHandlers []SpeakerEventHandler
	eventHandlers = append(eventHandlers, c.eventHandlers...)
	c.RUnlock()

	for _, l := range eventHandlers {
		l.OnConnected(c)
	}

	// in case of a handler disconnect the client directly
	if !c.IsConnected() {
		return
	}

	c.wg.Add(1)
	c.run()
}

// Connect to the server - and reconnect if necessary
func (c *Client) Connect() {
	go func() {
		for c.running.Load() == true {
			c.connect()
			time.Sleep(1 * time.Second)
		}
	}()
}

// NewClient returns a Client with a new connection.
func NewClient(host string, clientType common.ServiceType, url *url.URL, authOpts *shttp.AuthenticationOpts, headers http.Header, queueSize int, writeCompression bool, tlsConfig *tls.Config) *Client {
	wsconn := newConn(host, clientType, ProtobufProtocol, url, headers, queueSize, writeCompression)
	c := &Client{
		Conn:      wsconn,
		AuthOpts:  authOpts,
		tlsConfig: tlsConfig,
	}
	wsconn.wsSpeaker = c
	return c
}
