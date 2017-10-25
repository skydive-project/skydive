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

// TopologyServer serves Graph events through websocket connections. It forwards
// all the Graph events and handles specific websocket API calls. All the API calls
// handled by the server are of type graph.*MsgType.
type TopologyServer struct {
	Graph *graph.Graph
	Pool  shttp.WSJSONSpeakerPool
}

// OnWSJSONMessage is called by the pool of Websocket JSON speakers. it Implements the
// shttp.WSJSONMessageHandler interface.
func (t *TopologyServer) OnWSJSONMessage(c shttp.WSSpeaker, msg *shttp.WSJSONMessage) {
	msgType, obj, err := graph.UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

	// NOTE: currently the agent side server only support SyncRequestMsgType API call.
	// It is then impossible to modify the graph from the websocket API calls.
	if msgType == graph.SyncRequestMsgType {
		t.Graph.RLock()
		g, status := t.Graph, http.StatusOK
		if obj.(graph.SyncRequestMsg).TimeSlice != nil {
			logging.GetLogger().Error("agents don't support sync request with context")
			g, status = nil, http.StatusBadRequest
		}
		reply := msg.Reply(g, graph.SyncReplyMsgType, status)
		c.SendMessage(reply)
		t.Graph.RUnlock()
	}
}

// OnNodeUpdated graph node updated event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnNodeUpdated(n *graph.Node) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeUpdatedMsgType, n))
}

// OnNodeAdded graph node added event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnNodeAdded(n *graph.Node) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeAddedMsgType, n))
}

// OnNodeDeleted graph node deleted event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnNodeDeleted(n *graph.Node) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeDeletedMsgType, n))
}

// OnEdgeUpdated graph edge updated event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnEdgeUpdated(e *graph.Edge) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeUpdatedMsgType, e))
}

// OnEdgeAdded graph edge added event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnEdgeAdded(e *graph.Edge) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeAddedMsgType, e))
}

// OnEdgeDeleted graph edge deleted event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnEdgeDeleted(e *graph.Edge) {
	t.Pool.QueueBroadcastMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeDeletedMsgType, e))
}

// NewTopologyServer returns a new graph server for the given graph and WSJSONSpeakerPool.
func NewTopologyServer(g *graph.Graph, pool shttp.WSJSONSpeakerPool) *TopologyServer {
	t := &TopologyServer{
		Graph: g,
		Pool:  pool,
	}

	// listen to graph events
	t.Graph.AddEventListener(t)

	// will get OnWSJSONMessage events
	pool.AddJSONMessageHandler(t, []string{graph.Namespace})
	return t
}
