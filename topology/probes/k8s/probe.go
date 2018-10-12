/*
 * Copyright 2017 IBM Corp.
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

package k8s

import (
	"time"

	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

func int32ValueOrDefault(value *int32, defaultValue int32) int32 {
	if value == nil {
		return defaultValue
	}
	return *value
}

// Probe for tracking k8s events
type Probe struct {
	manager   string
	subprobes map[string]Subprobe
	linkers   []probe.Probe
}

// Subprobe describes a probe for a specific Kubernetes resource
// It must implement the ListenerHandler interface so that you
// listen for creation/update/removal of a resource
type Subprobe interface {
	probe.Probe
	graph.ListenerHandler
}

// Start k8s probe
func (p *Probe) Start() {
	for _, linker := range p.linkers {
		linker.Start()
	}

	for _, subprobe := range p.subprobes {
		subprobe.Start()
	}
}

// Stop k8s probe
func (p *Probe) Stop() {
	for _, linker := range p.linkers {
		linker.Stop()
	}

	for _, subprobe := range p.subprobes {
		subprobe.Stop()
	}
}

// NewProbe creates the probe for tracking k8s events
func NewProbe(manager, clusterName string, g *graph.Graph, subprobes map[string]Subprobe, linkers []probe.Probe) (*Probe, error) {
	clusterNode := g.NewNode(graph.GenID(), NewMetadata(manager, "cluster", nil, clusterName), "")
	for _, subprobe := range subprobes {
		if cache, ok := subprobe.(*ResourceCache); ok {
			if cache.handler.IsTopLevel() {
				if clusterLinker := newClusterLinker(g, clusterNode, cache); clusterLinker != nil {
					linkers = append(linkers, clusterLinker)
				}
			}
		}
	}

	return &Probe{
		subprobes: subprobes,
		linkers:   linkers,
	}, nil
}

type clusterLinker struct {
	graph.DefaultLinker
	graph       *graph.Graph
	clusterNode *graph.Node
}

func (cl *clusterLinker) GetBALinks(node *graph.Node) []*graph.Edge {
	id := graph.GenID(string(cl.clusterNode.ID), string(node.ID), "RelationType", topology.OwnershipLink)
	return []*graph.Edge{cl.graph.CreateEdge(id, cl.clusterNode, node, topology.OwnershipMetadata(), time.Now(), "")}
}

func newClusterLinker(g *graph.Graph, clusterNode *graph.Node, cache *ResourceCache) *graph.ResourceLinker {
	linker := &clusterLinker{graph: g, clusterNode: clusterNode}
	return graph.NewResourceLinker(g, nil, cache, linker, topology.OwnershipMetadata())
}
