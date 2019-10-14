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

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	gws "github.com/skydive-project/skydive/graffiti/websocket"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/websocket"
	ws "github.com/skydive-project/skydive/websocket"
)

type subscriber struct {
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
	subscribers   map[ws.Speaker]*subscriber
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
func (t *SubscriberEndpoint) OnConnected(c ws.Speaker) {
	gremlinFilter := c.GetHeaders().Get("X-Gremlin-Filter")
	if gremlinFilter == "" {
		gremlinFilter = c.GetURL().Query().Get("x-gremlin-filter")
	}

	if gremlinFilter != "" {
		host := c.GetRemoteHost()

		subscriber, err := t.newSubscriber(host, gremlinFilter, true)
		if err != nil {
			logging.GetLogger().Error(err)
			return
		}

		logging.GetLogger().Infof("Client %s subscribed with filter %s during the connection", host, gremlinFilter)
		t.Lock()
		t.subscribers[c] = subscriber
		t.Unlock()
	}
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
	msgType, obj, err := gws.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	// this kind of message usually comes from external clients like the WebUI
	if msgType == gws.SyncRequestMsgType {
		t.Graph.RLock()
		defer t.Graph.RUnlock()

		syncMsg, status := obj.(*gws.SyncRequestMsg), http.StatusOK
		result, err := t.Graph.CloneWithContext(syncMsg.Context)
		if err != nil {
			logging.GetLogger().Errorf("unable to get a graph with context %+v: %s", syncMsg, err)
			reply := msg.Reply(nil, gws.SyncReplyMsgType, http.StatusBadRequest)
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
					logging.GetLogger().Error(err)

					reply := msg.Reply(err.Error(), gws.SyncReplyMsgType, http.StatusBadRequest)
					c.SendMessage(reply)

					t.Lock()
					t.subscribers[c] = nil
					t.Unlock()

					return
				}

				logging.GetLogger().Infof("Client %s requested subscription with filter %s", host, *syncMsg.GremlinFilter)
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

		reply := msg.Reply(result, gws.SyncReplyMsgType, status)
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
				logging.GetLogger().Error(err)
				continue
			}

			addedNodes, removedNodes, addedEdges, removedEdges := subscriber.graph.Diff(g)

			for _, n := range addedNodes {
				c.SendMessage(gws.NewStructMessage(gws.NodeAddedMsgType, n))
			}

			for _, n := range removedNodes {
				c.SendMessage(gws.NewStructMessage(gws.NodeDeletedMsgType, n))
			}

			for _, e := range addedEdges {
				c.SendMessage(gws.NewStructMessage(gws.EdgeAddedMsgType, e))
			}

			for _, e := range removedEdges {
				c.SendMessage(gws.NewStructMessage(gws.EdgeDeletedMsgType, e))
			}

			// handle updates
			switch typ {
			case gws.NodeUpdatedMsgType:
				if g.GetNode(i.(*graph.Node).ID) != nil {
					c.SendMessage(gws.NewStructMessage(gws.NodeUpdatedMsgType, i))
				}
			case gws.EdgeUpdatedMsgType:
				if g.GetEdge(i.(*graph.Edge).ID) != nil {
					c.SendMessage(gws.NewStructMessage(gws.EdgeUpdatedMsgType, i))
				}
			}

			subscriber.graph = g
		} else {
			c.SendMessage(gws.NewStructMessage(typ, i))
		}
	}
}

// OnNodeUpdated graph node updated event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnNodeUpdated(n *graph.Node) {
	t.notifyClients(gws.NodeUpdatedMsgType, n)
}

// OnNodeAdded graph node added event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnNodeAdded(n *graph.Node) {
	t.notifyClients(gws.NodeAddedMsgType, n)
}

// OnNodeDeleted graph node deleted event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnNodeDeleted(n *graph.Node) {
	t.notifyClients(gws.NodeDeletedMsgType, n)
}

// OnEdgeUpdated graph edge updated event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnEdgeUpdated(e *graph.Edge) {
	t.notifyClients(gws.EdgeUpdatedMsgType, e)
}

// OnEdgeAdded graph edge added event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnEdgeAdded(e *graph.Edge) {
	t.notifyClients(gws.EdgeAddedMsgType, e)
}

// OnEdgeDeleted graph edge deleted event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnEdgeDeleted(e *graph.Edge) {
	t.notifyClients(gws.EdgeDeletedMsgType, e)
}

// NewSubscriberEndpoint returns a new server to be used by external subscribers,
// for instance the WebUI.
func NewSubscriberEndpoint(pool ws.StructSpeakerPool, g *graph.Graph, tr *traversal.GremlinTraversalParser) *SubscriberEndpoint {
	t := &SubscriberEndpoint{
		Graph:         g,
		pool:          pool,
		subscribers:   make(map[ws.Speaker]*subscriber),
		gremlinParser: tr,
	}

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{gws.Namespace})

	// subscribe to the local graph event
	g.AddEventListener(t)
	return t
}

// ClientOrigin return a string identifying a client using its service type and host id
func ClientOrigin(c websocket.Speaker) string {
	origin := string(c.GetServiceType())
	if len(c.GetRemoteHost()) > 0 {
		origin += "." + c.GetRemoteHost()
	}

	return origin
}

// DelSubGraphOfOrigin deletes all the nodes with a specified origin
func DelSubGraphOfOrigin(g *graph.Graph, origin string) {
	g.DelNodes(graph.Metadata{"Origin": origin})
}
