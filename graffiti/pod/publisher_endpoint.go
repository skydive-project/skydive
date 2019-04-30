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

package pod

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

// PublisherEndpoint serves the graph for external publishers, for instance
// an external program that interacts with the Skydive graph.
type PublisherEndpoint struct {
	common.RWMutex
	ws.DefaultSpeakerEventHandler
	pool            ws.StructSpeakerPool
	Graph           *graph.Graph
	schemaValidator *topology.SchemaValidator
	gremlinParser   *traversal.GremlinTraversalParser
}

// OnStructMessage is triggered by message coming from a publisher.
func (t *PublisherEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	msgType, obj, err := gws.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	/*switch msgType {
	case gws.NodeAddedMsgType, gws.NodeUpdatedMsgType, gws.NodeDeletedMsgType:
		err = t.schemaValidator.ValidateNode(obj.(*graph.Node))
	case gws.EdgeAddedMsgType, gws.EdgeUpdatedMsgType, gws.EdgeDeletedMsgType:
		err = t.schemaValidator.ValidateEdge(obj.(*graph.Edge))
	}*/

	/*if err != nil {
		logging.GetLogger().Error(err)
		return
	}*/

	t.Graph.Lock()
	defer t.Graph.Unlock()

	switch msgType {
	case gws.SyncRequestMsgType:
		reply := msg.Reply(t.Graph, gws.SyncReplyMsgType, http.StatusOK)
		c.SendMessage(reply)
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

// NewPublisherEndpoint returns a new server for external publishers.
func NewPublisherEndpoint(pool ws.StructSpeakerPool, g *graph.Graph) (*PublisherEndpoint, error) {
	schemaValidator, err := topology.NewSchemaValidator()
	if err != nil {
		return nil, err
	}

	t := &PublisherEndpoint{
		Graph:           g,
		pool:            pool,
		schemaValidator: schemaValidator,
		gremlinParser:   traversal.NewGremlinTraversalParser(),
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{gws.Namespace})

	return t, nil
}
