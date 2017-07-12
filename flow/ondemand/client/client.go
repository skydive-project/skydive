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

package client

import (
	"encoding/json"
	"net/http"
	"sync"

	cache "github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/flow/ondemand"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

// OnDemandProbeClient describes an ondemand probe client based on a websocket
type OnDemandProbeClient struct {
	sync.RWMutex
	shttp.DefaultWSServerEventHandler
	graph.DefaultGraphListener
	graph            *graph.Graph
	captureHandler   *api.CaptureAPIHandler
	wsServer         *shttp.WSServer
	captures         map[string]*api.Capture
	watcher          api.StoppableWatcher
	elector          *etcd.EtcdMasterElector
	registeredNodes  map[string]bool
	deletedNodeCache *cache.Cache
}

type nodeProbe struct {
	id      string
	host    string
	capture *api.Capture
}

// OnMessage event, valid message type : CaptureStartReply or CaptureStopReply message
func (o *OnDemandProbeClient) OnMessage(c *shttp.WSClient, m shttp.WSMessage) {
	var query ondemand.CaptureQuery
	if err := json.Unmarshal([]byte(*m.Obj), &query); err != nil {
		logging.GetLogger().Errorf("Unable to decode capture %v", m)
		return
	}

	switch m.Type {
	case "CaptureStartReply":
		// not registered thus remove from registered cache
		if m.Status != http.StatusOK {
			logging.GetLogger().Debugf("Capture start request failed %v", m)
			o.Lock()
			delete(o.registeredNodes, query.NodeID)
			o.Unlock()
		} else {
			logging.GetLogger().Debugf("Capture start request succeeded %v", m)
		}
	case "CaptureStopReply":
		if m.Status == http.StatusOK {
			logging.GetLogger().Debugf("Capture stop request succeeded %v", m)
			o.Lock()
			delete(o.registeredNodes, query.NodeID)
			o.Unlock()
		} else {
			logging.GetLogger().Debugf("Capture stop request failed %v", m)
		}
	}
}

func (o *OnDemandProbeClient) registerProbes(nodes []interface{}, capture *api.Capture) {
	toRegister := func(node *graph.Node, capture *api.Capture) (nodeID graph.Identifier, host string, register bool) {
		o.graph.RLock()
		defer o.graph.RUnlock()

		// check not already registered
		o.RLock()
		_, ok := o.registeredNodes[string(node.ID)]
		o.RUnlock()

		if ok {
			return
		}

		if _, err := node.GetFieldString("Capture.ID"); err == nil {
			return
		}
		tp, _ := node.GetFieldString("Type")
		if !common.IsCaptureAllowed(tp) {
			return
		}

		return node.ID, node.Host(), true
	}

	nps := map[graph.Identifier]nodeProbe{}
	for _, i := range nodes {
		switch i.(type) {
		case *graph.Node:
			node := i.(*graph.Node)
			if nodeID, host, ok := toRegister(node, capture); ok {
				nps[nodeID] = nodeProbe{string(nodeID), host, capture}
			}
		case []*graph.Node:
			// case of shortestpath that returns a list of nodes
			for _, node := range i.([]*graph.Node) {
				if nodeID, host, ok := toRegister(node, capture); ok {
					nps[nodeID] = nodeProbe{string(nodeID), host, capture}
				}
			}
		}
	}

	if len(nps) > 0 {
		go func() {
			for _, np := range nps {
				o.registerProbe(np)
			}
		}()
	}
}

func (o *OnDemandProbeClient) registerProbe(np nodeProbe) bool {
	cq := ondemand.CaptureQuery{
		NodeID:  np.id,
		Capture: *np.capture,
	}

	msg := shttp.NewWSMessage(ondemand.Namespace, "CaptureStart", cq)

	if !o.wsServer.SendWSMessageTo(msg, np.host) {
		logging.GetLogger().Errorf("Unable to send message to agent: %s", np.host)
		return false
	}
	o.Lock()
	o.registeredNodes[np.id] = true
	o.Unlock()

	return true
}

func (o *OnDemandProbeClient) unregisterProbe(node *graph.Node, capture *api.Capture) bool {
	cq := ondemand.CaptureQuery{
		NodeID:  string(node.ID),
		Capture: *capture,
	}

	msg := shttp.NewWSMessage(ondemand.Namespace, "CaptureStop", cq)

	if _, err := node.GetFieldString("Capture.ID"); err != nil {
		return false
	}

	if !o.wsServer.SendWSMessageTo(msg, node.Host()) {
		logging.GetLogger().Errorf("Unable to send message to agent: %s", node.Host())
		return false
	}

	return true
}

func (o *OnDemandProbeClient) applyGremlinExpr(query string) []interface{} {
	res, err := topology.ExecuteGremlinQuery(o.graph, query)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin error: %s", err.Error())
		return nil
	}
	return res.Values()
}

func (o *OnDemandProbeClient) onNodeEvent() {
	if !o.elector.IsMaster() {
		return
	}

	for _, capture := range o.captures {
		res := o.applyGremlinExpr(capture.GremlinQuery)
		if len(res) > 0 {
			go o.registerProbes(res, capture)
		}
	}
}

// OnNodeAdded event
func (o *OnDemandProbeClient) OnNodeAdded(n *graph.Node) {
	o.onNodeEvent()
}

// OnNodeUpdated event
func (o *OnDemandProbeClient) OnNodeUpdated(n *graph.Node) {
	o.onNodeEvent()
}

// OnEdgeAdded event
func (o *OnDemandProbeClient) OnEdgeAdded(e *graph.Edge) {
	o.onNodeEvent()
}

func (o *OnDemandProbeClient) registerCapture(capture *api.Capture) {
	o.graph.RLock()
	defer o.graph.RUnlock()

	o.Lock()
	o.captures[capture.UUID] = capture
	o.Unlock()

	nodes := o.applyGremlinExpr(capture.GremlinQuery)
	if len(nodes) > 0 {
		go o.registerProbes(nodes, capture)
	}
}

func (o *OnDemandProbeClient) onCaptureAdded(capture *api.Capture) {
	if !o.elector.IsMaster() {
		return
	}

	o.registerCapture(capture)
}

func (o *OnDemandProbeClient) unregisterCapture(capture *api.Capture) {
	o.graph.Lock()
	defer o.graph.Unlock()

	o.deletedNodeCache.Delete(capture.UUID)

	o.Lock()
	delete(o.captures, capture.UUID)
	o.Unlock()

	res, err := topology.ExecuteGremlinQuery(o.graph, capture.GremlinQuery)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin error: %s", err.Error())
		return
	}

	for _, value := range res.Values() {
		switch e := value.(type) {
		case *graph.Node:
			o.unregisterProbe(e, capture)
		case []*graph.Node:
			for _, node := range e {
				o.unregisterProbe(node, capture)
			}
		}
	}
}

func (o *OnDemandProbeClient) onCaptureDeleted(capture *api.Capture) {
	if !o.elector.IsMaster() {
		// fill the cache with recent delete in order to be able to delete then
		// in case we lose the master and nobody is master yet. This cache will
		// be used when becoming master.
		o.deletedNodeCache.Set(capture.UUID, capture, cache.DefaultExpiration)
		return
	}

	o.unregisterCapture(capture)
}

// OnStartAsMaster event
func (o *OnDemandProbeClient) OnStartAsMaster() {
}

// OnStartAsSlave event
func (o *OnDemandProbeClient) OnStartAsSlave() {
}

// OnSwitchToMaster event
func (o *OnDemandProbeClient) OnSwitchToMaster() {
	// try to delete recently added capture to handle case where the api got a delete but wasn't yet master
	for _, item := range o.deletedNodeCache.Items() {
		capture := item.Object.(*api.Capture)
		o.unregisterCapture(capture)
	}

	for _, resource := range o.captureHandler.Index() {
		capture := resource.(*api.Capture)
		o.onCaptureAdded(capture)
	}
}

// OnSwitchToSlave event
func (o *OnDemandProbeClient) OnSwitchToSlave() {
}

func (o *OnDemandProbeClient) onAPIWatcherEvent(action string, id string, resource api.Resource) {
	logging.GetLogger().Debugf("New watcher event %s for %s", action, id)
	capture := resource.(*api.Capture)
	switch action {
	case "init", "create", "set", "update":
		o.wsServer.BroadcastWSMessage(shttp.NewWSMessage(ondemand.Namespace, "CaptureAdded", capture))
		o.onCaptureAdded(capture)
	case "expire", "delete":
		o.wsServer.BroadcastWSMessage(shttp.NewWSMessage(ondemand.Namespace, "CaptureDeleted", capture))
		o.onCaptureDeleted(capture)
	}
}

// Start the probe
func (o *OnDemandProbeClient) Start() {
	o.elector.StartAndWait()

	o.watcher = o.captureHandler.AsyncWatch(o.onAPIWatcherEvent)
	o.graph.AddEventListener(o)
}

// Stop the probe
func (o *OnDemandProbeClient) Stop() {
	o.watcher.Stop()
	o.elector.Stop()
}

// NewOnDemandProbeClient creates a new ondemand probe client based on Capture API, graph and websocket
func NewOnDemandProbeClient(g *graph.Graph, ch *api.CaptureAPIHandler, w *shttp.WSServer, etcdClient *etcd.EtcdClient) *OnDemandProbeClient {
	resources := ch.Index()
	captures := make(map[string]*api.Capture)
	for _, resource := range resources {
		captures[resource.ID()] = resource.(*api.Capture)
	}

	elector := etcd.NewEtcdMasterElectorFromConfig(common.AnalyzerService, "ondemand-client", etcdClient)

	o := &OnDemandProbeClient{
		graph:            g,
		captureHandler:   ch,
		wsServer:         w,
		captures:         captures,
		elector:          elector,
		registeredNodes:  make(map[string]bool),
		deletedNodeCache: cache.New(elector.TTL()*2, elector.TTL()*2),
	}

	elector.AddEventListener(o)
	w.AddEventHandler(o, []string{ondemand.Namespace})

	return o
}
