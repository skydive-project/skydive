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

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

const (
	Namespace      = "WSServer"
	writeWait      = 10 * time.Second
	maxMessages    = 1024
	maxMessageSize = 0
)

type WSClient struct {
	conn   *websocket.Conn
	read   chan []byte
	send   chan []byte
	server *WSServer
	host   string
	kind   string
}

type WSMessage struct {
	Namespace string
	Type      string
	UUID      string `json:",omitempty"`
	Obj       *json.RawMessage
	Status    int
}

type WSServerEventHandler interface {
	OnMessage(c *WSClient, m WSMessage)
	OnRegisterClient(c *WSClient)
	OnUnregisterClient(c *WSClient)
}

type DefaultWSServerEventHandler struct {
}

type WSServer struct {
	sync.RWMutex
	DefaultWSServerEventHandler
	Server        *Server
	eventHandlers []WSServerEventHandler
	clients       map[*WSClient]bool
	broadcast     chan string
	quit          chan bool
	register      chan *WSClient
	unregister    chan *WSClient
	pongWait      time.Duration
	pingPeriod    time.Duration
	wg            sync.WaitGroup
	listening     atomic.Value
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

func (c *WSClient) GetHostInfo() (string, string) {
	return c.host, c.kind
}

func (c *WSClient) SendWSMessage(msg *WSMessage) {
	c.send <- []byte(msg.String())
}

func (c *WSClient) processMessage(m []byte) {
	var msg WSMessage
	if err := json.Unmarshal(m, &msg); err != nil {
		logging.GetLogger().Errorf("WSServer: Unable to parse the event %s: %s", msg, err.Error())
		return
	}

	if msg.Namespace != Namespace {
		for _, e := range c.server.eventHandlers {
			e.OnMessage(c, msg)
		}
	}
}

func (c *WSClient) processMessages(wg *sync.WaitGroup, quit chan struct{}) {
	for {
		select {
		case m, ok := <-c.read:
			if !ok {
				wg.Done()
				return
			}
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
			if err != websocket.ErrCloseSent {
				logging.GetLogger().Errorf("Error while reading websocket from %s: %s", c.host, err.Error())
			}
			break
		}

		c.read <- m
	}
}

func (c *WSClient) writePump(wg *sync.WaitGroup, quit chan struct{}) {
	ticker := time.NewTicker(c.server.pingPeriod)

	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				wg.Done()
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				logging.GetLogger().Warningf("Error while writing to the websocket: %s", err.Error())
				wg.Done()
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				wg.Done()
				return
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
		if c.host == host {
			c.SendWSMessage(msg)
			return true
		}
	}

	return false
}

func (s *WSServer) listenAndServe() {
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
		}
	}
}

func (s *WSServer) broadcastMessage(m string) {
	s.RLock()
	defer s.RUnlock()

	for c := range s.clients {
		c.send <- []byte(m)
	}
}

func (s *WSServer) serveMessages(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	// if X-Host-ID specified avoid having twice the same ID
	hostID := r.Header.Get("X-Host-ID")
	if hostID != "" {
		s.RLock()
		for c := range s.clients {
			if c.host == hostID {
				logging.GetLogger().Errorf("host_id error, connection from %s(%s) conflicts with another one", r.RemoteAddr, hostID)
				w.WriteHeader(http.StatusConflict)
				s.RUnlock()

				s.unregister <- c

				return
			}
		}
		s.RUnlock()
	}

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(w, &r.Request, nil)
	if err != nil {
		return
	}

	c := &WSClient{
		read:   make(chan []byte, maxMessages),
		send:   make(chan []byte, maxMessages),
		conn:   conn,
		server: s,
		host:   hostID,
		kind:   r.Header.Get("X-Client-Type"),
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

func (s *WSServer) BroadcastWSMessage(msg *WSMessage) {
	s.broadcast <- msg.String()
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

func (s *WSServer) AddEventHandler(h WSServerEventHandler) {
	s.eventHandlers = append(s.eventHandlers, h)
}

func (s *WSServer) GetClients() (clients []*WSClient) {
	s.RLock()
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.RUnlock()
	return clients
}

func (s *WSServer) GetClientsByType(kind string) (clients []*WSClient) {
	s.RLock()
	for client := range s.clients {
		if client.kind == kind {
			clients = append(clients, client)
		}
	}
	s.RUnlock()
	return clients
}

func NewWSServer(server *Server, pongWait time.Duration, endpoint string) *WSServer {
	s := &WSServer{
		Server:     server,
		broadcast:  make(chan string, 500),
		quit:       make(chan bool, 1),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		clients:    make(map[*WSClient]bool),
		pongWait:   pongWait,
		pingPeriod: (pongWait * 8) / 10,
	}

	server.HandleFunc(endpoint, s.serveMessages)

	return s
}

func NewWSServerFromConfig(server *Server, endpoint string) *WSServer {
	w := config.GetConfig().GetInt("ws_pong_timeout")

	return NewWSServer(server, time.Duration(w)*time.Second, endpoint)
}
