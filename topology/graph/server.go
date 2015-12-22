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

package graph

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/redhat-cip/skydive/logging"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 1024
)

type Server struct {
	Graph    *Graph
	Router   *mux.Router
	wsServer *WSServer
	Host     string
}

type WSClient struct {
	conn   *websocket.Conn
	send   chan []byte
	server *WSServer
}

type WSServer struct {
	Graph      *Graph
	clients    map[*WSClient]bool
	broadcast  chan string
	register   chan *WSClient
	unregister chan *WSClient
}

func (c *WSClient) processGraphMessage(msg GraphMessage) {
	g := c.server.Graph

	switch msg.Type {
	case "SyncRequest":
		reply := GraphMessage{
			Type: "SyncReply",
			Obj:  c.server.Graph,
		}
		c.send <- []byte(reply.String())

	case "SubtreeDeleted":
		n := msg.Obj.(*Node)

		logging.GetLogger().Debug("Got SubtreeDeleted event from the node %s", n.ID)

		node := g.GetNode(n.ID)
		if node != nil {
			g.SubtreeDel(node)
		}
	case "NodeUpdated":
		n := msg.Obj.(*Node)
		node := g.GetNode(n.ID)
		if node != nil {
			node.SetMetadatas(n.Metadatas)
		}
	case "NodeDeleted":
		g.DelNode(msg.Obj.(*Node))
	case "NodeAdded":
		n := msg.Obj.(*Node)
		if g.GetNode(n.ID) == nil {
			g.AddNode(n)
		}
	case "EdgeUpdated":
		e := msg.Obj.(*Edge)
		edge := g.GetEdge(e.ID)
		if edge != nil {
			edge.SetMetadatas(e.Metadatas)
		}
	case "EdgeDeleted":
		g.DelEdge(msg.Obj.(*Edge))
	case "EdgeAdded":
		e := msg.Obj.(*Edge)
		if g.GetEdge(e.ID) == nil {
			g.AddEdge(e)
		}
	}
}

func (c *WSClient) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, p, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		c.server.Graph.Lock()

		msg, err := c.server.Graph.UnmarshalGraphMessage(p)
		if err == nil {
			c.processGraphMessage(msg)
		} else {
			logging.GetLogger().Error("Unable to parse the event %s: %s", msg, err.Error())
		}

		c.server.Graph.Unlock()
	}
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				logging.GetLogger().Warning("Error while writing to the websocket: %s", err.Error())
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (c *WSClient) write(mt int, message []byte) error {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(mt, message)
}

func (s *WSServer) ListenAndServe() {
	for {
		select {
		case c := <-s.register:
			s.clients[c] = true
			break

		case c := <-s.unregister:
			_, ok := s.clients[c]
			if ok {
				delete(s.clients, c)
				close(c.send)
			}
			break

		case m := <-s.broadcast:
			s.broadcastMessage(m)
			break
		}
	}
}

func (s *WSServer) broadcastMessage(m string) {
	for c := range s.clients {
		select {
		case c.send <- []byte(m):
			break

		// We can't reach the client
		default:
			close(c.send)
			delete(s.clients, c)
		}
	}
}

func (s *Server) sendGraphUpdateEvent(g GraphMessage) {
	s.wsServer.broadcast <- g.String()
}

func (s *Server) OnNodeUpdated(n *Node) {
	s.sendGraphUpdateEvent(GraphMessage{"NodeUpdated", n})
}

func (s *Server) OnNodeAdded(n *Node) {
	s.sendGraphUpdateEvent(GraphMessage{"NodeAdded", n})
}

func (s *Server) OnNodeDeleted(n *Node) {
	s.sendGraphUpdateEvent(GraphMessage{"NodeDeleted", n})
}

func (s *Server) OnEdgeUpdated(e *Edge) {
	s.sendGraphUpdateEvent(GraphMessage{"EdgeUpdated", e})
}

func (s *Server) OnEdgeAdded(e *Edge) {
	s.sendGraphUpdateEvent(GraphMessage{"EdgeAdded", e})
}

func (s *Server) OnEdgeDeleted(e *Edge) {
	s.sendGraphUpdateEvent(GraphMessage{"EdgeDeleted", e})
}

func (s *Server) serveGraphMessages(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	c := &WSClient{
		send:   make(chan []byte, maxMessageSize),
		conn:   conn,
		server: s.wsServer,
	}
	logging.GetLogger().Info("New WebSocket Connection from %s", conn.RemoteAddr().String())

	s.wsServer.register <- c

	go c.writePump()
	c.readPump()
}

func (s *Server) ListenAndServe() {
	s.Graph.AddEventListener(s)

	s.Router.HandleFunc("/ws/graph", s.serveGraphMessages)

	s.wsServer.ListenAndServe()
}

func NewServer(g *Graph, router *mux.Router) *Server {
	return &Server{
		Graph:  g,
		Router: router,
		wsServer: &WSServer{
			Graph:      g,
			broadcast:  make(chan string, 500),
			register:   make(chan *WSClient),
			unregister: make(chan *WSClient),
			clients:    make(map[*WSClient]bool),
		},
	}
}
