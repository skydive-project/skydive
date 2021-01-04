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
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/messages"
	"github.com/skydive-project/skydive/graffiti/service"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

var (
	errAlreadyConnected  = errors.New("already connected")
	errConnectedToItself = errors.New("connected to itself")
)

// ReplicatorPeer is a remote connection to another Graph server. Only modification
// of the local Graph made either by the local server, by an agent message or by an external
// client will be forwarded to the peer.
type ReplicatorPeer struct {
	ws.DefaultSpeakerEventHandler
	URL       *url.URL
	Graph     *graph.Graph
	opts      *ws.ClientOpts
	wsspeaker ws.Speaker
	endpoint  *ReplicationEndpoint
}

type peerState struct {
	cnt int
}

// ReplicationEndpoint serves the local Graph and send local modification to its peers.
type ReplicationEndpoint struct {
	insanelock.RWMutex
	ws.DefaultSpeakerEventHandler
	in           ws.StructSpeakerPool
	out          *ws.StructClientPool
	peerStates   map[string]*peerState
	candidates   []*ReplicatorPeer
	Graph        *graph.Graph
	cached       *graph.CachedBackend
	replicateMsg atomic.Value
	wg           sync.WaitGroup
	logger       logging.Logger
}

// OnConnected is called when the peer gets connected then the whole graph
// is send to initialize it.
func (p *ReplicatorPeer) OnConnected(c ws.Speaker) error {
	p.endpoint.Lock()
	defer p.endpoint.Unlock()

	host := c.GetRemoteHost()
	if c.GetHost() == host {
		p.endpoint.logger.Debugf("Disconnecting from %s since it's me", p.URL.String())
		return errConnectedToItself
	}

	p.Graph.RLock()
	defer p.Graph.RUnlock()

	state, ok := p.endpoint.peerStates[host]
	if !ok {
		state = &peerState{}
	}

	// disconnect as can be connected to the same host from different addresses.
	if state.cnt > 0 {
		p.endpoint.logger.Debugf("Disconnecting from %s as already connected through %s", p.URL.String(), c.GetURL().String())
		return errAlreadyConnected
	}

	p.endpoint.peerStates[host] = state
	state.cnt++

	msg := &messages.SyncMsg{
		Elements: p.Graph.Elements(),
	}

	if err := p.wsspeaker.SendMessage(messages.NewStructMessage(messages.SyncMsgType, msg)); err != nil {
		return err
	}

	return p.endpoint.out.AddClient(c)
}

// OnDisconnected is called when the peer gets disconnected
func (p *ReplicatorPeer) OnDisconnected(c ws.Speaker) {
	p.endpoint.Lock()
	defer p.endpoint.Unlock()

	host := c.GetRemoteHost()
	if host == c.GetHost() {
		return
	}

	state := p.endpoint.peerStates[host]
	state.cnt--
	if state.cnt > 0 {
		return
	}

	origin := graph.ClientOrigin(c)
	if p.Graph.GetOrigin() == origin {
		return
	}

	p.endpoint.out.RemoveClient(c)
	delete(p.endpoint.peerStates, host)

	p.Graph.Lock()
	graph.DelSubGraphOfOrigin(p.Graph, origin)
	p.Graph.Unlock()
}

func (p *ReplicatorPeer) connect(wg *sync.WaitGroup) {
	defer wg.Done()

	p.endpoint.logger.Infof("Connecting to peer: %s", p.URL.String())
	serviceType := service.Type(strings.SplitN(p.Graph.GetOrigin(), ".", 2)[0])
	wsClient := ws.NewClient(p.Graph.GetHost(), serviceType, p.URL, *p.opts)

	structClient := wsClient.UpgradeToStructSpeaker()
	// will trigger shttp.SpeakerEventHandler, so OnConnected
	structClient.AddEventHandler(p)

	// subscribe to the graph messages
	structClient.AddStructMessageHandler(p.endpoint, []string{messages.Namespace})

	p.wsspeaker = structClient
	p.wsspeaker.Start()
}

func (p *ReplicatorPeer) disconnect() {
	if p.wsspeaker != nil {
		p.wsspeaker.Stop()
	}
}

func (t *ReplicationEndpoint) addCandidate(addr string, port int, opts *ws.ClientOpts) *ReplicatorPeer {
	url, _ := http.MakeURL("ws", addr, port, "/ws/replication", opts.TLSConfig != nil)
	peer := &ReplicatorPeer{
		URL:      url,
		Graph:    t.Graph,
		opts:     opts,
		endpoint: t,
	}

	t.candidates = append(t.candidates, peer)
	return peer
}

// ConnectPeers starts a goroutine connecting all the peers.
func (t *ReplicationEndpoint) ConnectPeers() {
	t.RLock()
	defer t.RUnlock()

	for _, candidate := range t.candidates {
		t.wg.Add(1)
		go candidate.connect(&t.wg)
	}
}

// DisconnectPeers disconnects all the peers and wait until all disconnected.
func (t *ReplicationEndpoint) DisconnectPeers() {
	t.RLock()
	defer t.RUnlock()

	for _, candidate := range t.candidates {
		candidate.disconnect()
	}
	t.wg.Wait()
}

// OnStructMessage is triggered by message coming from an other peer.
func (t *ReplicationEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	if c.GetRemoteHost() == t.Graph.GetHost() {
		t.logger.Debugf("Ignore message from myself(%s), %s", c.GetURL().String())
		return
	}

	msgType, obj, err := messages.UnmarshalMessage(msg)
	if err != nil {
		t.logger.Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	t.replicateMsg.Store(false)
	defer t.replicateMsg.Store(true)

	// replicated graph, do not used persistent backend, another hub will handle this
	t.cached.SetMode(graph.CacheOnlyMode)
	defer t.cached.SetMode(graph.DefaultMode)

	switch msgType {
	case messages.SyncMsgType:
		r := obj.(*messages.SyncMsg)

		for _, n := range r.Nodes {
			if t.Graph.GetNode(n.ID) == nil {
				if err := t.Graph.NodeAdded(n); err != nil {
					t.logger.Errorf("%s, %+v", err, n)
				}
			}
		}
		for _, e := range r.Edges {
			if t.Graph.GetEdge(e.ID) == nil {
				if err := t.Graph.EdgeAdded(e); err != nil {
					t.logger.Errorf("%s, %+v", err, e)
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
	case messages.NodePartiallyUpdatedMsgType:
		updateMsg := obj.(*messages.PartiallyUpdatedMsg)
		err = t.Graph.NodePartiallyUpdated(updateMsg.ID, updateMsg.Revision, updateMsg.UpdatedAt, updateMsg.Ops...)
	case messages.EdgePartiallyUpdatedMsgType:
		updateMsg := obj.(*messages.PartiallyUpdatedMsg)
		err = t.Graph.EdgePartiallyUpdated(updateMsg.ID, updateMsg.Revision, updateMsg.UpdatedAt, updateMsg.Ops...)
	}

	if err != nil {
		t.logger.Errorf("Error while processing message type %s from %s: %s", msgType, c.GetRemoteHost(), err)
	}
}

// SendToPeers sends the message to all the peers
func (t *ReplicationEndpoint) notifyPeers(msg *ws.StructMessage) {
	t.in.BroadcastMessage(msg)
	t.out.BroadcastMessage(msg)
}

// OnNodeUpdated graph node updated event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnNodeUpdated(n *graph.Node, ops []graph.PartiallyUpdatedOp) {
	if t.replicateMsg.Load() == true {
		msg := messages.NewStructMessage(
			messages.NodePartiallyUpdatedMsgType,
			messages.PartiallyUpdatedMsg{
				ID:        n.ID,
				Revision:  n.Revision,
				UpdatedAt: n.UpdatedAt,
				Ops:       ops,
			},
		)
		t.notifyPeers(msg)
	}
}

// OnNodeAdded graph node added event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnNodeAdded(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := messages.NewStructMessage(messages.NodeAddedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnNodeDeleted graph node deleted event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnNodeDeleted(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := messages.NewStructMessage(messages.NodeDeletedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnEdgeUpdated graph edge updated event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnEdgeUpdated(e *graph.Edge, ops []graph.PartiallyUpdatedOp) {
	if t.replicateMsg.Load() == true {
		msg := messages.NewStructMessage(
			messages.EdgePartiallyUpdatedMsgType,
			messages.PartiallyUpdatedMsg{
				ID:        e.ID,
				Revision:  e.Revision,
				UpdatedAt: e.UpdatedAt,
				Ops:       ops,
			},
		)
		t.notifyPeers(msg)
	}
}

// OnEdgeAdded graph edge added event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnEdgeAdded(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := messages.NewStructMessage(messages.EdgeAddedMsgType, e)
		t.notifyPeers(msg)
	}
}

// OnEdgeDeleted graph edge deleted event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnEdgeDeleted(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := messages.NewStructMessage(messages.EdgeDeletedMsgType, e)
		t.notifyPeers(msg)
	}
}

// GetIncomingSpeakers return incoming speakers
func (t *ReplicationEndpoint) GetIncomingSpeakers() []ws.Speaker {
	return t.in.GetSpeakers()
}

// GetOutgoingSpeakers return outgoing speakers
func (t *ReplicationEndpoint) GetOutgoingSpeakers() []ws.Speaker {
	return t.out.GetSpeakers()
}

// GetSpeakers return both incoming and outgoing speakers
func (t *ReplicationEndpoint) GetSpeakers() []ws.Speaker {
	return append(t.in.GetSpeakers(), t.out.GetSpeakers()...)
}

// OnConnected is called when an incoming peer got connected.
func (t *ReplicationEndpoint) OnConnected(c ws.Speaker) error {
	t.Lock()
	defer t.Unlock()

	host := c.GetRemoteHost()
	if host == c.GetHost() {
		t.logger.Debugf("Disconnect %s since it's me", host)
		return errConnectedToItself
	}

	state, ok := t.peerStates[host]
	if !ok {
		state = &peerState{}
	}

	if state.cnt > 0 {
		t.logger.Debugf("Disconnecting %s from %s as already connected", host, c.GetURL())
		return errAlreadyConnected
	}

	t.peerStates[host] = state
	state.cnt++

	// subscribe to websocket structured messages
	c.(*ws.StructSpeaker).AddStructMessageHandler(t, []string{messages.Namespace})

	t.Graph.RLock()
	defer t.Graph.RUnlock()

	msg := &messages.SyncMsg{
		Elements: t.Graph.Elements(),
	}

	return c.SendMessage(messages.NewStructMessage(messages.SyncMsgType, msg))
}

// OnDisconnected is called when an incoming peer got disconnected.
func (t *ReplicationEndpoint) OnDisconnected(c ws.Speaker) {
	t.Lock()
	defer t.Unlock()

	host := c.GetRemoteHost()
	if host == c.GetHost() {
		return
	}

	state := t.peerStates[host]
	state.cnt--
	if state.cnt > 0 {
		return
	}

	origin := graph.ClientOrigin(c)
	if t.Graph.GetOrigin() == origin {
		return
	}

	delete(t.peerStates, host)
}

// NewReplicationEndpoint returns a new server to be used by other analyzers for replication.
func NewReplicationEndpoint(pool ws.StructSpeakerPool, opts *ws.ClientOpts, cached *graph.CachedBackend, g *graph.Graph, peers []service.Address, logger logging.Logger) *ReplicationEndpoint {
	if logger == nil {
		logger = logging.GetLogger()
	}

	t := &ReplicationEndpoint{
		Graph:      g,
		cached:     cached,
		in:         pool,
		out:        ws.NewStructClientPool("ReplicationEndpoint", ws.PoolOpts{}),
		peerStates: make(map[string]*peerState),
		logger:     logger,
	}
	t.replicateMsg.Store(true)

	for _, sa := range peers {
		t.addCandidate(sa.Addr, sa.Port, opts)
	}

	pool.AddEventHandler(t)

	// subscribe to the local graph event
	g.AddEventListener(t)

	return t
}
