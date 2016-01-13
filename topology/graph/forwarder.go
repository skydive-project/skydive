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
	"os"

	"github.com/redhat-cip/skydive/logging"
)

type Forwarder struct {
	Client *AsyncClient
	Graph  *Graph
}

func (c *Forwarder) triggerResync() {
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
	c.Client.SendGraphMessage(GraphMessage{"SubtreeDeleted", root})

	// re-added all the nodes and edges
	nodes := c.Graph.GetNodes()
	for _, n := range nodes {
		c.Client.SendGraphMessage(GraphMessage{"NodeAdded", n})
	}

	edges := c.Graph.GetEdges()
	for _, e := range edges {
		c.Client.SendGraphMessage(GraphMessage{"EdgeAdded", e})
	}
}

func (c *Forwarder) OnConnected() {
	c.triggerResync()
}

func (c *Forwarder) OnDisconnected() {
}

func (c *Forwarder) OnNodeUpdated(n *Node) {
	c.Client.SendGraphMessage(GraphMessage{"NodeUpdated", n})
}

func (c *Forwarder) OnNodeAdded(n *Node) {
	c.Client.SendGraphMessage(GraphMessage{"NodeAdded", n})
}

func (c *Forwarder) OnNodeDeleted(n *Node) {
	c.Client.SendGraphMessage(GraphMessage{"NodeDeleted", n})
}

func (c *Forwarder) OnEdgeUpdated(e *Edge) {
	c.Client.SendGraphMessage(GraphMessage{"EdgeUpdated", e})
}

func (c *Forwarder) OnEdgeAdded(e *Edge) {
	c.Client.SendGraphMessage(GraphMessage{"EdgeAdded", e})
}

func (c *Forwarder) OnEdgeDeleted(e *Edge) {
	c.Client.SendGraphMessage(GraphMessage{"EdgeDeleted", e})
}

func NewForwarder(c *AsyncClient, g *Graph) *Forwarder {
	f := &Forwarder{
		Client: c,
		Graph:  g,
	}

	g.AddEventListener(f)
	c.AddListener(f)

	return f
}
