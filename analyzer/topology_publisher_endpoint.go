/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"net/http"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	ws "github.com/skydive-project/skydive/websocket"
)

// PersistencePolicy defines Persistent policy for publishers
type PersistencePolicy string

const (
	// Persistent means that the graph elements created will always remain
	Persistent PersistencePolicy = "Persistent"
	// DeleteOnDisconnect means the graph elements created will be deleted on client disconnect
	DeleteOnDisconnect PersistencePolicy = "DeleteOnDisconnect"
)

// TopologyPublisherEndpoint serves the graph for external publishers, for instance
// an external program that interacts with the Skydive graph.
type TopologyPublisherEndpoint struct {
	common.RWMutex
	ws.DefaultSpeakerEventHandler
	pool            ws.StructSpeakerPool
	Graph           *graph.Graph
	schemaValidator *topology.SchemaValidator
	gremlinParser   *traversal.GremlinTraversalParser
}

// OnDisconnected called when a publisher got disconnected.
func (t *TopologyPublisherEndpoint) OnDisconnected(c ws.Speaker) {
	policy := PersistencePolicy(c.GetHeaders().Get("X-Persistence-Policy"))
	if policy == Persistent {
		return
	}

	origin := string(c.GetServiceType())
	if len(c.GetRemoteHost()) > 0 {
		origin += "." + c.GetRemoteHost()
	}
	logging.GetLogger().Debugf("Authoritative client unregistered, delete resources %s", origin)

	t.Graph.Lock()
	t.Graph.DelOriginGraph(origin)
	t.Graph.Unlock()
}

// OnStructMessage is triggered by message coming from a publisher.
func (t *TopologyPublisherEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	msgType, obj, err := graph.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	origin := string(c.GetServiceType())
	if len(c.GetRemoteHost()) > 0 {
		origin += "." + c.GetRemoteHost()
	}

	switch msgType {
	case graph.NodeAddedMsgType, graph.NodeUpdatedMsgType, graph.NodeDeletedMsgType:
		obj.(*graph.Node).SetOrigin(origin)
		err = t.schemaValidator.ValidateNode(obj.(*graph.Node))
	case graph.EdgeAddedMsgType, graph.EdgeUpdatedMsgType, graph.EdgeDeletedMsgType:
		obj.(*graph.Edge).SetOrigin(origin)
		err = t.schemaValidator.ValidateEdge(obj.(*graph.Edge))
	}

	if err != nil {
		logging.GetLogger().Error(err)
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	switch msgType {
	case graph.SyncRequestMsgType:
		reply := msg.Reply(t.Graph, graph.SyncReplyMsgType, http.StatusOK)
		c.SendMessage(reply)
	case graph.OriginGraphDeletedMsgType:
		// OriginGraphDeletedMsgType is handled specifically as we need to be sure to not use the
		// cache while deleting otherwise the delete mechanism is using the cache to walk through
		// the graph.
		logging.GetLogger().Debugf("Got %s message for origin %s", graph.OriginGraphDeletedMsgType, obj.(string))
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

// NewTopologyPublisherEndpoint returns a new server for external publishers.
func NewTopologyPublisherEndpoint(pool ws.StructSpeakerPool, g *graph.Graph) (*TopologyPublisherEndpoint, error) {
	schemaValidator, err := topology.NewSchemaValidator()
	if err != nil {
		return nil, err
	}

	t := &TopologyPublisherEndpoint{
		Graph:           g,
		pool:            pool,
		schemaValidator: schemaValidator,
		gremlinParser:   traversal.NewGremlinTraversalParser(),
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{graph.Namespace})

	return t, nil
}
