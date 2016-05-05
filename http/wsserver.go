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

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
)

const (
	Namespace      = "WSServer"
	writeWait      = 10 * time.Second
	maxMessageSize = 1024 * 1024
)

type WSClient struct {
	conn   *websocket.Conn
	read   chan []byte
	send   chan []byte
	server *WSServer
	host   string
}

type WSMessage struct {
	Namespace string
	Type      string
	Obj       interface{}
}

type WSServerEventHandler interface {
	OnMessage(c *WSClient, m WSMessage)
	OnRegisterClient(c *WSClient)
	OnUnregisterClient(c *WSClient)
}

type DefaultWSServerEventHandler struct {
}

type WSServer struct {
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

func UnmarshalWSMessage(b []byte) (WSMessage, error) {
	msg := WSMessage{}
	if err := json.Unmarshal(b, &msg); err != nil {
		return msg, err
	}

	return msg, nil
}

func (d *DefaultWSServerEventHandler) OnMessage(c *WSClient, m WSMessage) {
}

func (d *DefaultWSServerEventHandler) OnRegisterClient(c *WSClient) {
}

func (d *DefaultWSServerEventHandler) OnUnregisterClient(c *WSClient) {
}

func (c *WSClient) SendWSMessage(msg WSMessage) {
	c.send <- []byte(msg.String())
}

func (c *WSClient) processMessage(m []byte) {
	msg, err := UnmarshalWSMessage(m)
	if err != nil {
		logging.GetLogger().Errorf("WSServer: Unable to parse the event %s: %s", msg, err.Error())
		return
	}

	if msg.Namespace == Namespace {
		switch msg.Type {
		case "Hello":
			c.host = msg.Obj.(string)

			logging.GetLogger().Infof("Hello received from WSClient: %s", c.host)
		}
	} else {
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
		c.conn.Close()
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
			break
		}

		c.read <- m
	}
}

func (c *WSClient) writePump(wg *sync.WaitGroup, quit chan struct{}) {
	ticker := time.NewTicker(c.server.pingPeriod)

	defer func() {
		ticker.Stop()
		c.conn.Close()
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

func (s *WSServer) SendWSMessageTo(msg WSMessage, host string) bool {
	for c := range s.clients {
		if c.host == host {
			c.SendWSMessage(msg)
			return true
		}
	}

	return false
}

func (s *WSServer) listenAndServe() {
	quit := false

	for {
		select {
		case <-s.quit:
			if len(s.clients) == 0 {
				return
			}

			// close all the client so that they will call unregister
			for c := range s.clients {
				c.conn.Close()
			}

			quit = true
		case c := <-s.register:
			s.clients[c] = true
			for _, e := range s.eventHandlers {
				e.OnRegisterClient(c)
			}
		case c := <-s.unregister:
			for _, e := range s.eventHandlers {
				e.OnUnregisterClient(c)
			}
			delete(s.clients, c)

			// if quit has been requested and there is no more clients then leave
			if quit && len(s.clients) == 0 {
				return
			}
		case m := <-s.broadcast:
			s.broadcastMessage(m)
		}
	}
}

func (s *WSServer) broadcastMessage(m string) {
	for c := range s.clients {
		select {
		case c.send <- []byte(m):
		default:
			delete(s.clients, c)
		}
	}
}

func (s *WSServer) serveMessages(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(w, &r.Request, nil)
	if err != nil {
		return
	}

	c := &WSClient{
		read:   make(chan []byte, maxMessageSize),
		send:   make(chan []byte, maxMessageSize),
		conn:   conn,
		server: s,
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

func (s *WSServer) BroadcastWSMessage(msg WSMessage) {
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
