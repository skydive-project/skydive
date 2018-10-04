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

package analyzer

import (
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	ws "github.com/skydive-project/skydive/websocket"
)

// TopologyAgentEndpoint serves the graph for agents.
type TopologyAgentEndpoint struct {
	common.RWMutex
	ws.DefaultSpeakerEventHandler
	pool   ws.StructSpeakerPool
	Graph  *graph.Graph
	cached *graph.CachedBackend
}

// OnDisconnected called when an agent disconnected.
func (t *TopologyAgentEndpoint) OnDisconnected(c ws.Speaker) {
	host := c.GetRemoteHost()

	origin := string(c.GetServiceType())
	if len(host) > 0 {
		origin += "." + host
	}

	t.Graph.Lock()
	logging.GetLogger().Debugf("Authoritative client unregistered, delete resources %s", origin)
	t.Graph.DelOriginGraph(origin)
	t.Graph.Unlock()
}

// OnStructMessage is triggered when a message from the agent is received.
func (t *TopologyAgentEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	msgType, obj, err := graph.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	switch msgType {
	case graph.OriginGraphDeletedMsgType:
		// OriginGraphDeletedMsgType is handled specifically as we need to be sure to not use the
		// cache while deleting otherwise the delete mechanism is using the cache to walk through
		// the graph.
		logging.GetLogger().Debugf("Got %s message for host %s", graph.OriginGraphDeletedMsgType, obj.(string))
		t.Graph.DelOriginGraph(obj.(string))
	case graph.SyncMsgType, graph.SyncReplyMsgType:
		r := obj.(*graph.SyncMsg)
		for _, n := range r.Nodes {
			if t.Graph.GetNode(n.ID) == nil {
				t.Graph.NodeAdded(n)
			}
		}
		for _, e := range r.Edges {
			if t.Graph.GetEdge(e.ID) == nil {
				t.Graph.EdgeAdded(e)
			}
		}
	case graph.NodeUpdatedMsgType:
		t.Graph.NodeUpdated(obj.(*graph.Node))
	case graph.NodeDeletedMsgType:
		t.Graph.NodeDeleted(obj.(*graph.Node))
	case graph.NodeAddedMsgType:
		t.Graph.NodeAdded(obj.(*graph.Node))
	case graph.EdgeUpdatedMsgType:
		t.Graph.EdgeUpdated(obj.(*graph.Edge))
	case graph.EdgeDeletedMsgType:
		t.Graph.EdgeDeleted(obj.(*graph.Edge))
	case graph.EdgeAddedMsgType:
		t.Graph.EdgeAdded(obj.(*graph.Edge))
	}
}

// NewTopologyAgentEndpoint returns a new server that handles messages from the agents
func NewTopologyAgentEndpoint(pool ws.StructSpeakerPool, cached *graph.CachedBackend, g *graph.Graph) (*TopologyAgentEndpoint, error) {
	t := &TopologyAgentEndpoint{
		Graph:  g,
		pool:   pool,
		cached: cached,
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{graph.Namespace})

	return t, nil
}
