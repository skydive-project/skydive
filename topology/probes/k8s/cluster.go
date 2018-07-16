/*
 * Copyright (C) 2018 IBM, Inc.
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
	"fmt"

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

const clusterName = "cluster"

type clusterProbe struct {
	graph.DefaultGraphListener
	graph          *graph.Graph
	clusterIndexer *graph.MetadataIndexer
	objectIndexer  *graph.MetadataIndexer
}

func newClusterLinkedObjectIndexer(g *graph.Graph) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", managerValue),
		filters.NewOrFilter(
			filters.NewTermStringFilter("Type", "namespace"),
			filters.NewTermStringFilter("Type", "networkpolicy"),
			filters.NewTermStringFilter("Type", "node"),
			filters.NewTermStringFilter("Type", "persistentvolume"),
			filters.NewTermStringFilter("Type", "persistentvolumeclaim"),
		),
	)
	m := graph.NewGraphElementFilter(filter)
	return graph.NewMetadataIndexer(g, m)
}

func newClusterIndexer(g *graph.Graph) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", managerValue),
		filters.NewTermStringFilter("Type", "cluster"),
		filters.NewNotNullFilter("Name"),
	)
	m := graph.NewGraphElementFilter(filter)
	return graph.NewMetadataIndexer(g, m, "Name")
}

func dumpCluster(name string) string {
	return fmt.Sprintf("cluster{'Name': %s}", name)
}

func (p *clusterProbe) newMetadata(name string) graph.Metadata {
	return newMetadata("cluster", "", name, nil)
}

func (p *clusterProbe) linkObject(objNode, clusterNode *graph.Node) {
	addOwnershipLink(p.graph, clusterNode, objNode)
}

func (p *clusterProbe) addNode(name string) {
	p.graph.Lock()
	defer p.graph.Unlock()

	clusterNode := newNode(p.graph, graph.GenID(), p.newMetadata(name))
	objNodes, _ := p.objectIndexer.Get()
	for _, objNode := range objNodes {
		p.linkObject(objNode, clusterNode)
	}

	logging.GetLogger().Debugf("Added %s", dumpCluster(name))
}

func (p *clusterProbe) delNode(name string) {
	p.graph.Lock()
	defer p.graph.Unlock()

	clusterNodes, _ := p.clusterIndexer.Get(name)
	for _, clusterNode := range clusterNodes {
		p.graph.DelNode(clusterNode)
	}

	logging.GetLogger().Debugf("Deleted %s", dumpCluster(name))
}

func (p *clusterProbe) OnNodeAdded(objNode *graph.Node) {
	logging.GetLogger().Debugf("Got event on adding %s", dumpGraphNode(objNode))
	clusterNodes, _ := p.clusterIndexer.Get(clusterName)
	if len(clusterNodes) > 0 {
		p.linkObject(objNode, clusterNodes[0])
	}
}

func (p *clusterProbe) OnNodeUpdated(objNode *graph.Node) {
	logging.GetLogger().Debugf("Got event on updating %s", dumpGraphNode(objNode))
	clusterNodes, _ := p.clusterIndexer.Get(clusterName)
	if len(clusterNodes) > 0 {
		p.linkObject(objNode, clusterNodes[0])
	}
}

func (p *clusterProbe) Start() {
	p.clusterIndexer.Start()
	p.objectIndexer.AddEventListener(p)
	p.objectIndexer.Start()
	p.addNode(clusterName)
}

func (p *clusterProbe) Stop() {
	p.delNode(clusterName)
	p.clusterIndexer.Stop()
	p.objectIndexer.RemoveEventListener(p)
	p.objectIndexer.Stop()
}

func newClusterProbe(g *graph.Graph) probe.Probe {
	p := &clusterProbe{
		graph:          g,
		clusterIndexer: newClusterIndexer(g),
		objectIndexer:  newClusterLinkedObjectIndexer(g),
	}
	return p
}
