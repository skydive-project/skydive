/*
 * Copyright (C) 2017 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package hub

import (
	"net/http"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	gws "github.com/skydive-project/skydive/graffiti/websocket"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
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
	cached          *graph.CachedBackend
}

// OnDisconnected called when a publisher got disconnected.
func (t *TopologyPublisherEndpoint) OnDisconnected(c ws.Speaker) {
	policy := PersistencePolicy(c.GetHeaders().Get("X-Persistence-Policy"))
	if policy == Persistent {
		return
	}

	origin := clientOrigin(c)

	logging.GetLogger().Debugf("Authoritative client unregistered, delete resources of %s", origin)

	t.Graph.Lock()
	delSubGraphOfOrigin(t.cached, t.Graph, origin)
	t.Graph.Unlock()
}

// OnStructMessage is triggered by message coming from a publisher.
func (t *TopologyPublisherEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	msgType, obj, err := gws.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	origin := clientOrigin(c)

	switch msgType {
	case gws.NodeAddedMsgType, gws.NodeUpdatedMsgType, gws.NodeDeletedMsgType:
		obj.(*graph.Node).Origin = origin
		err = t.schemaValidator.ValidateNode(obj.(*graph.Node))
	case gws.EdgeAddedMsgType, gws.EdgeUpdatedMsgType, gws.EdgeDeletedMsgType:
		obj.(*graph.Edge).Origin = origin
		err = t.schemaValidator.ValidateEdge(obj.(*graph.Edge))
	}

	if err != nil {
		logging.GetLogger().Error(err)
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	switch msgType {
	case gws.SyncRequestMsgType:
		reply := msg.Reply(t.Graph, gws.SyncReplyMsgType, http.StatusOK)
		c.SendMessage(reply)
	case gws.SyncMsgType, gws.SyncReplyMsgType:
		r := obj.(*gws.SyncMsg)

		delSubGraphOfOrigin(t.cached, t.Graph, clientOrigin(c))

		for _, n := range r.Nodes {
			if t.Graph.GetNode(n.ID) == nil {
				if err := t.Graph.NodeAdded(n); err != nil {
					logging.GetLogger().Error(err)
				}
			}
		}
		for _, e := range r.Edges {
			if t.Graph.GetEdge(e.ID) == nil {
				if err := t.Graph.EdgeAdded(e); err != nil {
					logging.GetLogger().Error(err)
				}
			}
		}
	case gws.NodeUpdatedMsgType:
		err = t.Graph.NodeUpdated(obj.(*graph.Node))
	case gws.NodeDeletedMsgType:
		err = t.Graph.NodeDeleted(obj.(*graph.Node))
	case gws.NodeAddedMsgType:
		err = t.Graph.NodeAdded(obj.(*graph.Node))
	case gws.EdgeUpdatedMsgType:
		err = t.Graph.EdgeUpdated(obj.(*graph.Edge))
	case gws.EdgeDeletedMsgType:
		if err = t.Graph.EdgeDeleted(obj.(*graph.Edge)); err == graph.ErrEdgeNotFound {
			return
		}
	case gws.EdgeAddedMsgType:
		err = t.Graph.EdgeAdded(obj.(*graph.Edge))
	}

	if err != nil {
		logging.GetLogger().Error(err)
	}
}

// NewTopologyPublisherEndpoint returns a new server for external publishers.
func NewTopologyPublisherEndpoint(pool ws.StructSpeakerPool, cached *graph.CachedBackend, g *graph.Graph) (*TopologyPublisherEndpoint, error) {
	schemaValidator, err := topology.NewSchemaValidator()
	if err != nil {
		return nil, err
	}

	t := &TopologyPublisherEndpoint{
		Graph:           g,
		pool:            pool,
		schemaValidator: schemaValidator,
		gremlinParser:   traversal.NewGremlinTraversalParser(),
		cached:          cached,
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{gws.Namespace})

	return t, nil
}
