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
	"sync"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/statics"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	"github.com/xeipuuv/gojsonschema"
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
	shttp.DefaultWSSpeakerEventHandler
	pool          shttp.WSStructSpeakerPool
	Graph         *graph.Graph
	cached        *graph.CachedBackend
	nodeSchema    gojsonschema.JSONLoader
	edgeSchema    gojsonschema.JSONLoader
	wg            sync.WaitGroup
	gremlinParser *traversal.GremlinTraversalParser
}

// OnDisconnected called when a publisher got disconnected.
func (t *TopologyPublisherEndpoint) OnDisconnected(c shttp.WSSpeaker) {
	policy := PersistencePolicy(c.GetHeaders().Get("X-Persistence-Policy"))
	if policy == Persistent {
		return
	}
	logging.GetLogger().Debugf("Authoritative client unregistered, delete resources %s", c.GetHost())

	t.Graph.Lock()
	t.Graph.DelHostGraph(c.GetHost())
	t.Graph.Unlock()
}

// OnWSStructMessage is triggered by message coming from a publisher.
func (t *TopologyPublisherEndpoint) OnWSStructMessage(c shttp.WSSpeaker, msg *shttp.WSStructMessage) {
	msgType, obj, err := graph.UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

	// We use JSON schema to validate the message
	loader := gojsonschema.NewGoLoader(obj)

	var schema gojsonschema.JSONLoader
	switch msgType {
	case graph.NodeAddedMsgType, graph.NodeUpdatedMsgType, graph.NodeDeletedMsgType:
		schema = t.nodeSchema
	case graph.EdgeAddedMsgType, graph.EdgeUpdatedMsgType, graph.EdgeDeletedMsgType:
		schema = t.edgeSchema
	}

	if schema != nil {
		if _, err := gojsonschema.Validate(t.edgeSchema, loader); err != nil {
			logging.GetLogger().Errorf("Invalid message: %s", err.Error())
			return
		}
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	switch msgType {
	case graph.SyncRequestMsgType:
		reply := msg.Reply(t.Graph, graph.SyncReplyMsgType, http.StatusOK)
		c.SendMessage(reply)
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

// NewTopologyPublisherEndpoint returns a new server for external publishers.
func NewTopologyPublisherEndpoint(pool shttp.WSStructSpeakerPool, auth *shttp.AuthenticationOpts, g *graph.Graph) (*TopologyPublisherEndpoint, error) {
	nodeSchema, err := statics.Asset("statics/schemas/node.schema")
	if err != nil {
		return nil, err
	}

	edgeSchema, err := statics.Asset("statics/schemas/edge.schema")
	if err != nil {
		return nil, err
	}

	t := &TopologyPublisherEndpoint{
		Graph:         g,
		pool:          pool,
		nodeSchema:    gojsonschema.NewBytesLoader(nodeSchema),
		edgeSchema:    gojsonschema.NewBytesLoader(edgeSchema),
		gremlinParser: traversal.NewGremlinTraversalParser(),
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{graph.Namespace})

	return t, nil
}
