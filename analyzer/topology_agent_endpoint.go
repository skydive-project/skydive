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
	"sync"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// TopologyAgentEndpoint serves the graph for agents.
type TopologyAgentEndpoint struct {
	common.RWMutex
	shttp.DefaultWSSpeakerEventHandler
	pool   shttp.WSStructSpeakerPool
	Graph  *graph.Graph
	cached *graph.CachedBackend
	wg     sync.WaitGroup
}

// OnDisconnected called when an agent disconnected.
func (t *TopologyAgentEndpoint) OnDisconnected(c shttp.WSSpeaker) {
	host := c.GetHost()
	t.Graph.Lock()
	logging.GetLogger().Debugf("Authoritative client unregistered, delete resources %s", host)
	t.Graph.DelHostGraph(host)
	t.Graph.Unlock()
}

// OnWSStructMessage is triggered when a message from the agent is received.
func (t *TopologyAgentEndpoint) OnWSStructMessage(c shttp.WSSpeaker, msg *shttp.WSStructMessage) {
	msgType, obj, err := graph.UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	switch msgType {
	case graph.HostGraphDeletedMsgType:
		// HostGraphDeletedMsgType is handled specifically as we need to be sure to not use the
		// cache while deleting otherwise the delete mechanism is using the cache to walk through
		// the graph.
		logging.GetLogger().Debugf("Got %s message for host %s", graph.HostGraphDeletedMsgType, obj.(string))
		t.Graph.DelHostGraph(obj.(string))
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
func NewTopologyAgentEndpoint(pool shttp.WSStructSpeakerPool, auth *shttp.AuthenticationOpts, cached *graph.CachedBackend, g *graph.Graph) (*TopologyAgentEndpoint, error) {
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
