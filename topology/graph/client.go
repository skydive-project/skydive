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
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/redhat-cip/skydive/logging"
)

type EventListener interface {
	OnConnected()
	OnDisconnected()
}

type AutoGraphClient struct {
	Client *AsyncClient
	Graph  *Graph
}

type AsyncClient struct {
	sync.RWMutex
	Addr      string
	Port      int
	messages  chan string
	listeners []EventListener
	connected bool
}

func (c *AutoGraphClient) triggerResync() {
	logging.GetLogger().Info("Start a resync of the graph")

	hostname, err := os.Hostname()
	if err != nil {
		logging.GetLogger().Error("Unable to retrieve the hostname: %s", err.Error())
		return
	}

	c.Graph.Lock()
	defer c.Graph.Unlock()

	// request for deletion of everythin belonging to host node
	root := c.Graph.GetNode(Identifier(hostname))
	if root == nil {
		return
	}
	c.SendGraphMessage(GraphMessage{"SubtreeDeleted", root})

	// re-added all the nodes and edges
	nodes := c.Graph.GetNodes()
	for _, n := range nodes {
		c.SendGraphMessage(GraphMessage{"NodeAdded", n})
	}

	edges := c.Graph.GetEdges()
	for _, n := range edges {
		c.SendGraphMessage(GraphMessage{"EdgeAdded", n})
	}
}

func (c *AutoGraphClient) OnConnected() {
	c.triggerResync()
}

func (c *AutoGraphClient) OnDisconnected() {
}

func (c *AutoGraphClient) OnNodeUpdated(n *Node) {
	c.SendGraphMessage(GraphMessage{"NodeUpdated", n})
}

func (c *AutoGraphClient) OnNodeAdded(n *Node) {
	c.SendGraphMessage(GraphMessage{"NodeAdded", n})
}

func (c *AutoGraphClient) OnNodeDeleted(n *Node) {
	c.SendGraphMessage(GraphMessage{"NodeDeleted", n})
}

func (c *AutoGraphClient) OnEdgeUpdated(e *Edge) {
	c.SendGraphMessage(GraphMessage{"EdgeUpdated", e})
}

func (c *AutoGraphClient) OnEdgeAdded(e *Edge) {
	c.SendGraphMessage(GraphMessage{"EdgeAdded", e})
}

func (c *AutoGraphClient) OnEdgeDeleted(e *Edge) {
	c.SendGraphMessage(GraphMessage{"EdgeDeleted", e})
}

func (c *AutoGraphClient) SendGraphMessage(e GraphMessage) {
	if !c.Client.IsConnected() {
		return
	}

	c.Client.messages <- e.String()
}

// SetAutoGraphUpdate the client will manage automatically the update event of the graph
// and will forword then to the server. To do that it register itself as a graph listener
// and as a connection listener as well. So that it can resync the graph when the connection
// is lost.
func (c *AsyncClient) SetAutoGraphUpdate(g *Graph) {
	a := &AutoGraphClient{
		Client: c,
		Graph:  g,
	}

	g.AddEventListener(a)
	c.AddListener(a)
}

func (c *AsyncClient) IsConnected() bool {
	c.Lock()
	defer c.Unlock()

	return c.connected
}

func (c *AsyncClient) sendMessage(conn *websocket.Conn, msg string) error {
	w, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	io.WriteString(w, msg)

	w.Close()

	return nil
}

func (c *AsyncClient) connect() {
	host := c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10)

	conn, err := net.Dial("tcp", host)
	if err != nil {
		logging.GetLogger().Error("Connection to the WebSocket server failed: %s", err.Error())
		return
	}
	defer conn.Close()

	endpoint := "ws://" + host + "/ws/graph"
	u, err := url.Parse(endpoint)
	if err != nil {
		logging.GetLogger().Error("Unable to parse the WebSocket Endpoint %s: %s", endpoint, err.Error())
		return
	}

	wsConn, _, err := websocket.NewClient(conn, u, http.Header{"Origin": {endpoint}}, 1024, 1024)
	if err != nil {
		logging.GetLogger().Error("Unable to create a WebSocket connection: %s", err.Error())
		return
	}
	defer wsConn.Close()

	logging.GetLogger().Info("Connected to %s", endpoint)

	c.Lock()
	c.connected = true
	c.Unlock()

	// notify connected
	for _, l := range c.listeners {
		l.OnConnected()
	}

	var msg string
	for {
		msg = <-c.messages
		err := c.sendMessage(wsConn, msg)
		if err != nil {
			logging.GetLogger().Error("Error while writing to the WebSocket: %s", err.Error())
			break
		}
	}
}

func (c *AsyncClient) Connect() {
	go func() {
		for {
			c.connect()

			c.Lock()
			connected := c.connected
			c.connected = false
			c.Unlock()

			if connected {
				for _, l := range c.listeners {
					l.OnDisconnected()
				}
			}

			time.Sleep(1 * time.Second)
		}
	}()
}

func (c *AsyncClient) AddListener(l EventListener) {
	c.listeners = append(c.listeners, l)
}

// Create new chat client.
func NewAsyncClient(addr string, port int) *AsyncClient {
	return &AsyncClient{
		Addr:      addr,
		Port:      port,
		messages:  make(chan string, 500),
		connected: false,
	}
}
