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
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/messages"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

type subscriber struct {
	graph         *graph.Graph
	gremlinFilter string
	ts            *traversal.GremlinTraversalSequence
}

// SubscriberEndpoint sends all the modifications to its subscribers.
type SubscriberEndpoint struct {
	insanelock.RWMutex
	ws.DefaultSpeakerEventHandler
	pool          ws.StructSpeakerPool
	Graph         *graph.Graph
	wg            sync.WaitGroup
	gremlinParser *traversal.GremlinTraversalParser
	subscribers   map[ws.Speaker]*subscriber
	logger        logging.Logger
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

func (t *SubscriberEndpoint) newSubscriber(host string, gremlinFilter string, lockGraph bool) (*subscriber, error) {
	ts, err := t.gremlinParser.Parse(strings.NewReader(gremlinFilter))
	if err != nil {
		return nil, fmt.Errorf("Invalid Gremlin filter '%s' for client %s", gremlinFilter, host)
	}

	g, err := t.getGraph(gremlinFilter, ts, lockGraph)
	if err != nil {
		return nil, err
	}

	return &subscriber{graph: g, ts: ts, gremlinFilter: gremlinFilter}, nil
}

// OnConnected called when a subscriber got connected.
func (t *SubscriberEndpoint) OnConnected(c ws.Speaker) error {
	gremlinFilter := c.GetHeaders().Get("X-Gremlin-Filter")
	if gremlinFilter == "" {
		gremlinFilter = c.GetURL().Query().Get("x-gremlin-filter")
	}

	if gremlinFilter != "" {
		host := c.GetRemoteHost()

		subscriber, err := t.newSubscriber(host, gremlinFilter, true)
		if err != nil {
			t.logger.Error(err)
			return err
		}

		t.logger.Infof("Client %s subscribed with filter %s during the connection", host, gremlinFilter)
		t.subscribers[c] = subscriber
	}

	return nil
}

// OnDisconnected called when a subscriber got disconnected.
func (t *SubscriberEndpoint) OnDisconnected(c ws.Speaker) {
	t.Lock()
	delete(t.subscribers, c)
	t.Unlock()
}

// OnStructMessage is triggered when receiving a message from a subscriber.
// It only responds to SyncRequestMsgType messages
func (t *SubscriberEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	msgType, obj, err := messages.UnmarshalMessage(msg)
	if err != nil {
		t.logger.Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	// this kind of message usually comes from external clients like the WebUI
	if msgType == messages.SyncRequestMsgType {
		t.Graph.RLock()
		defer t.Graph.RUnlock()

		syncMsg, status := obj.(*messages.SyncRequestMsg), http.StatusOK
		result, err := t.Graph.CloneWithContext(syncMsg.Context)
		if err != nil {
			t.logger.Errorf("unable to get a graph with context %+v: %s", syncMsg, err)
			reply := msg.Reply(nil, messages.SyncReplyMsgType, http.StatusBadRequest)
			c.SendMessage(reply)
			return
		}

		host := c.GetRemoteHost()

		if syncMsg.GremlinFilter != nil {
			// filter reset
			if *syncMsg.GremlinFilter == "" {
				t.Lock()
				delete(t.subscribers, c)
				t.Unlock()
			} else {
				subscriber, err := t.newSubscriber(host, *syncMsg.GremlinFilter, false)
				if err != nil {
					t.logger.Error(err)

					reply := msg.Reply(err.Error(), messages.SyncReplyMsgType, http.StatusBadRequest)
					c.SendMessage(reply)

					t.Lock()
					t.subscribers[c] = nil
					t.Unlock()

					return
				}

				t.logger.Infof("Client %s requested subscription with filter %s", host, *syncMsg.GremlinFilter)
				result = subscriber.graph

				t.Lock()
				t.subscribers[c] = subscriber
				t.Unlock()
			}
		} else {
			t.RLock()
			subscriber := t.subscribers[c]
			t.RUnlock()

			if subscriber != nil {
				result = subscriber.graph
			}
		}

		reply := msg.Reply(result, messages.SyncReplyMsgType, status)
		c.SendMessage(reply)

		return
	}
}

// notifyClients forwards local graph modification to subscribers. If a subscriber
// specified a Gremlin filter, a 'Diff' is applied between the previous graph state
// for this subscriber and the current graph state.
func (t *SubscriberEndpoint) notifyClients(typ string, i interface{}) {
	for _, c := range t.pool.GetSpeakers() {
		t.RLock()
		subscriber, found := t.subscribers[c]
		t.RUnlock()

		if found {
			// in the case of an error during the subscription we got a nil subscriber
			if subscriber == nil {
				return
			}

			g, err := t.getGraph(subscriber.gremlinFilter, subscriber.ts, false)
			if err != nil {
				t.logger.Error(err)
				continue
			}

			addedNodes, removedNodes, addedEdges, removedEdges := subscriber.graph.Diff(g)

			for _, n := range addedNodes {
				c.SendMessage(messages.NewStructMessage(messages.NodeAddedMsgType, n))
			}

			for _, n := range removedNodes {
				c.SendMessage(messages.NewStructMessage(messages.NodeDeletedMsgType, n))
			}

			for _, e := range addedEdges {
				c.SendMessage(messages.NewStructMessage(messages.EdgeAddedMsgType, e))
			}

			for _, e := range removedEdges {
				c.SendMessage(messages.NewStructMessage(messages.EdgeDeletedMsgType, e))
			}

			// handle updates
			switch typ {
			case messages.NodeUpdatedMsgType:
				if g.GetNode(i.(*graph.Node).ID) != nil {
					c.SendMessage(messages.NewStructMessage(messages.NodeUpdatedMsgType, i))
				}
			case messages.EdgeUpdatedMsgType:
				if g.GetEdge(i.(*graph.Edge).ID) != nil {
					c.SendMessage(messages.NewStructMessage(messages.EdgeUpdatedMsgType, i))
				}
			}

			subscriber.graph = g
		} else {
			c.SendMessage(messages.NewStructMessage(typ, i))
		}
	}
}

// OnNodeUpdated graph node updated event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnNodeUpdated(n *graph.Node) {
	t.notifyClients(messages.NodeUpdatedMsgType, n)
}

// OnNodeAdded graph node added event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnNodeAdded(n *graph.Node) {
	t.notifyClients(messages.NodeAddedMsgType, n)
}

// OnNodeDeleted graph node deleted event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnNodeDeleted(n *graph.Node) {
	t.notifyClients(messages.NodeDeletedMsgType, n)
}

// OnEdgeUpdated graph edge updated event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnEdgeUpdated(e *graph.Edge) {
	t.notifyClients(messages.EdgeUpdatedMsgType, e)
}

// OnEdgeAdded graph edge added event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnEdgeAdded(e *graph.Edge) {
	t.notifyClients(messages.EdgeAddedMsgType, e)
}

// OnEdgeDeleted graph edge deleted event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnEdgeDeleted(e *graph.Edge) {
	t.notifyClients(messages.EdgeDeletedMsgType, e)
}

// NewSubscriberEndpoint returns a new server to be used by external subscribers,
// for instance the WebUI.
func NewSubscriberEndpoint(pool ws.StructSpeakerPool, g *graph.Graph, tr *traversal.GremlinTraversalParser, logger logging.Logger) *SubscriberEndpoint {
	if logger == nil {
		logger = logging.GetLogger()
	}

	t := &SubscriberEndpoint{
		Graph:         g,
		pool:          pool,
		subscribers:   make(map[ws.Speaker]*subscriber),
		gremlinParser: tr,
		logger:        logger,
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{messages.Namespace})

	// subscribe to the local graph event
	g.AddEventListener(t)
	return t
}
