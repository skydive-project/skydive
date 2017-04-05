/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package agent

import (
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// TopologyForwarder forwards the topology to only one analyzer. Analyzers will forward
// messages between them in order to be synchronized. When switching from one analyzer to another one
// the agent will do a full re-sync because some messages could have been lost.
type TopologyForwarder struct {
	shttp.DefaultWSClientEventHandler
	WSAsyncClientPool *shttp.WSAsyncClientPool
	Graph             *graph.Graph
	Host              string
	master            *shttp.WSAsyncClient
}

func (t *TopologyForwarder) triggerResync() {
	logging.GetLogger().Infof("Start a re-sync for %s", t.Host)

	t.Graph.RLock()
	defer t.Graph.RUnlock()

	// request for deletion of everything belonging to Root node
	t.WSAsyncClientPool.SendWSMessageToMaster(shttp.NewWSMessage(graph.Namespace, graph.HostGraphDeletedMsgType, t.Host))

	// re-add all the nodes and edges
	t.WSAsyncClientPool.SendWSMessageToMaster(shttp.NewWSMessage(graph.Namespace, graph.SyncReplyMsgType, t.Graph))
}

func (t *TopologyForwarder) OnConnected(c *shttp.WSAsyncClient) {
	if c == t.WSAsyncClientPool.MasterClient() {
		// keep a track of the current master in order to detect master disconnection
		t.master = c

		logging.GetLogger().Infof("Using %s:%d as master of topology forwarder", c.Addr, c.Port)
		t.triggerResync()
	}
}

func (t *TopologyForwarder) OnDisconnected(c *shttp.WSAsyncClient) {
	if c == t.master {
		t.master = t.WSAsyncClientPool.MasterClient()

		// re-sync as we changed of master and some message could have lost by the previous one
		t.triggerResync()
	}
}

func (t *TopologyForwarder) OnNodeUpdated(n *graph.Node) {
	t.WSAsyncClientPool.SendWSMessageToMaster(shttp.NewWSMessage(graph.Namespace, graph.NodeUpdatedMsgType, n))
}

func (t *TopologyForwarder) OnNodeAdded(n *graph.Node) {
	t.WSAsyncClientPool.SendWSMessageToMaster(shttp.NewWSMessage(graph.Namespace, graph.NodeAddedMsgType, n))
}

func (t *TopologyForwarder) OnNodeDeleted(n *graph.Node) {
	t.WSAsyncClientPool.SendWSMessageToMaster(shttp.NewWSMessage(graph.Namespace, graph.NodeDeletedMsgType, n))
}

func (t *TopologyForwarder) OnEdgeUpdated(e *graph.Edge) {
	t.WSAsyncClientPool.SendWSMessageToMaster(shttp.NewWSMessage(graph.Namespace, graph.EdgeUpdatedMsgType, e))
}

func (t *TopologyForwarder) OnEdgeAdded(e *graph.Edge) {
	t.WSAsyncClientPool.SendWSMessageToMaster(shttp.NewWSMessage(graph.Namespace, graph.EdgeAddedMsgType, e))
}

func (t *TopologyForwarder) OnEdgeDeleted(e *graph.Edge) {
	t.WSAsyncClientPool.SendWSMessageToMaster(shttp.NewWSMessage(graph.Namespace, graph.EdgeDeletedMsgType, e))
}

func NewTopologyForwarder(host string, g *graph.Graph, wspool *shttp.WSAsyncClientPool) *TopologyForwarder {
	t := &TopologyForwarder{
		WSAsyncClientPool: wspool,
		Graph:             g,
		Host:              host,
	}

	g.AddEventListener(t)
	wspool.AddEventHandler(t, []string{})

	return t
}

func NewTopologyForwarderFromConfig(g *graph.Graph, wspool *shttp.WSAsyncClientPool) *TopologyForwarder {
	host := config.GetConfig().GetString("host_id")
	return NewTopologyForwarder(host, g, wspool)
}
