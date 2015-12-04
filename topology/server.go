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

package topology

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/statics"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 1024
)

type WSClient struct {
	conn   *websocket.Conn
	send   chan []byte
	server *WSServer
}

type WSServer struct {
	clients    map[*WSClient]bool
	broadcast  chan string
	register   chan *WSClient
	unregister chan *WSClient
}

type Server struct {
	Graph    *Graph
	router   *mux.Router
	wsServer *WSServer
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
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
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

func (s *WSServer) start() {
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

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type GraphUpdate struct {
	Event string
	Obj   interface{}
}

func (g GraphUpdate) String() string {
	j, _ := json.Marshal(g)
	return string(j)
}

func (s *Server) sendGraphUpdate(g GraphUpdate) {
	s.wsServer.broadcast <- g.String()
}

func (s *Server) OnNodeUpdated(n *Node) {
	s.sendGraphUpdate(GraphUpdate{"NodeUpdated", n})
}

func (s *Server) OnNodeAdded(n *Node) {
	s.sendGraphUpdate(GraphUpdate{"NodeAdded", n})
}

func (s *Server) OnNodeDeleted(n *Node) {
	s.sendGraphUpdate(GraphUpdate{"NodeDeleted", n})
}

func (s *Server) OnEdgeUpdated(e *Edge) {
	s.sendGraphUpdate(GraphUpdate{"EdgeUpdated", e})
}

func (s *Server) OnEdgeAdded(e *Edge) {
	s.sendGraphUpdate(GraphUpdate{"EdgeAdded", e})
}

func (s *Server) OnEdgeDeleted(e *Edge) {
	s.sendGraphUpdate(GraphUpdate{"EdgeDeleted", e})
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	html, err := statics.Asset("statics/topology.html")
	if err != nil {
		logging.GetLogger().Panic("Unable to find the topology asset")
	}

	t := template.New("topology template")

	t, err = t.Parse(string(html))
	if err != nil {
		panic(err)
	}

	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	var data = &struct {
		Hostname string
	}{
		Hostname: host,
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	t.Execute(w, data)
}

func (s *Server) serveTopologyUpdates(w http.ResponseWriter, r *http.Request) {
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

	s.wsServer.register <- c

	go c.writePump()
	c.readPump()
}

func (s *Server) TopologyIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(s.Graph); err != nil {
		panic(err)
	}
}

func (s *Server) TopologyShow(w http.ResponseWriter, r *http.Request) {
	/*vars := mux.Vars(r)
	host := vars["host"]

	if topo, ok := s.MultiNodeTopology.Get(host); ok {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(topo); err != nil {
			panic(err)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}*/
}

func (s *Server) TopologyInsert(w http.ResponseWriter, r *http.Request) {
	/*vars := mux.Vars(r)
	host := vars["host"]

	topology := NewTopology(host)

	err := json.NewDecoder(r.Body).Decode(topology)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.MultiNodeTopology.Add(topology)

	if topo, ok := s.MultiNodeTopology.Get(host); ok {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(topo); err != nil {
			panic(err)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}

	//logging.GetLogger().Debug(topology.String())*/
}

func (s *Server) RegisterStaticEndpoints() {

	// static routes
	s.router.HandleFunc("/static/topology", s.serveIndex)
}

func (s *Server) RegisterWebSocketEndpoint() {
	// add server itself as a graph listener so that it will genere ws updates
	s.Graph.AddEventListener(s)

	// static routes
	s.router.HandleFunc("/ws/topology", s.serveTopologyUpdates)
}

func (s *Server) RegisterRpcEndpoints() {
	routes := []Route{
		Route{
			"TopologiesIndex",
			"GET",
			"/rpc/topology",
			s.TopologyIndex,
		},
		Route{
			"TopologyShow",
			"GET",
			"/rpc/topology/{host}",
			s.TopologyShow,
		},
		Route{
			"TopologyInsert",
			"POST",
			"/rpc/topology/{host}",
			s.TopologyInsert,
		},
	}

	for _, route := range routes {
		s.router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}
}

func (s *Server) ListenAndServe(port int64) {
	go s.wsServer.start()

	http.ListenAndServe(":"+strconv.FormatInt(port, 10), s.router)
}

func NewServer(g *Graph) *Server {
	return &Server{
		Graph:  g,
		router: mux.NewRouter().StrictSlash(true),
		wsServer: &WSServer{
			broadcast:  make(chan string, 100),
			register:   make(chan *WSClient),
			unregister: make(chan *WSClient),
			clients:    make(map[*WSClient]bool),
		},
	}
}
