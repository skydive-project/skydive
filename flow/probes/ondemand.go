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
	"os"
	"strings"

	"github.com/redhat-cip/skydive/api"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
)

type OnDemandProbeListener struct {
	graph.DefaultGraphListener
	Graph          *graph.Graph
	Probes         *FlowProbeBundle
	CaptureHandler api.ApiHandler
	watcher        api.StoppableWatcher
	host           string
}

type FlowProbe interface {
	RegisterProbe(n *graph.Node, capture *api.Capture) error
	UnregisterProbe(n *graph.Node) error
}

func (o *OnDemandProbeListener) probeFromType(n *graph.Node) FlowProbe {
	var probeName string

	switch n.Metadata()["Type"] {
	case "ovsbridge":
		probeName = "ovssflow"
	default:
		probeName = "pcap"
	}

	probe := o.Probes.GetProbe(probeName)
	if probe == nil {
		return nil
	}

	return probe.(FlowProbe)
}

func (o *OnDemandProbeListener) registerProbe(n *graph.Node, capture *api.Capture) {
	fprobe := o.probeFromType(n)
	if fprobe == nil {
		logging.GetLogger().Errorf("Failed to register flow probe, unknown type")
		return
	}

	if err := fprobe.RegisterProbe(n, capture); err != nil {
		logging.GetLogger().Debugf("Failed to register flow probe: %s", err.Error())
	}
}

func (o *OnDemandProbeListener) unregisterProbe(n *graph.Node) {
	fprobe := o.probeFromType(n)
	if fprobe == nil {
		return
	}

	if err := fprobe.UnregisterProbe(n); err != nil {
		logging.GetLogger().Debugf("Failed to unregister flow probe: %s", err.Error())
	}
}

func (o *OnDemandProbeListener) OnNodeAdded(n *graph.Node) {
	nodes := o.Graph.LookupShortestPath(n, graph.Metadata{"Type": "host"}, topology.IsOwnershipEdge)
	if len(nodes) == 0 {
		return
	}

	path := topology.NodePath{nodes}.Marshal()

	var capture api.ApiResource
	var ok bool
	if capture, ok = o.CaptureHandler.Get(path); !ok {
		// try using the wildcard instead of the host
		wildcard := "*/" + topology.NodePath{nodes[:len(nodes)-1]}.Marshal()
		if capture, ok = o.CaptureHandler.Get(wildcard); !ok {
			return
		}
	}

	o.registerProbe(n, capture.(*api.Capture))
}

func (o *OnDemandProbeListener) OnNodeUpdated(n *graph.Node) {
	o.OnNodeAdded(n)
}

func (o *OnDemandProbeListener) OnEdgeAdded(e *graph.Edge) {
	parent, child := o.Graph.GetEdgeNodes(e)
	if parent == nil || child == nil {
		return
	}

	if parent.Metadata()["Type"] == "ovsbridge" {
		o.OnNodeAdded(parent)
		return
	}

	if child.Metadata()["Type"] == "ovsbridge" {
		o.OnNodeAdded(child)
		return
	}
}

func (o *OnDemandProbeListener) OnNodeDeleted(n *graph.Node) {
	o.unregisterProbe(n)
}

func (o *OnDemandProbeListener) onCaptureAdded(probePath string, capture *api.Capture) {
	o.Graph.Lock()
	defer o.Graph.Unlock()

	if node := topology.LookupNodeFromNodePathString(o.Graph, probePath); node != nil {
		o.registerProbe(node, capture)
	}
}

func (o *OnDemandProbeListener) onCaptureDeleted(probePath string) {
	o.Graph.Lock()
	defer o.Graph.Unlock()

	if node := topology.LookupNodeFromNodePathString(o.Graph, probePath); node != nil {
		o.unregisterProbe(node)
	}
}

func (o *OnDemandProbeListener) probePathFromID(id string) string {
	return strings.Replace(id, "*", o.host+"[Type=host]", 1)
}

func (o *OnDemandProbeListener) onApiWatcherEvent(action string, id string, resource api.ApiResource) {
	logging.GetLogger().Debugf("New watcher event %s for %s", action, id)
	capture := resource.(*api.Capture)
	switch action {
	case "init", "create", "set", "update":
		o.onCaptureAdded(o.probePathFromID(id), capture)
	case "expire", "delete":
		o.onCaptureDeleted(o.probePathFromID(id))
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

func NewOnDemandProbeListener(fb *FlowProbeBundle, g *graph.Graph, ch api.ApiHandler) (*OnDemandProbeListener, error) {
	h, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &OnDemandProbeListener{
		Graph:          g,
		Probes:         fb,
		CaptureHandler: ch,
		host:           h,
	}, nil
}
