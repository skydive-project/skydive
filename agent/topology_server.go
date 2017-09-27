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

package agent

import (
	"net/http"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// TopologyServer describes a graph server based on websocket
type TopologyServer struct {
	Graph *graph.Graph
	Pool  shttp.WSJSONSpeakerPool
}

// OnWSMessage event
func (t *TopologyServer) OnWSJSONMessage(c shttp.WSSpeaker, msg shttp.WSJSONMessage) {
	msgType, obj, err := graph.UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

	if msgType == graph.SyncRequestMsgType {
		t.Graph.RLock()
		g, status := t.Graph, http.StatusOK
		if obj.(graph.GraphContext).TimeSlice != nil {
			logging.GetLogger().Error("agents don't support sync request with context")
			g, status = nil, http.StatusBadRequest
		}
		reply := msg.Reply(g, graph.SyncReplyMsgType, status)
		c.Send(reply)
		t.Graph.RUnlock()
	}

	// NOTE(safchain) currently agents don't support add/del operations
}

// OnNodeUpdated event
func (t *TopologyServer) OnNodeUpdated(n *graph.Node) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeUpdatedMsgType, n))
}

// OnNodeAdded event
func (t *TopologyServer) OnNodeAdded(n *graph.Node) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeAddedMsgType, n))
}

// OnNodeDeleted event
func (t *TopologyServer) OnNodeDeleted(n *graph.Node) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeDeletedMsgType, n))
}

// OnEdgeUpdated event
func (t *TopologyServer) OnEdgeUpdated(e *graph.Edge) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeUpdatedMsgType, e))
}

// OnEdgeAdded event
func (t *TopologyServer) OnEdgeAdded(e *graph.Edge) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeAddedMsgType, e))
}

// OnEdgeDeleted event
func (t *TopologyServer) OnEdgeDeleted(e *graph.Edge) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeDeletedMsgType, e))
}

// NewTopologyServer creates a new topology graph server based on a websocket server
func NewTopologyServer(g *graph.Graph, pool shttp.WSJSONSpeakerPool) *TopologyServer {
	t := &TopologyServer{
		Graph: g,
		Pool:  pool,
	}
	t.Graph.AddEventListener(t)
	pool.AddJSONMessageHandler(t, []string{graph.Namespace})
	return t
}
