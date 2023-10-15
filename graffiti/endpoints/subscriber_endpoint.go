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

package endpoints

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/messages"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// UpdatePolicy specifies whether we should send partial or full updates
type UpdatePolicy = string

// Update policies
const (
	FullUpdates    UpdatePolicy = "full"
	PartialUpdates              = "partial"
)

type subscriber struct {
	ws.Speaker
	lastGraph     *graph.Graph
	gremlinFilter string
	ts            *traversal.GremlinTraversalSequence
	updatePolicy  UpdatePolicy
	inhibit       atomic.Value
}

func (s *subscriber) getSubGraph(g *graph.Graph, lockGraph bool) (*graph.Graph, error) {
	res, err := s.ts.Exec(g, lockGraph)
	if err != nil {
		return nil, err
	}

	tv, ok := res.(*traversal.GraphTraversal)
	if !ok {
		return nil, fmt.Errorf("Gremlin query '%s' did not return a graph", s.gremlinFilter)
	}

	return tv.Graph, nil
}

func (s *subscriber) resetFilter() {
	s.gremlinFilter = ""
	s.ts = nil
	s.lastGraph = nil
}

// SubscriberEndpoint sends all the modifications to its subscribers.
type SubscriberEndpoint struct {
	insanelock.RWMutex
	ws.DefaultSpeakerEventHandler
	pool          ws.StructSpeakerPool
	graph         *graph.Graph
	wg            sync.WaitGroup
	gremlinParser *traversal.GremlinTraversalParser
	subscribers   map[ws.Speaker]*subscriber
	logger        logging.Logger
	inhib         atomic.Value
}

func (t *SubscriberEndpoint) newSubscriber(speaker ws.Speaker, gremlinFilter string, lockGraph bool) (s *subscriber, err error) {
	s = &subscriber{Speaker: speaker, gremlinFilter: gremlinFilter}

	if gremlinFilter != "" {
		s.ts, err = t.gremlinParser.Parse(strings.NewReader(gremlinFilter))
		if err != nil {
			return nil, fmt.Errorf("Invalid Gremlin filter '%s' for client %s", gremlinFilter, speaker.GetRemoteHost())
		}

		if s.lastGraph, err = s.getSubGraph(t.graph, lockGraph); err != nil {
			return nil, err
		}
	} else {
		s.lastGraph = t.graph
	}

	return s, nil
}

// OnConnected called when a subscriber got connected.
func (t *SubscriberEndpoint) OnConnected(c ws.Speaker) error {
	gremlinFilter := c.GetHeaders().Get("X-Gremlin-Filter")
	if gremlinFilter == "" {
		gremlinFilter = c.GetURL().Query().Get("x-gremlin-filter")
	}

	subscriber, err := t.newSubscriber(c, gremlinFilter, true)
	if err != nil {
		t.logger.Error(err)
		return err
	}

	updatePolicy := c.GetHeaders().Get("X-Update-Policy")
	if updatePolicy == "" {
		updatePolicy = c.GetURL().Query().Get("x-update-policy")
	}

	switch strings.ToLower(updatePolicy) {
	case FullUpdates, PartialUpdates:
	case "":
		updatePolicy = FullUpdates
	default:
		err = fmt.Errorf("invalid update policy '%s'", subscriber.updatePolicy)
		t.logger.Error(err)
		return err
	}

	subscriber.updatePolicy = updatePolicy

	t.logger.Infof("Client %s subscribed with filter '%s' and update policy '%s'", c.GetRemoteHost(), gremlinFilter, updatePolicy)
	t.Lock()
	t.subscribers[c] = subscriber
	t.Unlock()

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
		t.graph.RLock()
		defer t.graph.RUnlock()

		syncMsg, status := obj.(*messages.SyncRequestMsg), http.StatusOK
		result, err := t.graph.CloneWithContext(syncMsg.Context)
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
				t.subscribers[c].resetFilter()
				t.Unlock()
			} else {
				subscriber, err := t.newSubscriber(c, *syncMsg.GremlinFilter, false)
				if err != nil {
					t.logger.Error(err)

					reply := msg.Reply(err.Error(), messages.SyncReplyMsgType, http.StatusBadRequest)
					c.SendMessage(reply)

					return
				}

				t.logger.Infof("Client %s requested subscription with filter %s", host, *syncMsg.GremlinFilter)
				result = subscriber.lastGraph

				t.Lock()
				t.subscribers[c] = subscriber
				t.Unlock()
			}
		} else {
			t.RLock()
			subscriber := t.subscribers[c]
			t.RUnlock()

			if subscriber != nil {
				result = subscriber.lastGraph
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
func (t *SubscriberEndpoint) notifyClients(typ string, i interface{}, ops []graph.PartiallyUpdatedOp) {
	var subscribers []*subscriber
	t.RLock()
	for _, subscriber := range t.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	t.RUnlock()

SUBSCRIBER:
	for _, subscriber := range subscribers {
		msg := i
		msgType := typ

		if inhibitedSpeaker := t.inhib.Load(); inhibitedSpeaker.(string) != "" {
			if inhibitedSpeaker.(string) == subscriber.Speaker.GetRemoteHost() {
				continue SUBSCRIBER
			}
		}

		if subscriber.updatePolicy == PartialUpdates {
			switch typ {
			case messages.NodeUpdatedMsgType:
				msgType = messages.NodePartiallyUpdatedMsgType
				n := msg.(*graph.Node)
				msg = messages.PartiallyUpdatedMsg{
					ID:        n.ID,
					Revision:  n.Revision,
					UpdatedAt: n.UpdatedAt,
					Ops:       ops,
				}
			case messages.EdgeUpdatedMsgType:
				msgType = messages.EdgePartiallyUpdatedMsgType
				e := msg.(*graph.Edge)
				msg = messages.PartiallyUpdatedMsg{
					ID:        e.ID,
					Revision:  e.Revision,
					UpdatedAt: e.UpdatedAt,
					Ops:       ops,
				}
			}
		}

		if subscriber.gremlinFilter != "" {
			g, err := subscriber.getSubGraph(t.graph, false)
			if err != nil {
				t.logger.Error(err)
				continue
			}

			addedNodes, removedNodes, addedEdges, removedEdges := subscriber.lastGraph.Diff(g)

			for _, n := range addedNodes {
				subscriber.SendMessage(messages.NewStructMessage(messages.NodeAddedMsgType, n))
			}

			for _, n := range removedNodes {
				subscriber.SendMessage(messages.NewStructMessage(messages.NodeDeletedMsgType, n))
			}

			for _, e := range addedEdges {
				subscriber.SendMessage(messages.NewStructMessage(messages.EdgeAddedMsgType, e))
			}

			for _, e := range removedEdges {
				subscriber.SendMessage(messages.NewStructMessage(messages.EdgeDeletedMsgType, e))
			}

			// handle updates
			switch typ {
			case messages.NodeUpdatedMsgType, messages.NodePartiallyUpdatedMsgType:
				if g.GetNode(i.(*graph.Node).ID) != nil {
					subscriber.SendMessage(messages.NewStructMessage(msgType, msg))
				}
			case messages.EdgeUpdatedMsgType, messages.EdgePartiallyUpdatedMsgType:
				if g.GetEdge(i.(*graph.Edge).ID) != nil {
					subscriber.SendMessage(messages.NewStructMessage(msgType, msg))
				}
			}

			subscriber.lastGraph = g
		} else {
			subscriber.SendMessage(messages.NewStructMessage(msgType, msg))
		}
	}
}

// OnNodeUpdated graph node updated event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnNodeUpdated(n *graph.Node, ops []graph.PartiallyUpdatedOp) {
	t.notifyClients(messages.NodeUpdatedMsgType, n, ops)
}

// OnNodeAdded graph node added event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnNodeAdded(n *graph.Node) {
	t.notifyClients(messages.NodeAddedMsgType, n, nil)
}

// OnNodeDeleted graph node deleted event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnNodeDeleted(n *graph.Node) {
	t.notifyClients(messages.NodeDeletedMsgType, n, nil)
}

// OnEdgeUpdated graph edge updated event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnEdgeUpdated(e *graph.Edge, ops []graph.PartiallyUpdatedOp) {
	t.notifyClients(messages.EdgeUpdatedMsgType, e, nil)
}

// OnEdgeAdded graph edge added event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnEdgeAdded(e *graph.Edge) {
	t.notifyClients(messages.EdgeAddedMsgType, e, nil)
}

// OnEdgeDeleted graph edge deleted event. Implements the GraphEventListener interface.
func (t *SubscriberEndpoint) OnEdgeDeleted(e *graph.Edge) {
	t.notifyClients(messages.EdgeDeletedMsgType, e, nil)
}

// Inhib node and edge forwarding
func (t *SubscriberEndpoint) Inhib(c ws.Speaker) {
	remoteHost := ""
	if c != nil {
		remoteHost = c.GetRemoteHost()
	}
	t.inhib.Store(remoteHost)
}

// NewSubscriberEndpoint returns a new server to be used by external subscribers,
// for instance the WebUI.
func NewSubscriberEndpoint(pool ws.StructSpeakerPool, g *graph.Graph, tr *traversal.GremlinTraversalParser, logger logging.Logger) *SubscriberEndpoint {
	if logger == nil {
		logger = logging.GetLogger()
	}

	t := &SubscriberEndpoint{
		graph:         g,
		pool:          pool,
		subscribers:   make(map[ws.Speaker]*subscriber),
		gremlinParser: tr,
		logger:        logger,
	}

	t.inhib.Store("")

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddStructMessageHandler(t, []string{messages.Namespace})

	// subscribe to the local graph event
	g.AddEventListener(t)
	return t
}
