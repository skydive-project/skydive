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
	"strings"
	"sync"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/flow/ondemand"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

type OnDemandProbeClient struct {
	sync.RWMutex
	graph.DefaultGraphListener
	graph          *graph.Graph
	captureHandler *api.CaptureApiHandler
	wsServer       *shttp.WSServer
	captures       map[string]*api.Capture
	watcher        api.StoppableWatcher
	parser         *traversal.GremlinTraversalParser
}

func (o *OnDemandProbeClient) registerProbe(node *graph.Node, capture *api.Capture) bool {
	cq := ondemand.CaptureQuery{
		NodeID:  string(node.ID),
		Capture: *capture,
	}

	msg := shttp.NewWSMessage(ondemand.Namespace, "CaptureStart", cq)

	if !o.wsServer.SendWSMessageTo(msg, node.Host()) {
		logging.GetLogger().Errorf("Unable to send message to agent: %s", node.Host())
		return false
	}

	return true
}

func (o *OnDemandProbeClient) unregisterProbe(node *graph.Node) bool {
	msg := shttp.NewWSMessage(ondemand.Namespace, "CaptureStop", ondemand.CaptureQuery{NodeID: string(node.ID)})

	if !o.wsServer.SendWSMessageTo(msg, node.Host()) {
		logging.GetLogger().Errorf("Unable to send message to agent: %s", node.Host())
		return false
	}

	return true
}

func (o *OnDemandProbeClient) matchGremlinExpr(node *graph.Node, gremlin string) bool {
	ts, err := o.parser.Parse(strings.NewReader(gremlin))
	if err != nil {
		logging.GetLogger().Errorf("Gremlin expression error: %s", err.Error())
		return false
	}

	res, err := ts.Exec()
	if err != nil {
		logging.GetLogger().Errorf("Gremlin execution error: %s", err.Error())
		return false
	}

	for _, value := range res.Values() {
		n, ok := value.(*graph.Node)
		if !ok {
			logging.GetLogger().Error("Gremlin expression doesn't return node")
			return false
		}

		if node.ID == n.ID {
			return true
		}
	}

	return false
}

func (o *OnDemandProbeClient) onNodeEvent(n *graph.Node) {
	if state, ok := n.Metadata()["State/FlowCapture"]; ok && state.(string) == "ON" {
		return
	}

	for _, capture := range o.captures {
		if o.matchGremlinExpr(n, capture.GremlinQuery) {
			go o.registerProbe(n, capture)
		}
	}
}

func (o *OnDemandProbeClient) OnNodeAdded(n *graph.Node) {
	o.onNodeEvent(n)
}

func (o *OnDemandProbeClient) OnNodeUpdated(n *graph.Node) {
	o.onNodeEvent(n)
}

func (o *OnDemandProbeClient) OnEdgeAdded(e *graph.Edge) {
	parent, child := o.graph.GetEdgeNodes(e)
	if parent == nil || child == nil || e.Metadata()["RelationType"] != "ownership" {
		return
	}

	o.onNodeEvent(child)
}

func (o *OnDemandProbeClient) onCaptureAdded(capture *api.Capture) {
	o.Lock()
	defer o.Unlock()

	o.graph.RLock()
	defer o.graph.RUnlock()

	o.captures[capture.UUID] = capture

	ts, err := o.parser.Parse(strings.NewReader(capture.GremlinQuery))
	if err != nil {
		logging.GetLogger().Errorf("Gremlin expression error: %s", err.Error())
		return
	}

	res, err := ts.Exec()
	if err != nil {
		logging.GetLogger().Errorf("Gremlin execution error: %s", err.Error())
		return
	}

	for _, value := range res.Values() {
		switch e := value.(type) {
		case *graph.Node:
			if !o.registerProbe(e, capture) {
				logging.GetLogger().Errorf("Failed to start capture on %s", e.ID)
			}
		case []*graph.Node:
			for _, node := range e {
				if !o.registerProbe(node, capture) {
					logging.GetLogger().Errorf("Failed to start capture on %s", node.ID)
				}
			}
		default:
			logging.GetLogger().Error("Gremlin expression doesn't return node")
			return
		}
	}
}

func (o *OnDemandProbeClient) onCaptureDeleted(capture *api.Capture) {
	o.Lock()
	defer o.Unlock()

	o.graph.Lock()
	defer o.graph.Unlock()

	delete(o.captures, capture.UUID)

	ts, err := o.parser.Parse(strings.NewReader(capture.GremlinQuery))
	if err != nil {
		logging.GetLogger().Errorf("Gremlin expression error: %s", err.Error())
		return
	}

	res, err := ts.Exec()
	if err != nil {
		logging.GetLogger().Errorf("Gremlin execution error: %s", err.Error())
		return
	}

	for _, value := range res.Values() {
		switch e := value.(type) {
		case *graph.Node:
			if !o.unregisterProbe(e) {
				logging.GetLogger().Errorf("Failed to stop capture on %s", e.ID)
			}
		case []*graph.Node:
			for _, node := range e {
				if !o.unregisterProbe(node) {
					logging.GetLogger().Errorf("Failed to stop capture on %s", node.ID)
				}
			}
		default:
			logging.GetLogger().Error("Gremlin expression doesn't return node")
			return
		}
	}
}

func (o *OnDemandProbeClient) onApiWatcherEvent(action string, id string, resource api.ApiResource) {
	logging.GetLogger().Debugf("New watcher event %s for %s", action, id)
	capture := resource.(*api.Capture)
	switch action {
	case "init", "create", "set", "update":
		o.onCaptureAdded(capture)
	case "expire", "delete":
		o.onCaptureDeleted(capture)
	}
}

func (o *OnDemandProbeClient) Start() {
	o.watcher = o.captureHandler.AsyncWatch(o.onApiWatcherEvent)
	o.graph.AddEventListener(o)
}

func (o *OnDemandProbeClient) Stop() {
	o.watcher.Stop()
}

func NewOnDemandProbeClient(g *graph.Graph, ch *api.CaptureApiHandler, w *shttp.WSServer) *OnDemandProbeClient {
	resources := ch.List()
	captures := make(map[string]*api.Capture)
	for _, resource := range resources {
		captures[resource.ID()] = resource.(*api.Capture)
	}

	return &OnDemandProbeClient{
		graph:          g,
		captureHandler: ch,
		wsServer:       w,
		captures:       captures,
		parser:         traversal.NewGremlinTraversalParser(g),
	}
}
