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

	"k8s.io/client-go/kubernetes"
)

// ClusterName is the name of the k8s cluster
const ClusterName = "cluster"

var clusterEventHandler = graph.NewEventHandler(100)

type clusterCache struct {
	*graph.EventHandler
	graph *graph.Graph
}

func (c *clusterCache) Start() {
	c.graph.Lock()
	defer c.graph.Unlock()

	m := NewMetadata(Manager, "cluster", nil, ClusterName)
	node := c.graph.NewNode(graph.GenID(), m, "")
	c.NotifyEvent(graph.NodeAdded, node)
	logging.GetLogger().Debugf("Added cluster{Name: %s}", ClusterName)
}

func (c *clusterCache) Stop() {
}

func newClusterProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return &clusterCache{
		EventHandler: clusterEventHandler,
		graph:        g,
	}
}

func newClusterLinker(g *graph.Graph, manager string, types ...string) probe.Probe {
	clusterIndexer := graph.NewMetadataIndexer(g, clusterEventHandler, graph.Metadata{"Manager": Manager, "Type": "cluster"})
	clusterIndexer.Start()

	objectFilter := newTypesFilter(manager, types...)
	objectIndexer := newObjectIndexerFromFilter(g, g, objectFilter)
	objectIndexer.Start()

	return graph.NewMetadataIndexerLinker(g, clusterIndexer, objectIndexer, topology.OwnershipMetadata())
}
