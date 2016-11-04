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

package probes

import (
	"fmt"
	"strings"
	"sync"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

type OnDemandProbeListener struct {
	sync.RWMutex
	graph.DefaultGraphListener
	Graph          *graph.Graph
	Probes         *FlowProbeBundle
	CaptureHandler *api.CaptureApiHandler
	watcher        api.StoppableWatcher
	fta            *flow.TableAllocator
	activeProbes   map[graph.Identifier]*flow.Table
	captures       map[graph.Identifier]*api.Capture
}

func (o *OnDemandProbeListener) isActive(n *graph.Node) bool {
	o.RLock()
	defer o.RUnlock()
	_, active := o.activeProbes[n.ID]

	return active
}

func (o *OnDemandProbeListener) getProbe(n *graph.Node, capture *api.Capture) (*FlowProbe, error) {
	capType := ""
	if capture.Type != "" {
		types := common.CaptureTypes[n.Metadata()["Type"].(string)].Allowed
		for _, t := range types {
			if t == capture.Type {
				capType = t
				break
			}
		}
		if capType == "" {
			return nil, fmt.Errorf("Capture type %v not allowed on this node: %v", capture, n)
		}
	} else {
		// no capture type defined for this type of node, ex: ovsport
		c, ok := common.CaptureTypes[n.Metadata()["Type"].(string)]
		if !ok {
			return nil, nil
		}
		capType = c.Default
	}
	probe := o.Probes.GetProbe(capType)
	if probe == nil {
		return nil, fmt.Errorf("Unable to find probe for this capture type: %v", capType)
	}

	fprobe := probe.(FlowProbe)
	return &fprobe, nil
}

func (o *OnDemandProbeListener) registerProbe(n *graph.Node, capture *api.Capture) bool {
	o.Lock()
	defer o.Unlock()

	if _, ok := o.activeProbes[n.ID]; ok {
		logging.GetLogger().Debugf("A probe already exists for %s", n.ID)
		return false
	}

	if _, ok := n.Metadata()["Type"]; !ok {
		logging.GetLogger().Infof("Unable to register flow probe type of node unknown %v", n)
		return false
	}

	fprobe, err := o.getProbe(n, capture)
	if fprobe == nil {
		if err != nil {
			logging.GetLogger().Error(err.Error())
		}
		return false
	}

	ft := o.fta.Alloc(fprobe.AsyncFlowPipeline)
	if err := fprobe.RegisterProbe(n, capture, ft); err != nil {
		logging.GetLogger().Debugf("Failed to register flow probe: %s", err.Error())
		o.fta.Release(ft)
		return false
	}

	o.activeProbes[n.ID] = ft
	o.captures[n.ID] = capture

	logging.GetLogger().Debugf("New active probe on: %v", n)
	return true
}

func (o *OnDemandProbeListener) unregisterProbe(n *graph.Node) bool {
	if !o.isActive(n) {
		return false
	}

	o.Lock()
	c := o.captures[n.ID]
	o.Unlock()
	fprobe, err := o.getProbe(n, c)
	if fprobe == nil {
		if err != nil {
			logging.GetLogger().Error(err.Error())
		}
		return false
	}

	if err := fprobe.UnregisterProbe(n); err != nil {
		logging.GetLogger().Debugf("Failed to unregister flow probe: %s", err.Error())
	}

	o.Lock()
	o.fta.Release(o.activeProbes[n.ID])
	delete(o.activeProbes, n.ID)
	delete(o.captures, n.ID)
	o.Unlock()

	return true
}

func (o *OnDemandProbeListener) matchGremlinExpr(node *graph.Node, gremlin string) bool {
	tr := traversal.NewGremlinTraversalParser(strings.NewReader(gremlin), o.Graph)
	ts, err := tr.Parse()
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

func (o *OnDemandProbeListener) onNodeEvent(n *graph.Node) {
	if o.isActive(n) {
		return
	}

	resources := o.CaptureHandler.List()
	for _, resource := range resources {
		capture := resource.(*api.Capture)

		if o.matchGremlinExpr(n, capture.GremlinQuery) {
			if o.registerProbe(n, capture) {
				t := o.Graph.StartMetadataTransaction(n)
				t.AddMetadata("State/FlowCapture", "ON")
				t.AddMetadata("CaptureID", capture.UUID)
				t.Commit()
			}
		}
	}
}

func (o *OnDemandProbeListener) OnNodeAdded(n *graph.Node) {
	o.onNodeEvent(n)
}

func (o *OnDemandProbeListener) OnNodeUpdated(n *graph.Node) {
	o.onNodeEvent(n)
}

func (o *OnDemandProbeListener) OnEdgeAdded(e *graph.Edge) {
	parent, child := o.Graph.GetEdgeNodes(e)
	if parent == nil || child == nil || e.Metadata()["RelationType"] != "ownership" {
		return
	}

	o.onNodeEvent(child)
}

func (o *OnDemandProbeListener) OnNodeDeleted(n *graph.Node) {
	if !o.isActive(n) {
		return
	}

	if o.unregisterProbe(n) {
		metadata := n.Metadata()
		metadata["State/FlowCapture"] = "OFF"
		delete(metadata, "CaptureID")
		o.Graph.SetMetadata(n, metadata)
	}
}

func (o *OnDemandProbeListener) onCaptureAdded(capture *api.Capture) {
	o.Graph.Lock()
	defer o.Graph.Unlock()

	tr := traversal.NewGremlinTraversalParser(strings.NewReader(capture.GremlinQuery), o.Graph)
	ts, err := tr.Parse()
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
		switch value.(type) {
		case *graph.Node:
			if o.registerProbe(value.(*graph.Node), capture) {
				t := o.Graph.StartMetadataTransaction(value.(*graph.Node))
				t.AddMetadata("State/FlowCapture", "ON")
				t.AddMetadata("CaptureID", capture.UUID)
				t.Commit()
			}
		case []*graph.Node:
			for _, node := range value.([]*graph.Node) {
				if o.registerProbe(node, capture) {
					t := o.Graph.StartMetadataTransaction(node)
					t.AddMetadata("State/FlowCapture", "ON")
					t.AddMetadata("CaptureID", capture.UUID)
					t.Commit()
				}
			}
		default:
			logging.GetLogger().Error("Gremlin expression doesn't return node")
			return
		}
	}
}

func (o *OnDemandProbeListener) onCaptureDeleted(capture *api.Capture) {
	o.Graph.Lock()
	defer o.Graph.Unlock()

	tr := traversal.NewGremlinTraversalParser(strings.NewReader(capture.GremlinQuery), o.Graph)
	ts, err := tr.Parse()
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
		switch value.(type) {
		case *graph.Node:
			if o.unregisterProbe(value.(*graph.Node)) {
				metadata := value.(*graph.Node).Metadata()
				metadata["State/FlowCapture"] = "OFF"
				delete(metadata, "CaptureID")
				o.Graph.SetMetadata(value.(*graph.Node), metadata)
			}
		case []*graph.Node:
			for _, node := range value.([]*graph.Node) {
				if o.unregisterProbe(node) {
					metadata := node.Metadata()
					metadata["State/FlowCapture"] = "OFF"
					delete(metadata, "CaptureID")
					o.Graph.SetMetadata(node, metadata)
				}
			}
		default:
			logging.GetLogger().Error("Gremlin expression doesn't return node")
			return
		}
	}
}

func (o *OnDemandProbeListener) onApiWatcherEvent(action string, id string, resource api.ApiResource) {
	logging.GetLogger().Debugf("New watcher event %s for %s", action, id)
	capture := resource.(*api.Capture)
	switch action {
	case "init", "create", "set", "update":
		o.onCaptureAdded(capture)
	case "expire", "delete":
		o.onCaptureDeleted(capture)
	}
}

func (o *OnDemandProbeListener) Start() error {
	o.watcher = o.CaptureHandler.AsyncWatch(o.onApiWatcherEvent)

	o.Graph.AddEventListener(o)

	return nil
}

func (o *OnDemandProbeListener) Stop() {
	o.watcher.Stop()
}

func NewOnDemandProbeListener(fb *FlowProbeBundle, g *graph.Graph, ch *api.CaptureApiHandler) (*OnDemandProbeListener, error) {
	return &OnDemandProbeListener{
		Graph:          g,
		Probes:         fb,
		CaptureHandler: ch,
		fta:            fb.FlowTableAllocator,
		activeProbes:   make(map[graph.Identifier]*flow.Table),
		captures:       make(map[graph.Identifier]*api.Capture),
	}, nil
}
