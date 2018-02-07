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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// TopologyReplicatorPeer is a remote connection to another Graph server. Only modification
// of the local Graph made either by the local server, by an agent message or by an external
// client will be forwarded to the peer.
type TopologyReplicatorPeer struct {
	shttp.DefaultWSSpeakerEventHandler
	URL         *url.URL
	Graph       *graph.Graph
	AuthOptions *shttp.AuthenticationOpts
	wsspeaker   shttp.WSSpeaker
	endpoint    *TopologyReplicationEndpoint
	host        string
	ephemeral   bool
}

// TopologyReplicationEndpoint serves the local Graph and send local modification to its peers.
type TopologyReplicationEndpoint struct {
	sync.RWMutex
	shttp.DefaultWSSpeakerEventHandler
	in           shttp.WSJSONSpeakerPool
	out          *shttp.WSJSONClientPool
	conns        map[string]shttp.WSSpeaker
	candidates   []*TopologyReplicatorPeer
	Graph        *graph.Graph
	cached       *graph.CachedBackend
	replicateMsg atomic.Value
	wg           sync.WaitGroup
}

func (t *TopologyReplicationEndpoint) debug() bool {
	return config.GetBool("analyzer.replication.debug")
}

// getHostID loop until being able to get the host-id of the peer.
func (p *TopologyReplicatorPeer) getHostID() string {
	addr := common.NormalizeAddrForURL(p.URL.Hostname())
	port, _ := strconv.Atoi(p.URL.Port())
	client := shttp.NewRestClient(config.GetURL("http", addr, port, ""), p.AuthOptions)
	contentReader := bytes.NewReader([]byte{})

	var data []byte
	var info api.Info

	for {
		resp, err := client.Request("GET", "api", contentReader, nil)
		if err != nil {
			goto NotReady
		}

		if resp.StatusCode != http.StatusOK {
			goto NotReady
		}

		data, _ = ioutil.ReadAll(resp.Body)
		if len(data) == 0 {
			goto NotReady
		}

		if err := json.Unmarshal(data, &info); err != nil {
			goto NotReady
		}
		p.host = info.Host

		return p.host

	NotReady:
		time.Sleep(1 * time.Second)
	}
}

// OnConnected is called when the peer gets connected then the whole graph
// is send to initialize it.
func (p *TopologyReplicatorPeer) OnConnected(c shttp.WSSpeaker) {
	p.Graph.RLock()
	defer p.Graph.RUnlock()

	p.endpoint.Lock()
	defer p.endpoint.Unlock()

	if _, found := p.endpoint.conns[p.host]; found {
		c.Disconnect()
		return
	}

	p.wsspeaker.SendMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.SyncMsgType, p.Graph))

	p.endpoint.conns[p.host] = c
	p.endpoint.out.AddClient(c)
}

// OnConnected is called when the peer gets Disconnected
func (p *TopologyReplicatorPeer) OnDisconnected(c shttp.WSSpeaker) {
	p.endpoint.Lock()
	defer p.endpoint.Unlock()

	if peer, found := p.endpoint.conns[p.host]; found {
		for _, outgoer := range p.endpoint.out.GetSpeakers() {
			if outgoer.GetHost() == peer.GetHost() {
				p.endpoint.out.RemoveClient(c)
				delete(p.endpoint.conns, p.host)
				break
			}
		}
	}
}

func (p *TopologyReplicatorPeer) connect(wg *sync.WaitGroup) {
	defer wg.Done()

	// check whether the peer is the local server itself or not thanks to the /api
	// the goal is to not add itself as peer.
	if p.getHostID() == config.GetString("host_id") {
		logging.GetLogger().Debugf("No connection to %s since it's me", p.URL.String())
		return
	}

	authAddr := common.NormalizeAddrForURL(p.URL.Hostname())
	authPort, _ := strconv.Atoi(p.URL.Port())
	authClient := shttp.NewAuthenticationClient(config.GetURL("http", authAddr, authPort, ""), p.AuthOptions)
	wsClient := shttp.NewWSClientFromConfig(common.AnalyzerService, p.URL, authClient, http.Header{}).UpgradeToWSJSONSpeaker()

	// will trigger shttp.WSSpeakerEventHandler, so OnConnected
	wsClient.AddEventHandler(p)

	// subscribe to the graph messages
	wsClient.AddJSONMessageHandler(p.endpoint, []string{graph.Namespace})

	p.wsspeaker = wsClient
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

// OnWSJSONMessage is triggered by message coming from an other peer.
func (t *TopologyReplicationEndpoint) OnWSJSONMessage(c shttp.WSSpeaker, msg *shttp.WSJSONMessage) {
	msgType, obj, err := graph.UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	t.replicateMsg.Store(false)
	defer t.replicateMsg.Store(true)

	t.cached.SetMode(graph.CacheOnlyMode)
	defer t.cached.SetMode(graph.DefaultMode)

	if t.debug() {
		logging.GetLogger().Debugf("Recieved message from peer %s: %s", c.GetURL().String(), msg.Bytes())
	}
	switch msgType {
	case graph.SyncRequestMsgType:
		reply := msg.Reply(t.Graph, graph.SyncReplyMsgType, http.StatusOK)
		c.SendMessage(reply)
	case graph.HostGraphDeletedMsgType:
		logging.GetLogger().Debugf("Got %s message for host %s", graph.HostGraphDeletedMsgType, obj.(string))
		t.Graph.DelHostGraph(obj.(string))
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
func (t *TopologyReplicationEndpoint) notifyPeers(msg *shttp.WSJSONMessage) {
	if t.debug() {
		logging.GetLogger().Debugf("Broadcasting message to all peers: %s", msg.Bytes())
	}
	t.in.BroadcastMessage(msg)
	t.out.BroadcastMessage(msg)
}

// OnNodeUpdated graph node updated event. Implements the GraphEventListener interface.
func (t *TopologyReplicationEndpoint) OnNodeUpdated(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := shttp.NewWSJSONMessage(graph.Namespace, graph.NodeUpdatedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnNodeAdded graph node added event. Implements the GraphEventListener interface.
func (t *TopologyReplicationEndpoint) OnNodeAdded(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := shttp.NewWSJSONMessage(graph.Namespace, graph.NodeAddedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnNodeDeleted graph node deleted event. Implements the GraphEventListener interface.
func (t *TopologyReplicationEndpoint) OnNodeDeleted(n *graph.Node) {
	if t.replicateMsg.Load() == true {
		msg := shttp.NewWSJSONMessage(graph.Namespace, graph.NodeDeletedMsgType, n)
		t.notifyPeers(msg)
	}
}

// OnEdgeUpdated graph edge updated event. Implements the GraphEventListener interface.
func (t *TopologyReplicationEndpoint) OnEdgeUpdated(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeUpdatedMsgType, e)
		t.notifyPeers(msg)
	}
}

// OnEdgeAdded graph edge added event. Implements the GraphEventListener interface.
func (t *TopologyReplicationEndpoint) OnEdgeAdded(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeAddedMsgType, e)
		t.notifyPeers(msg)
	}
}

// OnEdgeDeleted graph edge deleted event. Implements the GraphEventListener interface.
func (t *TopologyReplicationEndpoint) OnEdgeDeleted(e *graph.Edge) {
	if t.replicateMsg.Load() == true {
		msg := shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeDeletedMsgType, e)
		t.notifyPeers(msg)
	}
}

func (t *TopologyReplicationEndpoint) GetSpeakers() []shttp.WSSpeaker {
	return append(t.in.GetSpeakers(), t.out.GetSpeakers()...)
}

func (t *TopologyReplicationEndpoint) getPeer(host string) shttp.WSSpeaker {
	for _, peer := range t.GetSpeakers() {
		if peer.GetHost() == host {
			return peer
		}
	}

	return nil
}

// OnConnected is called when an incoming peer got connected.
func (t *TopologyReplicationEndpoint) OnConnected(c shttp.WSSpeaker) {
	t.Graph.RLock()
	defer t.Graph.RUnlock()

	t.Lock()
	defer t.Unlock()

	if _, found := t.conns[c.GetHost()]; found {
		c.Disconnect()
		return
	}

	t.conns[c.GetHost()] = c

	// subscribe to JSON messages
	c.(*shttp.WSJSONSpeaker).AddJSONMessageHandler(t, []string{graph.Namespace})
	c.SendMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.SyncMsgType, t.Graph))
}

// OnDisconnected is called when an incoming peer got disconnected.
func (t *TopologyReplicationEndpoint) OnDisconnected(c shttp.WSSpeaker) {
	t.Lock()
	defer t.Unlock()

	if peer, found := t.conns[c.GetHost()]; found {
		if peer.GetHost() == c.GetHost() {
			delete(t.conns, c.GetHost())
		}
	}
}

// NewTopologyServer returns a new server to be used by other analyzers for replication.
func NewTopologyReplicationEndpoint(pool shttp.WSJSONSpeakerPool, auth *shttp.AuthenticationOpts, cached *graph.CachedBackend, g *graph.Graph) (*TopologyReplicationEndpoint, error) {
	addresses, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err)
	}

	t := &TopologyReplicationEndpoint{
		Graph:  g,
		cached: cached,
		in:     pool,
		out:    shttp.NewWSJSONClientPool("TopologyReplicationEndpoint"),
		conns:  make(map[string]shttp.WSSpeaker),
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
