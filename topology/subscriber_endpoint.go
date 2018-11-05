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

package topology

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	ws "github.com/skydive-project/skydive/websocket"
)

type topologySubscriber struct {
	graph         *graph.Graph
	gremlinFilter string
	ts            *traversal.GremlinTraversalSequence
}

// SubscriberEndpoint sends all the modifications to its subscribers.
type SubscriberEndpoint struct {
	common.RWMutex
	ws.DefaultSpeakerEventHandler
	pool          ws.StructSpeakerPool
	Graph         *graph.Graph
	wg            sync.WaitGroup
	gremlinParser *traversal.GremlinTraversalParser
	subscribers   map[string]*topologySubscriber
}

func (t *SubscriberEndpoint) getGraph(gremlinQuery string, ts *traversal.GremlinTraversalSequence, lockGraph bool) (*graph.Graph, error) {
	res, err := ts.Exec(t.Graph, lockGraph)
	if err != nil {
		return nil, err
	}

	tv, ok := res.(*traversal.GraphTraversal)
	if !ok {
		return nil, fmt.Errorf("Gremlin query '%s' did not return a graph", gremlinQuery)
	}

	return tv.Graph, nil
}

func (t *SubscriberEndpoint) newTopologySubscriber(host string, gremlinFilter string, lockGraph bool) (*topologySubscriber, error) {
	ts, err := t.gremlinParser.Parse(strings.NewReader(gremlinFilter))
	if err != nil {
		return nil, fmt.Errorf("Invalid Gremlin filter '%s' for client %s", gremlinFilter, host)
	}

	g, err := t.getGraph(gremlinFilter, ts, lockGraph)
	if err != nil {
		return nil, err
	}

	return &topologySubscriber{graph: g, ts: ts, gremlinFilter: gremlinFilter}, nil
}

// OnConnected called when a subscriber got connected.
func (t *SubscriberEndpoint) OnConnected(c ws.Speaker) {
	gremlinFilter := c.GetHeaders().Get("X-Gremlin-Filter")
	if gremlinFilter == "" {
		gremlinFilter = c.GetURL().Query().Get("x-gremlin-filter")
	}

	if gremlinFilter != "" {
		host := c.GetRemoteHost()

		subscriber, err := t.newTopologySubscriber(host, gremlinFilter, false)
		if err != nil {
			logging.GetLogger().Error(err)
			return
		}

		logging.GetLogger().Infof("Client %s subscribed with filter %s", host, gremlinFilter)
		t.subscribers[host] = subscriber
	}
}

// OnDisconnected called when a subscriber got disconnected.
func (t *SubscriberEndpoint) OnDisconnected(c ws.Speaker) {
	t.Lock()
	delete(t.subscribers, c.GetRemoteHost())
	t.Unlock()
}

// OnStructMessage is triggered when receiving a message from a subscriber.
// It only responds to SyncRequestMsgType messages
func (t *SubscriberEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	msgType, obj, err := graph.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	// this kind of message usually comes from external clients like the WebUI
	if msgType == graph.SyncRequestMsgType {
		t.Graph.RLock()
		defer t.Graph.RUnlock()

		syncMsg, status := obj.(graph.SyncRequestMsg), http.StatusOK
		g, err := t.Graph.CloneWithContext(syncMsg.Context)
		var result interface{} = g
		if err != nil {
			logging.GetLogger().Errorf("unable to get a graph with context %+v: %s", syncMsg, err)
			result, status = nil, http.StatusBadRequest
		}

		if syncMsg.GremlinFilter != "" {
			host := c.GetRemoteHost()

			subscriber, err := t.newTopologySubscriber(host, syncMsg.GremlinFilter, false)
			if err != nil {
				logging.GetLogger().Error(err)
				return
			}

			logging.GetLogger().Infof("Client %s subscribed with filter %s", host, syncMsg.GremlinFilter)
			result = subscriber.graph
			t.Lock()
			t.subscribers[host] = subscriber
			t.Unlock()
		}

		reply := msg.Reply(result, graph.SyncReplyMsgType, status)
		c.SendMessage(reply)

		return
	}
}

// notifyClients forwards local graph modification to subscribers. If a subscriber
// specified a Gremlin filter, a 'Diff' is applied between the previous graph state
// for this subscriber and the current graph state.
func (t *SubscriberEndpoint) notifyClients(msg *ws.StructMessage) {
	for _, c := range t.pool.GetSpeakers() {
		t.RLock()
		subscriber, found := t.subscribers[c.GetRemoteHost()]
		t.RUnlock()

		if found {
			g, err := t.getGraph(subscriber.gremlinFilter, subscriber.ts, false)
			if err != nil {
				logging.GetLogger().Error(err)
				continue
			}

			addedNodes, removedNodes, addedEdges, removedEdges := subscriber.graph.Diff(g)

			for _, n := range addedNodes {
				c.SendMessage(ws.NewStructMessage(graph.Namespace, graph.NodeAddedMsgType, n))
			}

			for _, n := range removedNodes {
				c.SendMessage(ws.NewStructMessage(graph.Namespace, graph.NodeDeletedMsgType, n))
			}

			for _, e := range addedEdges {
				c.SendMessage(ws.NewStructMessage(graph.Namespace, graph.EdgeAddedMsgType, e))
			}

			for _, e := range removedEdges {
				c.SendMessage(ws.NewStructMessage(graph.Namespace, graph.EdgeDeletedMsgType, e))
			}

			subscriber.graph = g
		} else {
			c.SendMessage(msg)
		}
	}
}

// OnNodeUpdated graph node updated event. Implements the EventListener interface.
func (t *SubscriberEndpoint) OnNodeUpdated(n *graph.Node) {
	t.notifyClients(ws.NewStructMessage(graph.Namespace, graph.NodeUpdatedMsgType, n))
}

// OnNodeAdded graph node added event. Implements the EventListener interface.
func (t *SubscriberEndpoint) OnNodeAdded(n *graph.Node) {
	t.notifyClients(ws.NewStructMessage(graph.Namespace, graph.NodeAddedMsgType, n))
}

// OnNodeDeleted graph node deleted event. Implements the EventListener interface.
func (t *SubscriberEndpoint) OnNodeDeleted(n *graph.Node) {
	t.notifyClients(ws.NewStructMessage(graph.Namespace, graph.NodeDeletedMsgType, n))
}

// OnEdgeUpdated graph edge updated event. Implements the EventListener interface.
func (t *SubscriberEndpoint) OnEdgeUpdated(e *graph.Edge) {
	t.notifyClients(ws.NewStructMessage(graph.Namespace, graph.EdgeUpdatedMsgType, e))
}

// OnEdgeAdded graph edge added event. Implements the EventListener interface.
func (t *SubscriberEndpoint) OnEdgeAdded(e *graph.Edge) {
	t.notifyClients(ws.NewStructMessage(graph.Namespace, graph.EdgeAddedMsgType, e))
}

// OnEdgeDeleted graph edge deleted event. Implements the EventListener interface.
func (t *SubscriberEndpoint) OnEdgeDeleted(e *graph.Edge) {
	t.notifyClients(ws.NewStructMessage(graph.Namespace, graph.EdgeDeletedMsgType, e))
}

// NewSubscriberEndpoint returns a new server to be used by external subscribers,
// for instance the WebUI.
func NewSubscriberEndpoint(pool ws.StructSpeakerPool, g *graph.Graph, tr *traversal.GremlinTraversalParser) *SubscriberEndpoint {
	t := &SubscriberEndpoint{
		Graph:         g,
		pool:          pool,
		subscribers:   make(map[string]*topologySubscriber),
		gremlinParser: tr,
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{graph.Namespace})

	// subscribe to the local graph event
	g.AddEventListener(t)
	return t
}
