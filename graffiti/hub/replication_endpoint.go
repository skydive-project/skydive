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
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	gws "github.com/skydive-project/skydive/graffiti/websocket"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/websocket"
	ws "github.com/skydive-project/skydive/websocket"
)

// ReplicatorPeer is a remote connection to another Graph server. Only modification
// of the local Graph made either by the local server, by an agent message or by an external
// client will be forwarded to the peer.
type ReplicatorPeer struct {
	ws.DefaultSpeakerEventHandler
	URL       *url.URL
	Graph     *graph.Graph
	AuthOpts  *shttp.AuthenticationOpts
	wsspeaker ws.Speaker
	endpoint  *ReplicationEndpoint
}

type peerState struct {
	cnt int
}

// ReplicationEndpoint serves the local Graph and send local modification to its peers.
type ReplicationEndpoint struct {
	common.RWMutex
	ws.DefaultSpeakerEventHandler
	in           ws.StructSpeakerPool
	out          *ws.StructClientPool
	peerStates   map[string]*peerState
	candidates   []*ReplicatorPeer
	Graph        *graph.Graph
	cached       *graph.CachedBackend
	replicateMsg atomic.Value
	wg           sync.WaitGroup
}

func (t *ReplicationEndpoint) debug() bool {
	return config.GetBool("analyzer.replication.debug")
}

// OnConnected is called when the peer gets connected then the whole graph
// is send to initialize it.
func (p *ReplicatorPeer) OnConnected(c ws.Speaker) {
	p.endpoint.Lock()
	defer p.endpoint.Unlock()

	p.Graph.RLock()
	defer p.Graph.RUnlock()

	host := c.GetRemoteHost()

	state, ok := p.endpoint.peerStates[host]
	if !ok {
		state = &peerState{}
		p.endpoint.peerStates[host] = state
	}
	state.cnt++

	if host == config.GetString("host_id") {
		logging.GetLogger().Debugf("Disconnecting from %s since it's me", p.URL.String())
		c.Stop()
		return
	}

	// disconnect as can be connected to the same host from different addresses.
	if state.cnt > 1 {
		logging.GetLogger().Debugf("Disconnecting from %s as already connected through %s", p.URL.String(), c.GetURL().String())
		c.Stop()
		return
	}

	msg := &gws.SyncMsg{
		Elements: p.Graph.Elements(),
	}

	p.wsspeaker.SendMessage(gws.NewStructMessage(gws.SyncMsgType, msg))

	p.endpoint.out.AddClient(c)
}

// OnDisconnected is called when the peer gets disconnected
func (p *ReplicatorPeer) OnDisconnected(c ws.Speaker) {
	p.endpoint.Lock()
	defer p.endpoint.Unlock()

	host := c.GetRemoteHost()

	state := p.endpoint.peerStates[host]
	state.cnt--
	if state.cnt > 0 {
		return
	}

	origin := clientOrigin(c)
	if p.Graph.Origin() == origin {
		return
	}

	logging.GetLogger().Debugf("Peer unregistered, delete resources of %s", origin)

	p.Graph.Lock()
	delSubGraphOfOrigin(p.endpoint.cached, p.Graph, origin)
	p.Graph.Unlock()

	p.endpoint.out.RemoveClient(c)
	delete(p.endpoint.peerStates, host)
}

func (p *ReplicatorPeer) connect(wg *sync.WaitGroup) {
	defer wg.Done()

	logging.GetLogger().Infof("Connecting to peer: %s", p.URL.String())
	wsClient, err := config.NewWSClient(common.AnalyzerService, p.URL, websocket.ClientOpts{AuthOpts: p.AuthOpts})
	if err != nil {
		logging.GetLogger().Errorf("Failed to create client: %s", err)
		return
	}

	structClient := wsClient.UpgradeToStructSpeaker()
	// will trigger shttp.SpeakerEventHandler, so OnConnected
	structClient.AddEventHandler(p)

	// subscribe to the graph messages
	structClient.AddStructMessageHandler(p.endpoint, []string{gws.Namespace})

	p.wsspeaker = structClient
	p.wsspeaker.Start()
}

func (p *ReplicatorPeer) disconnect() {
	if p.wsspeaker != nil {
		p.wsspeaker.Stop()
	}
}

func (t *ReplicationEndpoint) addCandidate(url *url.URL, auth *shttp.AuthenticationOpts) *ReplicatorPeer {
	peer := &ReplicatorPeer{
		URL:      url,
		Graph:    t.Graph,
		AuthOpts: auth,
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
		if t.debug() {
			logging.GetLogger().Debugf("Connecting to peer %s", candidate.URL.String())
		}
		go candidate.connect(&t.wg)
	}
}

// DisconnectPeers disconnects all the peers and wait until all disconnected.
func (t *ReplicationEndpoint) DisconnectPeers() {
	t.RLock()
	defer t.RUnlock()

	for _, candidate := range t.candidates {
		if t.debug() {
			logging.GetLogger().Debugf("Disconnecting from peer %s", candidate.URL.String())
		}
		candidate.disconnect()
	}
	t.wg.Wait()
}

// OnStructMessage is triggered by message coming from an other peer.
func (t *ReplicationEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	if c.GetRemoteHost() == config.GetString("host_id") {
		logging.GetLogger().Debugf("Ignore message from myself(%s), %s", c.GetURL().String())
		return
	}

	msgType, obj, err := gws.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	t.replicateMsg.Store(false)
	defer t.replicateMsg.Store(true)

	// replicated graph, do not used persistent backend, another hub will handle this
	t.cached.SetMode(graph.CacheOnlyMode)
	defer t.cached.SetMode(graph.DefaultMode)

	if t.debug() {
		b, _ := msg.Bytes(ws.JSONProtocol)
		logging.GetLogger().Debugf("Received message from peer %s: %s", c.GetURL().String(), string(b))
	}

	switch msgType {
	case gws.SyncMsgType:
		r := obj.(*gws.SyncMsg)

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
		logging.GetLogger().Errorf("Error while processing message type %s: %s", msgType, err)
	}
}

// SendToPeers sends the message to all the peers
func (t *ReplicationEndpoint) notifyPeers(msg *ws.StructMessage) {
	if t.debug() {
		b, _ := msg.Bytes(ws.JSONProtocol)
		logging.GetLogger().Debugf("Broadcasting message to all peers: %s", string(b))
	}

	t.in.BroadcastMessage(msg)
	t.out.BroadcastMessage(msg)
}

// OnNodeUpdated graph node updated event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnNodeUpdated(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := gws.NewStructMessage(gws.NodeUpdatedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnNodeAdded graph node added event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnNodeAdded(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := gws.NewStructMessage(gws.NodeAddedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnNodeDeleted graph node deleted event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnNodeDeleted(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := gws.NewStructMessage(gws.NodeDeletedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnEdgeUpdated graph edge updated event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnEdgeUpdated(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := gws.NewStructMessage(gws.EdgeUpdatedMsgType, e)
		t.notifyPeers(msg)
	}
}

// OnEdgeAdded graph edge added event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnEdgeAdded(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := gws.NewStructMessage(gws.EdgeAddedMsgType, e)
		t.notifyPeers(msg)
	}
}

// OnEdgeDeleted graph edge deleted event. Implements the EventListener interface.
func (t *ReplicationEndpoint) OnEdgeDeleted(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := gws.NewStructMessage(gws.EdgeDeletedMsgType, e)
		t.notifyPeers(msg)
	}
}

// GetSpeakers return both incoming and outgoing speakers
func (t *ReplicationEndpoint) GetSpeakers() []ws.Speaker {
	return append(t.in.GetSpeakers(), t.out.GetSpeakers()...)
}

// OnConnected is called when an incoming peer got connected.
func (t *ReplicationEndpoint) OnConnected(c ws.Speaker) {
	t.Lock()
	defer t.Unlock()

	host := c.GetRemoteHost()

	state, ok := t.peerStates[host]
	if !ok {
		state = &peerState{}
		t.peerStates[host] = state
	}
	state.cnt++

	if state.cnt > 1 {
		logging.GetLogger().Debugf("Disconnecting %s from %s as already connected", host, c.GetURL())

		c.Stop()
		return
	}

	// subscribe to websocket structured messages
	c.(*ws.StructSpeaker).AddStructMessageHandler(t, []string{gws.Namespace})

	t.Graph.RLock()
	defer t.Graph.RUnlock()

	msg := &gws.SyncMsg{
		Elements: t.Graph.Elements(),
	}

	c.SendMessage(gws.NewStructMessage(gws.SyncMsgType, msg))
}

// OnDisconnected is called when an incoming peer got disconnected.
func (t *ReplicationEndpoint) OnDisconnected(c ws.Speaker) {
	t.Lock()
	defer t.Unlock()

	host := c.GetRemoteHost()

	state := t.peerStates[host]
	state.cnt--
	if state.cnt > 0 {
		return
	}

	origin := clientOrigin(c)
	if t.Graph.Origin() == origin {
		return
	}

	logging.GetLogger().Debugf("Peer unregistered, delete resources of %s", origin)

	t.Graph.Lock()
	delSubGraphOfOrigin(t.cached, t.Graph, origin)
	t.Graph.Unlock()

	delete(t.peerStates, host)
}

// NewReplicationEndpoint returns a new server to be used by other analyzers for replication.
func NewReplicationEndpoint(pool ws.StructSpeakerPool, auth *shttp.AuthenticationOpts, cached *graph.CachedBackend, g *graph.Graph, peers []common.ServiceAddress) (*ReplicationEndpoint, error) {
	t := &ReplicationEndpoint{
		Graph:      g,
		cached:     cached,
		in:         pool,
		out:        ws.NewStructClientPool("ReplicationEndpoint", ws.PoolOpts{}),
		peerStates: make(map[string]*peerState),
	}
	t.replicateMsg.Store(true)

	for _, sa := range peers {
		t.addCandidate(config.GetURL("ws", sa.Addr, sa.Port, "/ws/replication"), auth)
	}

	pool.AddEventHandler(t)

	// subscribe to the local graph event
	g.AddEventListener(t)

	return t, nil
}
