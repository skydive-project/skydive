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
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/websocket"
	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

const (
	WilcardNamespace = "*"
	BulkMsgType      = "BulkMessage"
	writeWait        = 10 * time.Second
	maxMessages      = 1024
	maxMessageSize   = 0
)

type WSClient struct {
	Host         string
	ClientType   common.ServiceType
	conn         *websocket.Conn
	read         chan []byte
	send         chan []byte
	server       *WSServer
	nsSubscribed map[string]bool
}

type WSMessage struct {
	Namespace string
	Type      string
	UUID      string `json:",omitempty"`
	Obj       *json.RawMessage
	Status    int
}

type WSBulkMessage []json.RawMessage

type WSServerEventHandler interface {
	OnMessage(c *WSClient, m WSMessage)
	OnRegisterClient(c *WSClient)
	OnUnregisterClient(c *WSClient)
}

type DefaultWSServerEventHandler struct {
}

type broadcastMessage struct {
	namespace string
	message   string
}

type WSServer struct {
	sync.RWMutex
	DefaultWSServerEventHandler
	Server          *Server
	eventHandlers   []WSServerEventHandler
	nsEventHandlers map[string][]WSServerEventHandler
	clients         map[*WSClient]bool
	broadcast       chan broadcastMessage
	quit            chan bool
	register        chan *WSClient
	unregister      chan *WSClient
	pongWait        time.Duration
	pingPeriod      time.Duration
	bulkMaxMsgs     int
	bulkMaxDelay    time.Duration
	eventBuffer     []*WSMessage
	wg              sync.WaitGroup
	listening       atomic.Value
}

func (g WSMessage) Marshal() []byte {
	j, _ := json.Marshal(g)
	return j
}

func (g WSMessage) String() string {
	return string(g.Marshal())
}

func (g *WSMessage) Reply(v interface{}, kind string, status int) *WSMessage {
	b, _ := json.Marshal(v)
	raw := json.RawMessage(b)

	return &WSMessage{
		Namespace: g.Namespace,
		UUID:      g.UUID,
		Obj:       &raw,
		Type:      kind,
		Status:    status,
	}
}

func NewWSMessage(ns string, tp string, v interface{}, uuids ...string) *WSMessage {
	var u string
	if len(uuids) != 0 {
		u = uuids[0]
	} else {
		v4, _ := uuid.NewV4()
		u = v4.String()
	}

	b, _ := json.Marshal(v)
	raw := json.RawMessage(b)

	return &WSMessage{
		Namespace: ns,
		Type:      tp,
		UUID:      u,
		Obj:       &raw,
		Status:    http.StatusOK,
	}
}

func (d *DefaultWSServerEventHandler) OnMessage(c *WSClient, m WSMessage) {
}

func (d *DefaultWSServerEventHandler) OnRegisterClient(c *WSClient) {
}

func (d *DefaultWSServerEventHandler) OnUnregisterClient(c *WSClient) {
}

func (c *WSClient) SendWSMessage(msg *WSMessage) bool {
	if _, ok := c.nsSubscribed[msg.Namespace]; ok {
		c.send <- []byte(msg.String())
		return true
	}
	if _, ok := c.nsSubscribed[WilcardNamespace]; ok {
		c.send <- []byte(msg.String())
		return true
	}

	return false
}

func (c *WSClient) processMessage(m []byte) {
	var msg WSMessage
	if err := json.Unmarshal(m, &msg); err != nil {
		logging.GetLogger().Errorf("WSServer: Unable to parse the event %s: %s", msg, err.Error())
		return
	}

	if msg.Type == BulkMsgType {
		var bulkMessage WSBulkMessage
		if err := json.Unmarshal([]byte(*msg.Obj), &bulkMessage); err != nil {
			for _, msg := range bulkMessage {
				c.processMessage([]byte(msg))
			}
		}
	} else {
		for _, e := range c.server.nsEventHandlers[msg.Namespace] {
			e.OnMessage(c, msg)
		}

		for _, e := range c.server.nsEventHandlers[WilcardNamespace] {
			e.OnMessage(c, msg)
		}
	}
}

func (c *WSClient) processMessages(wg *sync.WaitGroup, quit chan struct{}) {
	for {
		select {
		case m := <-c.read:
			c.processMessage(m)
		case <-quit:
			wg.Done()
			return
		}
	}
}

func (c *WSClient) readPump() {
	defer func() {
		c.server.unregister <- c
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.server.pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.server.pongWait))
		return nil
	})

	for {
		_, m, err := c.conn.ReadMessage()
		if err != nil {
			if e, ok := err.(*websocket.CloseError); ok && e.Code == websocket.CloseAbnormalClosure {
				logging.GetLogger().Infof("Read on closed connection from %s: %s", c.Host, err.Error())
			} else {
				logging.GetLogger().Errorf("Error while reading websocket from %s: %s", c.Host, err.Error())
			}

			break
		}

		c.read <- m
	}
}

func (c *WSClient) writePump(wg *sync.WaitGroup, quit chan struct{}) {
	ticker := time.NewTicker(c.server.pingPeriod)
	defer ticker.Stop()

	// send a first ping to help firefox and some other client which wait for a
	// first ping before doing something
	c.write(websocket.PingMessage, []byte{})

	for {
		select {
		case message := <-c.send:
			if err := c.write(websocket.TextMessage, message); err != nil {
				logging.GetLogger().Warningf("Error while writing to the websocket: %s", err.Error())
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				logging.GetLogger().Warningf("Error while sending ping to the websocket: %s", err.Error())
			}
		case <-quit:
			wg.Done()
			return
		}
	}
}

func (c *WSClient) write(mt int, message []byte) error {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(mt, message)
}

func (s *WSServer) SendWSMessageTo(msg *WSMessage, host string) bool {
	s.RLock()
	defer s.RUnlock()

	for c := range s.clients {
		if c.Host == host {
			return c.SendWSMessage(msg)
		}
	}

	return false
}

func (s *WSServer) listenAndServe() {
	bulkTicker := time.NewTicker(s.bulkMaxDelay)
	defer bulkTicker.Stop()

	for {
		select {
		case <-s.quit:
			// close all the client so that they will call unregister
			s.Lock()
			for c := range s.clients {
				c.conn.Close()
				delete(s.clients, c)
			}
			s.Unlock()
			return
		case c := <-s.register:
			s.Lock()
			s.clients[c] = true
			s.Unlock()
			for _, e := range s.eventHandlers {
				e.OnRegisterClient(c)
			}
		case c := <-s.unregister:
			for _, e := range s.eventHandlers {
				e.OnUnregisterClient(c)
			}
			s.Lock()
			c.conn.Close()
			delete(s.clients, c)
			s.Unlock()
		case m := <-s.broadcast:
			s.broadcastMessage(m)
		case <-bulkTicker.C:
			s.Lock()
			msgs := s.flushMessages()
			s.Unlock()
			s.broadcastMessages(msgs)
		}
	}
}

func (s *WSServer) broadcastMessage(m broadcastMessage) {
	s.RLock()
	defer s.RUnlock()

	for c := range s.clients {
		if _, ok := c.nsSubscribed[m.namespace]; ok {
			c.send <- []byte(m.message)
		} else if _, ok := c.nsSubscribed[WilcardNamespace]; ok {
			c.send <- []byte(m.message)
		}
	}
}

func nsSubscribed(r *auth.AuthenticatedRequest) map[string]bool {
	subscribed := make(map[string]bool)

	// from header
	if namespaces, ok := r.Header["X-Websocket-Namespace"]; ok {
		for _, ns := range namespaces {
			subscribed[ns] = true
		}
	}

	// from parameter, useful for browser client
	if namespaces, ok := r.URL.Query()["x-websocket-namespace"]; ok {
		for _, ns := range namespaces {
			subscribed[ns] = true
		}
	}

	// if empty use wilcard for backward compatibility
	if len(subscribed) == 0 {
		subscribed[WilcardNamespace] = true
	}

	return subscribed
}

func (s *WSServer) serveMessages(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	// if X-Host-ID specified avoid having twice the same ID
	host := r.Header.Get("X-Host-ID")
	if host != "" {
		s.RLock()
		for c := range s.clients {
			if c.Host == host {
				logging.GetLogger().Errorf("host_id error, connection from %s(%s) conflicts with another one", r.RemoteAddr, host)
				w.WriteHeader(http.StatusConflict)
				s.RUnlock()

				s.unregister <- c

				return
			}
		}
		s.RUnlock()
	}

	conn, err := websocket.Upgrade(w, &r.Request, nil, 1024, 1024)
	if err != nil {
		return
	}

	c := &WSClient{
		read:         make(chan []byte, maxMessages),
		send:         make(chan []byte, maxMessages),
		conn:         conn,
		server:       s,
		Host:         host,
		ClientType:   common.ServiceType(r.Header.Get("X-Client-Type")),
		nsSubscribed: nsSubscribed(r),
	}
	logging.GetLogger().Infof("New WebSocket Connection from %s : URI path %s", conn.RemoteAddr().String(), r.URL.Path)

	s.register <- c

	var wg sync.WaitGroup
	wg.Add(2)

	quit := make(chan struct{})

	go c.writePump(&wg, quit)
	go c.processMessages(&wg, quit)

	c.readPump()

	quit <- struct{}{}
	quit <- struct{}{}

	close(c.read)
	close(c.send)

	wg.Wait()
}

func (s *WSServer) broadcastMessages(msgs []*WSMessage) {
	namespace := ""
	var bulkMessage WSBulkMessage
	for _, msg := range msgs {
		if namespace != "" && msg.Namespace != namespace {
			s.BroadcastWSMessage(NewWSMessage(namespace, BulkMsgType, bulkMessage))
			bulkMessage = bulkMessage[:0]
		}
		b, _ := json.Marshal(msg)
		raw := json.RawMessage(b)
		namespace = msg.Namespace
		bulkMessage = append(bulkMessage, raw)
	}
	s.BroadcastWSMessage(NewWSMessage(namespace, BulkMsgType, bulkMessage))
}

func (s *WSServer) flushMessages() (msgs []*WSMessage) {
	msgs = make([]*WSMessage, len(s.eventBuffer))
	copy(msgs, s.eventBuffer)
	s.eventBuffer = s.eventBuffer[:0]
	return
}

func (s *WSServer) BroadcastWSMessage(msg *WSMessage) {
	s.broadcast <- broadcastMessage{namespace: msg.Namespace, message: msg.String()}
}

func (s *WSServer) QueueBroadcastWSMessage(msg *WSMessage) {
	s.Lock()
	defer s.Unlock()

	s.eventBuffer = append(s.eventBuffer, msg)
	if len(s.eventBuffer) == s.bulkMaxMsgs {
		s.broadcastMessages(s.flushMessages())
	}
}

func (s *WSServer) ListenAndServe() {
	s.wg.Add(1)
	defer s.wg.Done()

	s.listening.Store(true)
	s.listenAndServe()
}

func (s *WSServer) Stop() {
	s.quit <- true
	if s.listening.Load() == true {
		s.wg.Wait()
	}
	s.listening.Store(false)
}

func (s *WSServer) AddEventHandler(h WSServerEventHandler, namespaces []string) {
	s.eventHandlers = append(s.eventHandlers, h)

	// add this handler per namespace
	for _, ns := range namespaces {
		if _, ok := s.nsEventHandlers[ns]; !ok {
			s.nsEventHandlers[ns] = []WSServerEventHandler{h}
		} else {
			s.nsEventHandlers[ns] = append(s.nsEventHandlers[ns], h)
		}
	}
}

func (s *WSServer) GetClients() (clients []*WSClient) {
	s.RLock()
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.RUnlock()
	return clients
}

func (s *WSServer) GetClientsByType(clientType common.ServiceType) (clients []*WSClient) {
	s.RLock()
	for client := range s.clients {
		if client.ClientType == clientType {
			clients = append(clients, client)
		}
	}
	s.RUnlock()
	return clients
}

func NewWSServer(server *Server, pongWait time.Duration, bulkMaxMsgs int, bulkMaxDelay time.Duration, endpoint string) *WSServer {
	s := &WSServer{
		Server:          server,
		broadcast:       make(chan broadcastMessage, 100000),
		quit:            make(chan bool, 1),
		register:        make(chan *WSClient),
		unregister:      make(chan *WSClient),
		clients:         make(map[*WSClient]bool),
		nsEventHandlers: make(map[string][]WSServerEventHandler),
		pongWait:        pongWait,
		pingPeriod:      (pongWait * 8) / 10,
		bulkMaxMsgs:     bulkMaxMsgs,
		bulkMaxDelay:    bulkMaxDelay,
	}

	server.HandleFunc(endpoint, s.serveMessages)

	return s
}

func NewWSServerFromConfig(server *Server, endpoint string) *WSServer {
	pongTimeout := config.GetConfig().GetInt("ws_pong_timeout")
	bulkMaxMsgs := config.GetConfig().GetInt("ws_bulk_maxmsgs")
	bulkMaxDelay := config.GetConfig().GetInt("ws_bulk_maxdelay")

	return NewWSServer(server, time.Duration(pongTimeout)*time.Second, bulkMaxMsgs, time.Duration(bulkMaxDelay)*time.Second, endpoint)
}
