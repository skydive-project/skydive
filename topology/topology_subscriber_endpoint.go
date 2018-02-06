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

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

type topologySubscriber struct {
	graph         *graph.Graph
	gremlinFilter string
	ts            *traversal.GremlinTraversalSequence
}

// TopologySubscriberEndpoint sends all the modifications to its subscribers.
type TopologySubscriberEndpoint struct {
	sync.RWMutex
	shttp.DefaultWSSpeakerEventHandler
	pool          shttp.WSJSONSpeakerPool
	Graph         *graph.Graph
	wg            sync.WaitGroup
	gremlinParser *traversal.GremlinTraversalParser
	subscribers   map[string]*topologySubscriber
}

func (t *TopologySubscriberEndpoint) getGraph(gremlinQuery string, ts *traversal.GremlinTraversalSequence, lockGraph bool) (*graph.Graph, error) {
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

func (t *TopologySubscriberEndpoint) newTopologySubscriber(host string, gremlinFilter string, lockGraph bool) (*topologySubscriber, error) {
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
func (t *TopologySubscriberEndpoint) OnConnected(c shttp.WSSpeaker) {
	gremlinFilter := c.GetHeaders().Get("X-Gremlin-Filter")
	if gremlinFilter == "" {
		gremlinFilter = c.GetURL().Query().Get("x-gremlin-filter")
	}

	if gremlinFilter != "" {
		subscriber, err := t.newTopologySubscriber(c.GetHost(), gremlinFilter, false)
		if err != nil {
			logging.GetLogger().Error(err)
			return
		}

		logging.GetLogger().Infof("Client %s subscribed with filter %s", c.GetHost(), gremlinFilter)
		t.subscribers[c.GetHost()] = subscriber
	}
}

// OnDisconnected called when a subscriber got disconnected.
func (t *TopologySubscriberEndpoint) OnDisconnected(c shttp.WSSpeaker) {
	t.Lock()
	delete(t.subscribers, c.GetHost())
	t.Unlock()
}

// OnWSJSONMessage is triggered when receiving a message from a subscriber.
// It only responds to SyncRequestMsgType messages
func (t *TopologySubscriberEndpoint) OnWSJSONMessage(c shttp.WSSpeaker, msg *shttp.WSJSONMessage) {
	msgType, obj, err := graph.UnmarshalWSMessage(msg)

	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

	// this kind of message usually comes from external clients like the WebUI
	if msgType == graph.SyncRequestMsgType {
		t.Graph.RLock()
		defer t.Graph.RUnlock()

		syncMsg, status := obj.(graph.SyncRequestMsg), http.StatusOK
		g, err := t.Graph.WithContext(syncMsg.GraphContext)
		var result interface{} = g
		if err != nil {
			logging.GetLogger().Errorf("unable to get a graph with context %+v: %s", syncMsg, err.Error())
			result, status = nil, http.StatusBadRequest
		}

		if syncMsg.GremlinFilter != "" {
			subscriber, err := t.newTopologySubscriber(c.GetHost(), syncMsg.GremlinFilter, false)
			if err != nil {
				logging.GetLogger().Error(err)
				return
			}

			logging.GetLogger().Infof("Client %s subscribed with filter %s", c.GetHost(), syncMsg.GremlinFilter)
			result = subscriber.graph
			t.Lock()
			t.subscribers[c.GetHost()] = subscriber
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
func (t *TopologySubscriberEndpoint) notifyClients(msg *shttp.WSJSONMessage) {
	for _, c := range t.pool.GetSpeakers() {
		t.RLock()
		subscriber, found := t.subscribers[c.GetHost()]
		t.RUnlock()

		if found {
			g, err := t.getGraph(subscriber.gremlinFilter, subscriber.ts, false)
			if err != nil {
				logging.GetLogger().Error(err)
				continue
			}

			addedNodes, removedNodes, addedEdges, removedEdges := subscriber.graph.Diff(g)

			for _, n := range addedNodes {
				c.SendMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeAddedMsgType, n))
			}

			for _, n := range removedNodes {
				c.SendMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeDeletedMsgType, n))
			}

			for _, e := range addedEdges {
				c.SendMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeAddedMsgType, e))
			}

			for _, e := range removedEdges {
				c.SendMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeDeletedMsgType, e))
			}

			subscriber.graph = g
		} else {
			c.SendMessage(msg)
		}
	}
}

// OnNodeUpdated graph node updated event. Implements the GraphEventListener interface.
func (t *TopologySubscriberEndpoint) OnNodeUpdated(n *graph.Node) {
	t.notifyClients(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeUpdatedMsgType, n))
}

// OnNodeAdded graph node added event. Implements the GraphEventListener interface.
func (t *TopologySubscriberEndpoint) OnNodeAdded(n *graph.Node) {
	t.notifyClients(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeAddedMsgType, n))
}

// OnNodeDeleted graph node deleted event. Implements the GraphEventListener interface.
func (t *TopologySubscriberEndpoint) OnNodeDeleted(n *graph.Node) {
	t.notifyClients(shttp.NewWSJSONMessage(graph.Namespace, graph.NodeDeletedMsgType, n))
}

// OnEdgeUpdated graph edge updated event. Implements the GraphEventListener interface.
func (t *TopologySubscriberEndpoint) OnEdgeUpdated(e *graph.Edge) {
	t.notifyClients(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeUpdatedMsgType, e))
}

// OnEdgeAdded graph edge added event. Implements the GraphEventListener interface.
func (t *TopologySubscriberEndpoint) OnEdgeAdded(e *graph.Edge) {
	t.notifyClients(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeAddedMsgType, e))
}

// OnEdgeDeleted graph edge deleted event. Implements the GraphEventListener interface.
func (t *TopologySubscriberEndpoint) OnEdgeDeleted(e *graph.Edge) {
	t.notifyClients(shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeDeletedMsgType, e))
}

// NewTopologySubscriberEndpoint returns a new server to be used by external subscribers,
// for instance the WebUI.
func NewTopologySubscriberEndpoint(pool shttp.WSJSONSpeakerPool, auth *shttp.AuthenticationOpts, g *graph.Graph) *TopologySubscriberEndpoint {
	t := &TopologySubscriberEndpoint{
		Graph:         g,
		pool:          pool,
		subscribers:   make(map[string]*topologySubscriber),
		gremlinParser: traversal.NewGremlinTraversalParser(),
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddJSONMessageHandler(t, []string{graph.Namespace})

	// subscribe to the local graph event
	g.AddEventListener(t)
	return t
}
