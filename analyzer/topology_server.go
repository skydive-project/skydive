/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/statics"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	"github.com/xeipuuv/gojsonschema"
)

// TopologyReplicatorPeer is a remote connection to another Graph server. Only modification
// of the local Graph made either by the local server, by an agent message or by an external
// client will be forwarded to the peer.
type TopologyReplicatorPeer struct {
	shttp.DefaultWSSpeakerEventHandler
	URL         *url.URL
	Graph       *graph.Graph
	AuthOptions *shttp.AuthenticationOpts
	wsclient    *shttp.WSClient
	host        string
}

type topologySubscriber struct {
	graph         *graph.Graph
	gremlinFilter string
	ts            *traversal.GremlinTraversalSequence
}

// TopologyServer serves the local Graph and send local modification to its peers.
// Only modification of the local Graph made either by the local server,
// by an agent message or by an external client will be forwarded to the peers.
type TopologyServer struct {
	sync.RWMutex
	shttp.DefaultWSSpeakerEventHandler
	pool          shttp.WSJSONSpeakerPool
	Graph         *graph.Graph
	cached        *graph.CachedBackend
	nodeSchema    gojsonschema.JSONLoader
	edgeSchema    gojsonschema.JSONLoader
	authors       map[string]bool // authors of graph modification meaning not forwarding them.
	peers         []*TopologyReplicatorPeer
	wg            sync.WaitGroup
	replicateMsg  atomic.Value
	gremlinParser *traversal.GremlinTraversalParser
	subscribers   map[string]*topologySubscriber
}

// getHostID loop until being able to get the host-id of the peer.
func (p *TopologyReplicatorPeer) getHostID() string {
	port, _ := strconv.Atoi(p.URL.Port())
	client := shttp.NewRestClient(config.GetURL("http", p.URL.Hostname(), port, ""), p.AuthOptions)
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

	logging.GetLogger().Infof("Send the whole graph to: %s", p.Graph.GetHost())
	p.wsclient.SendMessage(shttp.NewWSJSONMessage(graph.Namespace, graph.SyncMsgType, p.Graph))
}

func (p *TopologyReplicatorPeer) connect(wg *sync.WaitGroup) {
	defer wg.Done()

	// check whether the peer is the local server itself or not thanks to the /api
	// the goal is to not add itself as peer.
	if p.getHostID() == config.GetConfig().GetString("host_id") {
		logging.GetLogger().Debugf("No connection to %s since it's me", p.URL.String())
		return
	}

	authPort, _ := strconv.Atoi(p.URL.Port())
	authClient := shttp.NewAuthenticationClient(config.GetURL("http", p.URL.Hostname(), authPort, ""), p.AuthOptions)
	p.wsclient = shttp.NewWSClientFromConfig(common.AnalyzerService, p.URL, authClient, http.Header{})

	// will trigger shttp.WSSpeakerEventHandler, so OnConnected
	p.wsclient.AddEventHandler(p)

	p.wsclient.Connect()
}

func (p *TopologyReplicatorPeer) disconnect() {
	if p.wsclient != nil {
		p.wsclient.Disconnect()
	}
}

func (t *TopologyServer) getGraph(gremlinQuery string, ts *traversal.GremlinTraversalSequence) (*graph.Graph, error) {
	res, err := ts.Exec(t.Graph, false)
	if err != nil {
		return nil, err
	}

	tv, ok := res.(*traversal.GraphTraversal)
	if !ok {
		return nil, fmt.Errorf("Gremlin query '%s' did not return a graph", gremlinQuery)
	}

	return tv.Graph, nil
}

func (t *TopologyServer) addPeer(url *url.URL, auth *shttp.AuthenticationOpts, g *graph.Graph) {
	peer := &TopologyReplicatorPeer{
		URL:         url,
		Graph:       g,
		AuthOptions: auth,
	}

	t.peers = append(t.peers, peer)
}

func (t *TopologyServer) newTopologySubscriber(host string, gremlinFilter string) (*topologySubscriber, error) {
	ts, err := t.gremlinParser.Parse(strings.NewReader(gremlinFilter))
	if err != nil {
		return nil, fmt.Errorf("Invalid Gremlin filter '%s' for client %s", gremlinFilter, host)
	}

	g, err := t.getGraph(gremlinFilter, ts)
	if err != nil {
		return nil, err
	}

	return &topologySubscriber{graph: g, ts: ts, gremlinFilter: gremlinFilter}, nil
}

// ConnectPeers starts a goroutine connecting all the peers.
func (t *TopologyServer) ConnectPeers() {
	for _, peer := range t.peers {
		t.wg.Add(1)
		go peer.connect(&t.wg)
	}
}

// DisconnectPeers disconnects all the peers and wait until all disconnected.
func (t *TopologyServer) DisconnectPeers() {
	for _, peer := range t.peers {
		peer.disconnect()
	}
	t.wg.Wait()
}

func (t *TopologyServer) hostGraphDeleted(host string, mode int) {
	t.cached.SetMode(mode)
	defer t.cached.SetMode(graph.DefaultMode)

	t.Graph.DelHostGraph(host)
}

// OnConnected called when a WSSpeaker got connected. The WSSPeaker can be
// either a peer, an agent or an external client.
func (t *TopologyServer) OnConnected(c shttp.WSSpeaker) {
	gremlinFilter := c.GetHeaders().Get("X-Gremlin-Filter")
	if gremlinFilter == "" {
		gremlinFilter = c.GetURL().Query().Get("x-gremlin-filter")
	}

	if gremlinFilter != "" {
		subscriber, err := t.newTopologySubscriber(c.GetHost(), gremlinFilter)
		if err != nil {
			logging.GetLogger().Error(err)
			return
		}

		logging.GetLogger().Infof("Client %s subscribed with filter %s", c.GetHost(), gremlinFilter)
		t.subscribers[c.GetHost()] = subscriber
	}
}

// OnDisconnected called when a WSSpeaker got disconnected. The WSSPeaker can be
// either a peer, an agent or an external client.
func (t *TopologyServer) OnDisconnected(c shttp.WSSpeaker) {
	host := c.GetHost()

	t.RLock()
	_, ok := t.authors[host]
	t.RUnlock()

	// not an author so do not delete resources
	if !ok {
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	// it's an authors so already received a message meaning that the client chose this analyzer as master
	logging.GetLogger().Debugf("Authoritative client unregistered, delete resources %s", host)
	t.hostGraphDeleted(host, graph.DefaultMode)

	t.Lock()
	delete(t.authors, host)
	delete(t.subscribers, host)
	t.Unlock()
}

func (t *TopologyServer) sendSyncReply(c shttp.WSSpeaker, msg *shttp.WSJSONMessage, result interface{}, status int) {
	reply := msg.Reply(result, graph.SyncReplyMsgType, status)
	c.SendMessage(reply)
}

// OnWSJSONMessage is triggered by message coming from websocket Speaker. It can be
// any kind of client, peer, agent, external client.
func (t *TopologyServer) OnWSJSONMessage(c shttp.WSSpeaker, msg *shttp.WSJSONMessage) {
	msgType, obj, err := graph.UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

	// this kind of message usually comes from external clients like the WebUI
	if msgType == graph.SyncRequestMsgType {
		t.Graph.RLock()
		delete(t.subscribers, c.GetHost())
		syncMsg, status := obj.(graph.SyncRequestMsg), http.StatusOK
		g, err := t.Graph.WithContext(syncMsg.GraphContext)
		var result interface{} = g

		if err != nil {
			logging.GetLogger().Errorf("analyzer is unable to get a graph with context %+v: %s", syncMsg, err.Error())
			t.Graph.RUnlock()
			t.sendSyncReply(c, msg, nil, http.StatusBadRequest)
			return
		}
		if syncMsg.GremlinFilter != "" {
			subscriber, err := t.newTopologySubscriber(c.GetHost(), syncMsg.GremlinFilter)
			if err != nil {
				logging.GetLogger().Error(err)
				t.Graph.RUnlock()
				t.sendSyncReply(c, msg, nil, http.StatusBadRequest)
				return
			}

			logging.GetLogger().Infof("Client %s subscribed with filter %s", c.GetHost(), syncMsg.GremlinFilter)
			result = subscriber.graph
			t.subscribers[c.GetHost()] = subscriber
		}
		t.Graph.RUnlock()
		t.sendSyncReply(c, msg, result, status)
		return
	}

	serviceType := c.GetServiceType()

	// set this WSSpeaker as author if a serviceType is provided and it is not a peer.
	// So basically an author is a WSSpeaker not forwarding a message.
	if serviceType != "" && serviceType != common.AnalyzerService {
		t.Lock()
		t.authors[c.GetHost()] = true
		t.Unlock()
	} else {
		// if the message doesn't come from an author meaning that it comes from
		// a peer. In that case we don't have to forward it otherwise it leads to
		// infinite forwarding loop.
		t.replicateMsg.Store(false)
		defer t.replicateMsg.Store(true)
	}

	// If the message comes from an external WSSpeaker we use the schema to validate the
	// message.
	if serviceType != common.AnalyzerService && serviceType != common.AgentService {
		loader := gojsonschema.NewGoLoader(obj)

		var schema gojsonschema.JSONLoader
		switch msgType {
		case graph.NodeAddedMsgType, graph.NodeUpdatedMsgType, graph.NodeDeletedMsgType:
			schema = t.nodeSchema
		case graph.EdgeAddedMsgType, graph.EdgeUpdatedMsgType, graph.EdgeDeletedMsgType:
			schema = t.edgeSchema
		}

		if schema != nil {
			if _, err := gojsonschema.Validate(t.edgeSchema, loader); err != nil {
				logging.GetLogger().Errorf("Invalid message: %s", err.Error())
				return
			}
		}
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	// HostGraphDeletedMsgType is handled specifically as we need to be sure to not use the
	// cache while deleting otherwise the delete mechanism is using the cache to walk throught
	// the graph.
	if msgType == graph.HostGraphDeletedMsgType {
		logging.GetLogger().Debugf("Got %s message for host %s", graph.HostGraphDeletedMsgType, obj.(string))

		t.hostGraphDeleted(obj.(string), graph.CacheOnlyMode)
		if serviceType != common.AnalyzerService {
			t.hostGraphDeleted(obj.(string), graph.PersistentOnlyMode)
		}

		return
	}

	// If the message comes from analyzer we only need to apply it on cache
	// as it is a forwarded message.
	if serviceType == common.AnalyzerService {
		t.cached.SetMode(graph.CacheOnlyMode)
	}
	defer t.cached.SetMode(graph.DefaultMode)

	switch msgType {
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

// notifyClients aims to forward local graph modification to external clients
// the goal here is not to handle analyzer replication.
func (t *TopologyServer) notifyClients(msg *shttp.WSJSONMessage, id graph.Identifier) {
	for _, c := range t.pool.GetSpeakers() {
		serviceType := c.GetServiceType()
		if serviceType != common.AnalyzerService && serviceType != common.AgentService {
			if subscriber, found := t.subscribers[c.GetHost()]; found {
				g, err := t.getGraph(subscriber.gremlinFilter, subscriber.ts)
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

				if (msg.Type == graph.NodeUpdatedMsgType && g.GetNode(id) != nil) ||
					(msg.Type == graph.EdgeUpdatedMsgType && g.GetEdge(id) != nil) {
					c.SendMessage(msg)
				}

				subscriber.graph = g
			} else {
				c.SendMessage(msg)
			}
		}
	}
}

// SendToPeers sends the message to all the peers
func (t *TopologyServer) notifyPeers(msg *shttp.WSJSONMessage) {
	for _, p := range t.peers {
		if p.wsclient != nil {
			p.wsclient.SendMessage(msg)
		}
	}
}

// OnNodeUpdated graph node updated event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnNodeUpdated(n *graph.Node) {
	msg := shttp.NewWSJSONMessage(graph.Namespace, graph.NodeUpdatedMsgType, n)
	t.notifyClients(msg, n.ID)
	if t.replicateMsg.Load() == true {
		t.notifyPeers(msg)
	}
}

// OnNodeAdded graph node added event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnNodeAdded(n *graph.Node) {
	msg := shttp.NewWSJSONMessage(graph.Namespace, graph.NodeAddedMsgType, n)
	t.notifyClients(msg, n.ID)
	if t.replicateMsg.Load() == true {
		t.notifyPeers(msg)
	}
}

// OnNodeDeleted graph node deleted event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnNodeDeleted(n *graph.Node) {
	msg := shttp.NewWSJSONMessage(graph.Namespace, graph.NodeDeletedMsgType, n)
	t.notifyClients(msg, n.ID)
	if t.replicateMsg.Load() == true {
		t.notifyPeers(msg)
	}
}

// OnEdgeUpdated graph edge updated event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnEdgeUpdated(e *graph.Edge) {
	msg := shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeUpdatedMsgType, e)
	t.notifyClients(msg, e.ID)
	if t.replicateMsg.Load() == true {
		t.notifyPeers(msg)
	}
}

// OnEdgeAdded graph edge added event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnEdgeAdded(e *graph.Edge) {
	msg := shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeAddedMsgType, e)
	t.notifyClients(msg, e.ID)
	if t.replicateMsg.Load() == true {
		t.notifyPeers(msg)
	}
}

// OnEdgeDeleted graph edge deleted event. Implements the GraphEventListener interface.
func (t *TopologyServer) OnEdgeDeleted(e *graph.Edge) {
	msg := shttp.NewWSJSONMessage(graph.Namespace, graph.EdgeDeletedMsgType, e)
	t.notifyClients(msg, e.ID)
	if t.replicateMsg.Load() == true {
		t.notifyPeers(msg)
	}
}

// NewTopologyServer returns a new server which servers the local Graph and replicates messages
// coming from agents or external clients.
func NewTopologyServer(pool shttp.WSJSONSpeakerPool, auth *shttp.AuthenticationOpts) (*TopologyServer, error) {
	persistent, err := graph.BackendFromConfig()
	if err != nil {
		return nil, err
	}

	cached, err := graph.NewCachedBackend(persistent)
	if err != nil {
		return nil, err
	}

	g := graph.NewGraphFromConfig(cached)

	nodeSchema, err := statics.Asset("statics/schemas/node.schema")
	if err != nil {
		return nil, err
	}

	edgeSchema, err := statics.Asset("statics/schemas/edge.schema")
	if err != nil {
		return nil, err
	}

	addresses, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err)
	}

	t := &TopologyServer{
		Graph:         g,
		pool:          pool,
		cached:        cached,
		authors:       make(map[string]bool),
		nodeSchema:    gojsonschema.NewBytesLoader(nodeSchema),
		edgeSchema:    gojsonschema.NewBytesLoader(edgeSchema),
		subscribers:   make(map[string]*topologySubscriber),
		gremlinParser: traversal.NewGremlinTraversalParser(),
	}
	t.replicateMsg.Store(true)

	pool.AddEventHandler(t)

	// subscribe to the graph messages
	pool.AddJSONMessageHandler(t, []string{graph.Namespace})

	// subscribe to the local graph event
	g.AddEventListener(t)

	for _, sa := range addresses {
		t.addPeer(config.GetURL("ws", sa.Addr, sa.Port, "/ws"), auth, g)
	}

	return t, nil
}
