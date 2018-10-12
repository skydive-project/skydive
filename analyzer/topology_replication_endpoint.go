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

package analyzer

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	ws "github.com/skydive-project/skydive/websocket"
)

// TopologyReplicatorPeer is a remote connection to another Graph server. Only modification
// of the local Graph made either by the local server, by an agent message or by an external
// client will be forwarded to the peer.
type TopologyReplicatorPeer struct {
	ws.DefaultSpeakerEventHandler
	URL         *url.URL
	Graph       *graph.Graph
	AuthOptions *shttp.AuthenticationOpts
	wsspeaker   ws.Speaker
	endpoint    *TopologyReplicationEndpoint
}

// TopologyReplicationEndpoint serves the local Graph and send local modification to its peers.
type TopologyReplicationEndpoint struct {
	common.RWMutex
	ws.DefaultSpeakerEventHandler
	in           ws.StructSpeakerPool
	out          *ws.StructClientPool
	inByHost     map[string]ws.Speaker
	outByHost    map[string]*url.URL
	candidates   []*TopologyReplicatorPeer
	Graph        *graph.Graph
	cached       *graph.CachedBackend
	replicateMsg atomic.Value
	wg           sync.WaitGroup
}

func (t *TopologyReplicationEndpoint) debug() bool {
	return config.GetBool("analyzer.replication.debug")
}

// OnConnected is called when the peer gets connected then the whole graph
// is send to initialize it.
func (p *TopologyReplicatorPeer) OnConnected(c ws.Speaker) {
	p.Graph.RLock()
	defer p.Graph.RUnlock()

	p.endpoint.Lock()
	defer p.endpoint.Unlock()

	host := c.GetRemoteHost()

	if host == config.GetString("host_id") {
		logging.GetLogger().Debugf("Disconnecting from %s since it's me", p.URL.String())
		c.Disconnect()
		return
	}

	// disconnect as can be connect to the same host from different addresses.
	if u, ok := p.endpoint.outByHost[host]; ok {
		logging.GetLogger().Debugf("Disconnecting from %s as already connected through", p.URL.String(), u.String())
		c.Disconnect()
		return
	}

	p.wsspeaker.SendMessage(ws.NewStructMessage(graph.Namespace, graph.SyncMsgType, p.Graph))

	p.endpoint.out.AddClient(c)
	p.endpoint.outByHost[host] = c.GetURL()
}

// OnDisconnected is called when the peer gets disconnected
func (p *TopologyReplicatorPeer) OnDisconnected(c ws.Speaker) {
	p.endpoint.Lock()
	defer p.endpoint.Unlock()

	delete(p.endpoint.outByHost, c.GetRemoteHost())
}

func (p *TopologyReplicatorPeer) connect(wg *sync.WaitGroup) {
	defer wg.Done()

	logging.GetLogger().Infof("Connecting to peer: %s", p.URL.String())
	wsClient, err := config.NewWSClient(common.AnalyzerService, p.URL, p.AuthOptions, http.Header{})
	if err != nil {
		logging.GetLogger().Errorf("Failed to create client: %s", err)
		return
	}

	structClient := wsClient.UpgradeToStructSpeaker()
	// will trigger shttp.SpeakerEventHandler, so OnConnected
	structClient.AddEventHandler(p)

	// subscribe to the graph messages
	structClient.AddStructMessageHandler(p.endpoint, []string{graph.Namespace})

	p.wsspeaker = structClient
	p.wsspeaker.Connect()
}

func (p *TopologyReplicatorPeer) disconnect() {
	if p.wsspeaker != nil {
		p.wsspeaker.Disconnect()
	}
}

func (t *TopologyReplicationEndpoint) addCandidate(url *url.URL, auth *shttp.AuthenticationOpts) *TopologyReplicatorPeer {
	peer := &TopologyReplicatorPeer{
		URL:         url,
		Graph:       t.Graph,
		AuthOptions: auth,
		endpoint:    t,
	}

	t.candidates = append(t.candidates, peer)
	return peer
}

// ConnectPeers starts a goroutine connecting all the peers.
func (t *TopologyReplicationEndpoint) ConnectPeers() {
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
func (t *TopologyReplicationEndpoint) DisconnectPeers() {
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
func (t *TopologyReplicationEndpoint) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	host := c.GetRemoteHost()
	if host == config.GetString("host_id") {
		logging.GetLogger().Debugf("Ignore message from myself(%s), %s", c.GetURL().String())
		return
	}

	msgType, obj, err := graph.UnmarshalMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err)
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	t.replicateMsg.Store(false)
	defer t.replicateMsg.Store(true)

	t.cached.SetMode(graph.CacheOnlyMode)
	defer t.cached.SetMode(graph.DefaultMode)

	if t.debug() {
		logging.GetLogger().Debugf("Received message from peer %s: %s", c.GetURL().String(), msg.Bytes(c.GetClientProtocol()))
	}
	switch msgType {
	case graph.SyncRequestMsgType:
		reply := msg.Reply(t.Graph, graph.SyncReplyMsgType, http.StatusOK)
		c.SendMessage(reply)
	case graph.OriginGraphDeletedMsgType:
		logging.GetLogger().Debugf("Got %s message for origin %s", graph.OriginGraphDeletedMsgType, obj.(string))
		t.Graph.DelOriginGraph(obj.(string))
	case graph.SyncMsgType, graph.SyncReplyMsgType:
		r := obj.(*graph.SyncMsg)
		for _, n := range r.Nodes {
			if t.Graph.GetNode(n.ID) == nil {
				t.Graph.NodeAdded(n)
			}
		}
		for _, e := range r.Edges {
			if t.Graph.GetEdge(e.ID) == nil {
				t.Graph.EdgeAdded(e)
			}
		}
	case graph.NodeUpdatedMsgType:
		t.Graph.NodeUpdated(obj.(*graph.Node))
	case graph.NodeDeletedMsgType:
		t.Graph.NodeDeleted(obj.(*graph.Node))
	case graph.NodeAddedMsgType:
		t.Graph.NodeAdded(obj.(*graph.Node))
	case graph.EdgeUpdatedMsgType:
		t.Graph.EdgeUpdated(obj.(*graph.Edge))
	case graph.EdgeDeletedMsgType:
		t.Graph.EdgeDeleted(obj.(*graph.Edge))
	case graph.EdgeAddedMsgType:
		t.Graph.EdgeAdded(obj.(*graph.Edge))
	}
}

// SendToPeers sends the message to all the peers
func (t *TopologyReplicationEndpoint) notifyPeers(msg *ws.StructMessage) {
	if t.debug() {
		logging.GetLogger().Debugf("Broadcasting message to all peers: (protobuf) %s", msg.Bytes(ws.ProtobufProtocol))
	}
	t.in.BroadcastMessage(msg)
	t.out.BroadcastMessage(msg)
}

// OnNodeUpdated graph node updated event. Implements the EventListener interface.
func (t *TopologyReplicationEndpoint) OnNodeUpdated(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := ws.NewStructMessage(graph.Namespace, graph.NodeUpdatedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnNodeAdded graph node added event. Implements the EventListener interface.
func (t *TopologyReplicationEndpoint) OnNodeAdded(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := ws.NewStructMessage(graph.Namespace, graph.NodeAddedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnNodeDeleted graph node deleted event. Implements the EventListener interface.
func (t *TopologyReplicationEndpoint) OnNodeDeleted(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := ws.NewStructMessage(graph.Namespace, graph.NodeDeletedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnEdgeUpdated graph edge updated event. Implements the EventListener interface.
func (t *TopologyReplicationEndpoint) OnEdgeUpdated(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := ws.NewStructMessage(graph.Namespace, graph.EdgeUpdatedMsgType, e)
		t.notifyPeers(msg)
	}
}

// OnEdgeAdded graph edge added event. Implements the EventListener interface.
func (t *TopologyReplicationEndpoint) OnEdgeAdded(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := ws.NewStructMessage(graph.Namespace, graph.EdgeAddedMsgType, e)
		t.notifyPeers(msg)
	}
}

// OnEdgeDeleted graph edge deleted event. Implements the EventListener interface.
func (t *TopologyReplicationEndpoint) OnEdgeDeleted(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := ws.NewStructMessage(graph.Namespace, graph.EdgeDeletedMsgType, e)
		t.notifyPeers(msg)
	}
}

// GetSpeakers return both incoming and outgoing speakers
func (t *TopologyReplicationEndpoint) GetSpeakers() []ws.Speaker {
	return append(t.in.GetSpeakers(), t.out.GetSpeakers()...)
}

// OnConnected is called when an incoming peer got connected.
func (t *TopologyReplicationEndpoint) OnConnected(c ws.Speaker) {
	t.Graph.RLock()
	defer t.Graph.RUnlock()

	t.Lock()
	defer t.Unlock()

	host := c.GetRemoteHost()
	if speaker := t.inByHost[host]; speaker != nil {
		logging.GetLogger().Debugf("Disconnecting %s from %s as already connected from %s", host, c.GetURL(), speaker.GetURL())
		c.Disconnect()
		return
	}

	// subscribe to websocket structured messages
	c.(*ws.StructSpeaker).AddStructMessageHandler(t, []string{graph.Namespace})
	c.SendMessage(ws.NewStructMessage(graph.Namespace, graph.SyncMsgType, t.Graph))
}

// OnDisconnected is called when an incoming peer got disconnected.
func (t *TopologyReplicationEndpoint) OnDisconnected(c ws.Speaker) {
	t.Lock()
	defer t.Unlock()

	delete(t.inByHost, c.GetRemoteHost())
}

// NewTopologyReplicationEndpoint returns a new server to be used by other analyzers for replication.
func NewTopologyReplicationEndpoint(pool ws.StructSpeakerPool, auth *shttp.AuthenticationOpts, cached *graph.CachedBackend, g *graph.Graph) (*TopologyReplicationEndpoint, error) {
	addresses, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err)
	}

	t := &TopologyReplicationEndpoint{
		Graph:     g,
		cached:    cached,
		in:        pool,
		out:       ws.NewStructClientPool("TopologyReplicationEndpoint"),
		inByHost:  make(map[string]ws.Speaker),
		outByHost: make(map[string]*url.URL),
	}
	t.replicateMsg.Store(true)

	for _, sa := range addresses {
		t.addCandidate(config.GetURL("ws", sa.Addr, sa.Port, "/ws/replication"), auth)
	}

	pool.AddEventHandler(t)

	// subscribe to the local graph event
	g.AddEventListener(t)

	return t, nil
}
