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
	"time"

	cache "github.com/pmylund/go-cache"

	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/flow/ondemand"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// OnDemandProbeClient describes an ondemand probe client based on a websocket
type OnDemandProbeClient struct {
	sync.RWMutex
	*etcd.MasterElector
	graph.DefaultGraphListener
	graph                *graph.Graph
	captureHandler       *api.CaptureAPIHandler
	agentPool            shttp.WSJSONSpeakerPool
	subscriberPool       shttp.WSJSONSpeakerPool
	captures             map[string]*types.Capture
	watcher              api.StoppableWatcher
	registeredNodes      map[string]string
	deletedNodeCache     *cache.Cache
	checkForRegistration *common.Debouncer
}

type nodeProbe struct {
	id      string
	host    string
	capture *types.Capture
}

// OnMessage event, valid message type : CaptureStartReply or CaptureStopReply message
func (o *OnDemandProbeClient) OnWSJSONMessage(c shttp.WSSpeaker, m *shttp.WSJSONMessage) {
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
		o.subscriberPool.BroadcastMessage(shttp.NewWSJSONMessage(ondemand.NotificationNamespace, "CaptureNodeUpdated", query.Capture.UUID))
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

func (o *OnDemandProbeClient) registerProbes(nodes []interface{}, capture *types.Capture) {
	toRegister := func(node *graph.Node, capture *types.Capture) (nodeID graph.Identifier, host string, register bool) {
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

	msg := shttp.NewWSJSONMessage(ondemand.Namespace, "CaptureStart", cq)

	if err := o.agentPool.SendMessageTo(msg, np.host); err != nil {
		logging.GetLogger().Errorf("Unable to send message to agent %s: %s", np.host, err.Error())
		return false
	}
	o.Lock()
	o.registeredNodes[np.id] = cq.Capture.ID()
	o.Unlock()

	return true
}

func (o *OnDemandProbeClient) unregisterProbe(node *graph.Node, capture *types.Capture) bool {
	cq := ondemand.CaptureQuery{
		NodeID:  string(node.ID),
		Capture: *capture,
	}

	msg := shttp.NewWSJSONMessage(ondemand.Namespace, "CaptureStop", cq)

	if _, err := node.GetFieldString("Capture.ID"); err != nil {
		return false
	}

	if err := o.agentPool.SendMessageTo(msg, node.Host()); err != nil {
		logging.GetLogger().Errorf("Unable to send message to agent %s: %s", node.Host(), err.Error())
		return false
	}

	return true
}

func (o *OnDemandProbeClient) applyGremlinExpr(query string) []interface{} {
	res, err := ge.TopologyGremlinQuery(o.graph, query)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin %s error: %s", query, err.Error())
		return nil
	}
	return res.Values()
}

// checkForRegistration check the capture gremlin expression in order to
// register new probe.
func (o *OnDemandProbeClient) checkForRegistrationCallback() {
	if !o.IsMaster() {
		return
	}

	o.graph.RLock()
	defer o.graph.RUnlock()

	o.RLock()
	defer o.RUnlock()

	for _, capture := range o.captures {
		res := o.applyGremlinExpr(capture.GremlinQuery)
		if len(res) > 0 {
			go o.registerProbes(res, capture)
		}
	}
}

// OnNodeAdded graph event
func (o *OnDemandProbeClient) OnNodeAdded(n *graph.Node) {
	// a node comes up with already a capture, this could be due to a re-connect of
	// an agent. Check if the capture is still active.
	if id, err := n.GetFieldString("Capture.ID"); err == nil {
		if !o.IsMaster() {
			return
		}

		o.RLock()
		_, found := o.captures[id]
		o.RUnlock()

		if found {
			return
		}

		// not present unregister it
		logging.GetLogger().Debugf("Unregister remaining capture for node %s: %s", n.ID, id)
		go o.unregisterProbe(n, &types.Capture{UUID: id})
	} else {
		o.checkForRegistration.Call()
	}
}

// OnNodeUpdated graph event
func (o *OnDemandProbeClient) OnNodeUpdated(n *graph.Node) {
	o.checkForRegistration.Call()
}

// OnNodeDeleted graph event
func (o *OnDemandProbeClient) OnNodeDeleted(n *graph.Node) {
	o.RLock()
	if uuid, ok := o.registeredNodes[string(n.ID)]; ok {
		o.subscriberPool.BroadcastMessage(shttp.NewWSJSONMessage(ondemand.NotificationNamespace, "CaptureNodeUpdated", uuid))
	}
	o.RUnlock()
}

// OnEdgeAdded graph event
func (o *OnDemandProbeClient) OnEdgeAdded(e *graph.Edge) {
	o.checkForRegistration.Call()
}

func (o *OnDemandProbeClient) registerCapture(capture *types.Capture) {
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

func (o *OnDemandProbeClient) onCaptureAdded(capture *types.Capture) {
	if !o.IsMaster() {
		return
	}

	o.registerCapture(capture)
}

func (o *OnDemandProbeClient) unregisterCapture(capture *types.Capture) {
	o.graph.Lock()
	defer o.graph.Unlock()

	o.deletedNodeCache.Delete(capture.UUID)

	o.Lock()
	delete(o.captures, capture.UUID)
	o.Unlock()

	res, err := ge.TopologyGremlinQuery(o.graph, capture.GremlinQuery)
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

func (o *OnDemandProbeClient) onCaptureDeleted(capture *types.Capture) {
	if !o.IsMaster() {
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
		capture := item.Object.(*types.Capture)
		o.unregisterCapture(capture)
	}

	for _, resource := range o.captureHandler.Index() {
		capture := resource.(*types.Capture)
		o.onCaptureAdded(capture)
	}
}

// OnSwitchToSlave event
func (o *OnDemandProbeClient) OnSwitchToSlave() {
}

func (o *OnDemandProbeClient) onAPIWatcherEvent(action string, id string, resource types.Resource) {
	logging.GetLogger().Debugf("New watcher event %s for %s", action, id)
	capture := resource.(*types.Capture)
	switch action {
	case "init", "create", "set", "update":
		o.subscriberPool.BroadcastMessage(shttp.NewWSJSONMessage(ondemand.NotificationNamespace, "CaptureAdded", capture))
		o.onCaptureAdded(capture)
	case "expire", "delete":
		o.subscriberPool.BroadcastMessage(shttp.NewWSJSONMessage(ondemand.NotificationNamespace, "CaptureDeleted", capture))
		o.onCaptureDeleted(capture)
	}
}

// Start the probe
func (o *OnDemandProbeClient) Start() {
	o.MasterElector.StartAndWait()

	o.checkForRegistration.Start()

	o.watcher = o.captureHandler.AsyncWatch(o.onAPIWatcherEvent)
	o.graph.AddEventListener(o)
}

// Stop the probe
func (o *OnDemandProbeClient) Stop() {
	o.watcher.Stop()
	o.MasterElector.Stop()
	o.checkForRegistration.Stop()
}

// InvokeCaptureFromConfig invokes capture based on preconfigured selected SubGraph
func (o *OnDemandProbeClient) InvokeCaptureFromConfig() {
	gremlin := config.GetString("analyzer.startup.capture_gremlin")
	bpf := config.GetString("analyzer.startup.capture_bpf")
	if gremlin == "" {
		return
	}
	logging.GetLogger().Infof("Invoke capturing from the startup with gremlin: %s and BPF: %s", gremlin, bpf)
	capture := types.NewCapture(gremlin, bpf)
	capture.SocketInfo = true
	capture.Type = "pcap"
	o.onCaptureAdded(capture)
	return
}

// NewOnDemandProbeClient creates a new ondemand probe client based on Capture API, graph and websocket
func NewOnDemandProbeClient(g *graph.Graph, ch *api.CaptureAPIHandler, agentPool shttp.WSJSONSpeakerPool, subscriberPool shttp.WSJSONSpeakerPool, etcdClient *etcd.Client) *OnDemandProbeClient {
	resources := ch.Index()
	captures := make(map[string]*types.Capture)
	for _, resource := range resources {
		captures[resource.ID()] = resource.(*types.Capture)
	}

	elector := etcd.NewMasterElectorFromConfig(common.AnalyzerService, "ondemand-client", etcdClient)

	o := &OnDemandProbeClient{
		MasterElector:    elector,
		graph:            g,
		captureHandler:   ch,
		agentPool:        agentPool,
		subscriberPool:   subscriberPool,
		captures:         captures,
		registeredNodes:  make(map[string]string),
		deletedNodeCache: cache.New(elector.TTL()*2, elector.TTL()*2),
	}
	o.checkForRegistration = common.NewDebouncer(time.Second, o.checkForRegistrationCallback)

	elector.AddEventListener(o)
	agentPool.AddJSONMessageHandler(o, []string{ondemand.Namespace})
	o.InvokeCaptureFromConfig()

	return o
}
