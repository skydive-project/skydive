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

// TopologyForwarder forwards the topology to only one master server.
// When switching from one analyzer to another one the agent does a full
// re-sync since some messages could have been lost.
type TopologyForwarder struct {
	masterElection *shttp.WSMasterElection
	graph          *graph.Graph
	host           string
}

func (t *TopologyForwarder) triggerResync() {
	logging.GetLogger().Infof("Start a re-sync for %s", t.host)

	t.graph.RLock()
	defer t.graph.RUnlock()

	// request for deletion of everything belonging this host
	t.masterElection.SendMessageToMaster(shttp.NewWSJSONMessage(graph.Namespace, graph.HostGraphDeletedMsgType, t.host))

	// re-add all the nodes and edges
	t.masterElection.SendMessageToMaster(shttp.NewWSJSONMessage(graph.Namespace, graph.SyncMsgType, t.graph))
}

// OnNewMaster is called by the master election mechanism when a new master is elected. In
// such case a "Re-sync" is triggerd in order to be in sync with the new master.
func (t *TopologyForwarder) OnNewMaster(c shttp.WSSpeaker) {
	if c == nil {
		logging.GetLogger().Warn("Lost connection to master")
	} else {
		addr, port := c.GetAddrPort()
		logging.GetLogger().Infof("Using %s:%d as master of topology forwarder", addr, port)
		t.triggerResync()
	}
}

// OnNodeUpdated graph node updated event. Implements the GraphEventListener interface.
func (t *TopologyForwarder) OnNodeUpdated(n *graph.Node) {
	t.masterElection.SendMessageToMaster(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeUpdatedMsgType, n))
}

// OnNodeAdded graph node added event. Implements the GraphEventListener interface.
func (t *TopologyForwarder) OnNodeAdded(n *graph.Node) {
	t.masterElection.SendMessageToMaster(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeAddedMsgType, n))
}

// OnNodeDeleted graph node deleted event. Implements the GraphEventListener interface.
func (t *TopologyForwarder) OnNodeDeleted(n *graph.Node) {
	t.masterElection.SendMessageToMaster(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeDeletedMsgType, n))
}

// OnEdgeUpdated graph edge updated event. Implements the GraphEventListener interface.
func (t *TopologyForwarder) OnEdgeUpdated(e *graph.Edge) {
	t.masterElection.SendMessageToMaster(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeUpdatedMsgType, e))
}

// OnEdgeAdded graph edge added event. Implements the GraphEventListener interface.
func (t *TopologyForwarder) OnEdgeAdded(e *graph.Edge) {
	t.masterElection.SendMessageToMaster(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeAddedMsgType, e))
}

// OnEdgeDeleted graph edge deleted event. Implements the GraphEventListener interface.
func (t *TopologyForwarder) OnEdgeDeleted(e *graph.Edge) {
	t.masterElection.SendMessageToMaster(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeDeletedMsgType, e))
}

// GetMaster returns the current analyzer the agent is sending its events to
func (t *TopologyForwarder) GetMaster() shttp.WSSpeaker {
	return t.masterElection.GetMaster()
}

// NewTopologyForwarder returns a new Graph forwarder which forwards event of the given graph
// to the given WebSocket JSON speakers.
func NewTopologyForwarder(host string, g *graph.Graph, pool shttp.WSJSONSpeakerPool) *TopologyForwarder {
	masterElection := shttp.NewWSMasterElection(pool)

	t := &TopologyForwarder{
		masterElection: masterElection,
		graph:          g,
		host:           host,
	}

	masterElection.AddEventHandler(t)
	g.AddEventListener(t)

	return t
}

// NewTopologyForwarderFromConfig creates a TopologyForwarder from configuration
func NewTopologyForwarderFromConfig(g *graph.Graph, pool shttp.WSJSONSpeakerPool) *TopologyForwarder {
	host := config.GetString("host_id")
	return NewTopologyForwarder(host, g, pool)
}
