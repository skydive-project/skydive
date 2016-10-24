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
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

type Forwarder struct {
	shttp.DefaultWSClientEventHandler
	Client *shttp.WSAsyncClient
	Graph  *Graph
	Host   string
}

func (c *Forwarder) triggerResync() {
	logging.GetLogger().Infof("Start a resync of the graph")

	c.Graph.RLock()
	defer c.Graph.RUnlock()

	// request for deletion of everything belonging to Root node
	c.Client.SendWSMessage(shttp.NewWSMessage(Namespace, "HostGraphDeleted", c.Host))

	// re-added all the nodes and edges
	nodes := c.Graph.GetNodes(Metadata{})
	for _, n := range nodes {
		c.Client.SendWSMessage(shttp.NewWSMessage(Namespace, "NodeAdded", n))
	}

	edges := c.Graph.GetEdges(Metadata{})
	for _, e := range edges {
		c.Client.SendWSMessage(shttp.NewWSMessage(Namespace, "EdgeAdded", e))
	}
}

func (c *Forwarder) OnConnected() {
	c.triggerResync()
}

func (c *Forwarder) OnNodeUpdated(n *Node) {
	c.Client.SendWSMessage(shttp.NewWSMessage(Namespace, "NodeUpdated", n))
}

func (c *Forwarder) OnNodeAdded(n *Node) {
	c.Client.SendWSMessage(shttp.NewWSMessage(Namespace, "NodeAdded", n))
}

func (c *Forwarder) OnNodeDeleted(n *Node) {
	c.Client.SendWSMessage(shttp.NewWSMessage(Namespace, "NodeDeleted", n))
}

func (c *Forwarder) OnEdgeUpdated(e *Edge) {
	c.Client.SendWSMessage(shttp.NewWSMessage(Namespace, "EdgeUpdated", e))
}

func (c *Forwarder) OnEdgeAdded(e *Edge) {
	c.Client.SendWSMessage(shttp.NewWSMessage(Namespace, "EdgeAdded", e))
}

func (c *Forwarder) OnEdgeDeleted(e *Edge) {
	c.Client.SendWSMessage(shttp.NewWSMessage(Namespace, "EdgeDeleted", e))
}

func NewForwarder(c *shttp.WSAsyncClient, g *Graph, host string) *Forwarder {
	f := &Forwarder{
		Client: c,
		Graph:  g,
		Host:   host,
	}

	g.AddEventListener(f)
	c.AddEventHandler(f)

	return f
}
