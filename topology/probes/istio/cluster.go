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

package istio

import (
	"fmt"

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/k8s"
)

const ClusterName = "cluster"

type clusterProbe struct {
	graph.DefaultGraphListener
	graph          *graph.Graph
	clusterIndexer *graph.MetadataIndexer
}

func dumpCluster(name string) string {
	return fmt.Sprintf("cluster{'Name': %s}", name)
}

func (p *clusterProbe) newMetadata(name string) graph.Metadata {
	return k8s.NewMetadata(Manager, "cluster", "", name, nil)
}

func (p *clusterProbe) addNode(name string) {
	p.graph.Lock()
	defer p.graph.Unlock()

	k8s.NewNode(p.graph, graph.GenID(), p.newMetadata(name))
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

func (p *clusterProbe) Start() {
	p.clusterIndexer.Start()
	p.addNode(ClusterName)
}

func (p *clusterProbe) Stop() {
	p.delNode(ClusterName)
	p.clusterIndexer.Stop()
}

func newClusterProbe(g *graph.Graph) probe.Probe {
	p := &clusterProbe{
		graph:          g,
		clusterIndexer: k8s.NewObjectIndexerByName(Manager, g, "cluster"),
	}
	return p
}
