/*
 * Copyright 2018 IBM Corp.
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
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

// ClusterName is the name of the k8s cluster
const ClusterName = "cluster"

var clusterEventHandler = graph.NewEventHandler(100)

var clusterNode *graph.Node

type clusterCache struct {
	*graph.EventHandler
	graph *graph.Graph
}

func (c *clusterCache) addClusterNode() {
	c.graph.Lock()
	defer c.graph.Unlock()

	m := graph.Metadata{"Name": ClusterName}
	clusterNode = c.graph.NewNode(graph.GenID(), NewMetadata(Manager, "cluster", m, nil, ClusterName), "")
	c.NotifyEvent(graph.NodeAdded, clusterNode)
	logging.GetLogger().Debugf("Added cluster{Name: %s}", ClusterName)
}

func (c *clusterCache) Start() {
}

func (c *clusterCache) Stop() {
}

func newClusterProbe(clientset interface{}, g *graph.Graph) Subprobe {
	c := &clusterCache{
		EventHandler: clusterEventHandler,
		graph:        g,
	}
	c.addClusterNode()
	return c
}

type clusterLinker struct {
	graph.DefaultLinker
	*graph.ResourceLinker
	g             *graph.Graph
	objectIndexer *graph.MetadataIndexer
}

func (linker *clusterLinker) createEdge(cluster, object *graph.Node) *graph.Edge {
	id := graph.GenID(string(cluster.ID), string(object.ID))
	return linker.g.CreateEdge(id, cluster, object, topology.OwnershipMetadata(), graph.TimeUTC(), "")
}

// GetBALinks returns all the incoming links for a node
func (linker *clusterLinker) GetBALinks(objectNode *graph.Node) (edges []*graph.Edge) {
	edges = append(edges, linker.createEdge(clusterNode, objectNode))
	return
}

func newClusterLinker(g *graph.Graph, manager string, types ...string) probe.Probe {
	objectIndexer := newObjectIndexerFromFilter(g, g, newTypesFilter(manager, types...))
	objectIndexer.Start()

	return graph.NewResourceLinker(
		g,
		nil,
		objectIndexer,
		&clusterLinker{g: g},
		topology.OwnershipMetadata(),
	)
}
