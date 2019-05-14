/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	gws "github.com/skydive-project/skydive/graffiti/websocket"
	"github.com/skydive-project/skydive/logging"
	ws "github.com/skydive-project/skydive/websocket"
)

// AgentEndpoint serves the graph for agents.
type AgentEndpoint struct {
	common.RWMutex
	ws.DefaultSpeakerEventHandler
	pool    ws.StructSpeakerPool
	Graph   *graph.Graph
	cached  *graph.CachedBackend
	authors map[string]bool
}

// OnDisconnected called when an agent disconnected.
func (t *AgentEndpoint) OnDisconnected(c ws.Speaker) {
	origin := clientOrigin(c)

	t.RLock()
	_, ok := t.authors[origin]
	t.RUnlock()

	// not an author so do not delete resources
	if !ok {
		return
	}

	logging.GetLogger().Debugf("Authoritative client unregistered, delete resources of %s", origin)

	t.Graph.Lock()
	delSubGraphOfOrigin(t.cached, t.Graph, origin)
	t.Graph.Unlock()

	t.Lock()
	delete(t.authors, origin)
	t.Unlock()
}

// OnStructMessage is triggered when a message from the agent is received.
func (t *AgentEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	origin := clientOrigin(c)

	t.Lock()
	// received a message thus the pod has chosen this hub as master
	if _, ok := t.authors[origin]; !ok {
		t.authors[origin] = true
		logging.GetLogger().Debugf("Authoritative client registered %s", origin)
	}
	t.Unlock()

	msgType, obj, err := gws.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event : %s", err)
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	switch msgType {
	case gws.SyncMsgType, gws.SyncReplyMsgType:
		r := obj.(*gws.SyncMsg)

		delSubGraphOfOrigin(t.cached, t.Graph, origin)

		for _, n := range r.Nodes {
			if t.Graph.GetNode(n.ID) == nil {
				if err := t.Graph.NodeAdded(n); err != nil {
					logging.GetLogger().Errorf("%s, %+v", err, n)
				}
			}
		}
		for _, e := range r.Edges {
			if t.Graph.GetEdge(e.ID) == nil {
				if err := t.Graph.EdgeAdded(e); err != nil {
					logging.GetLogger().Errorf("%s, %+v", err, e)
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
		logging.GetLogger().Errorf("%s, %+v", err, msg)
	}
}

// NewPodEndpoint returns a new server that handles messages from the agents
func NewPodEndpoint(pool ws.StructSpeakerPool, cached *graph.CachedBackend, g *graph.Graph) (*AgentEndpoint, error) {
	t := &AgentEndpoint{
		Graph:   g,
		pool:    pool,
		cached:  cached,
		authors: make(map[string]bool),
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{gws.Namespace})

	return t, nil
}
