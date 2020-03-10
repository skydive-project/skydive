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

package common

import (
	"net/http"

	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/messages"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// PersistencePolicy defines Persistent policy for publishers
type PersistencePolicy string

const (
	// Persistent means that the graph elements created will always remain
	Persistent PersistencePolicy = "Persistent"
	// DeleteOnDisconnect means the graph elements created will be deleted on client disconnect
	DeleteOnDisconnect PersistencePolicy = "DeleteOnDisconnect"
)

// PublisherEndpoint serves the graph for external publishers, for instance
// an external program that interacts with the Skydive graph.
type PublisherEndpoint struct {
	insanelock.RWMutex
	ws.DefaultSpeakerEventHandler
	pool      ws.StructSpeakerPool
	Graph     *graph.Graph
	validator server.Validator
	authors   map[string]bool
}

// OnDisconnected called when a publisher got disconnected.
func (t *PublisherEndpoint) OnDisconnected(c ws.Speaker) {
	origin := ClientOrigin(c)

	t.RLock()
	_, ok := t.authors[origin]
	t.RUnlock()

	// not an author so do not delete resources
	if !ok {
		return
	}

	policy := PersistencePolicy(c.GetHeaders().Get("X-Persistence-Policy"))
	if policy != Persistent {
		logging.GetLogger().Debugf("Authoritative client unregistered, delete resources of %s", origin)

		t.Graph.Lock()
		DelSubGraphOfOrigin(t.Graph, origin)
		t.Graph.Unlock()
	}

	t.Lock()
	delete(t.authors, origin)
	t.Unlock()
}

// OnStructMessage is triggered by message coming from a publisher.
func (t *PublisherEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	msgType, obj, err := messages.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	origin := ClientOrigin(c)

	t.Lock()
	// received a message thus the pod has chosen this hub as master
	if _, ok := t.authors[origin]; !ok {
		t.authors[origin] = true
		logging.GetLogger().Debugf("Authoritative client registered %s", origin)
	}
	t.Unlock()

	if t.validator != nil {
		switch msgType {
		case messages.NodeAddedMsgType, messages.NodeUpdatedMsgType, messages.NodeDeletedMsgType:
			obj.(*graph.Node).Origin = origin
			err = t.validator.Validate("node", obj.(*graph.Node))
		case messages.EdgeAddedMsgType, messages.EdgeUpdatedMsgType, messages.EdgeDeletedMsgType:
			obj.(*graph.Edge).Origin = origin
			err = t.validator.Validate("edge", obj.(*graph.Edge))
		}

		if err != nil {
			logging.GetLogger().Error(err)
			return
		}
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	switch msgType {
	case messages.SyncRequestMsgType:
		reply := msg.Reply(t.Graph, messages.SyncReplyMsgType, http.StatusOK)
		c.SendMessage(reply)
	case messages.SyncMsgType, messages.SyncReplyMsgType:
		logging.GetLogger().Debugf("Handling sync message from %s", c.GetRemoteHost())

		DelSubGraphOfOrigin(t.Graph, ClientOrigin(c))

		r := obj.(*messages.SyncMsg)
		for _, n := range r.Nodes {
			if t.Graph.GetNode(n.ID) == nil {
				if err := t.Graph.NodeAdded(n); err != nil {
					logging.GetLogger().Errorf("Error while processing sync message from %s: %s", c.GetRemoteHost(), err)
				}
			}
		}
		for _, e := range r.Edges {
			if t.Graph.GetEdge(e.ID) == nil {
				if err := t.Graph.EdgeAdded(e); err != nil {
					logging.GetLogger().Errorf("Error while processing sync message from %s: %s", c.GetRemoteHost(), err)
				}
			}
		}
	case messages.NodeUpdatedMsgType:
		err = t.Graph.NodeUpdated(obj.(*graph.Node))
	case messages.NodeDeletedMsgType:
		err = t.Graph.NodeDeleted(obj.(*graph.Node))
	case messages.NodeAddedMsgType:
		err = t.Graph.NodeAdded(obj.(*graph.Node))
	case messages.EdgeUpdatedMsgType:
		err = t.Graph.EdgeUpdated(obj.(*graph.Edge))
	case messages.EdgeDeletedMsgType:
		if err = t.Graph.EdgeDeleted(obj.(*graph.Edge)); err == graph.ErrEdgeNotFound {
			return
		}
	case messages.EdgeAddedMsgType:
		err = t.Graph.EdgeAdded(obj.(*graph.Edge))
	}

	if err != nil {
		logging.GetLogger().Errorf("Error while processing message type %s from %s: %s", msgType, c.GetRemoteHost(), err)
	}
}

// NewPublisherEndpoint returns a new server for external publishers.
func NewPublisherEndpoint(pool ws.StructSpeakerPool, g *graph.Graph, validator server.Validator) (*PublisherEndpoint, error) {
	t := &PublisherEndpoint{
		Graph:     g,
		pool:      pool,
		validator: validator,
		authors:   make(map[string]bool),
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{messages.Namespace})

	return t, nil
}
