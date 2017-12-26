/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"sync"

	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
)

type nodeCache struct {
	sync.RWMutex
	defaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*kubeCache
	graph       *graph.Graph
	hostIndexer *graph.MetadataIndexer
	podIndexer  *graph.MetadataIndexer
}

func newHostIndexer(g *graph.Graph) *graph.MetadataIndexer {
	return graph.NewMetadataIndexer(g, graph.Metadata{"Type": "host"}, "Name")
}

func (c *nodeCache) newMetadata(node *v1.Node) graph.Metadata {
	return newMetadata("host", node.GetName(), node)
}

func (c *nodeCache) onAdd(obj interface{}) {
	host := obj.(*v1.Node)

	c.Lock()
	defer c.Unlock()

	c.graph.Lock()
	defer c.graph.Unlock()

	hostName := host.GetName()
	hostNodes := c.hostIndexer.Get(hostName)
	var hostNode *graph.Node
	if len(hostNodes) == 0 {
		hostNode = c.graph.NewNode(graph.Identifier(host.GetUID()), c.newMetadata(host))
	} else {
		hostNode = hostNodes[0]
		addMetadata(c.graph, hostNode, host)
	}

	linkPodsToHost(c.graph, hostNode, c.podIndexer.Get(hostName))
}

func (c *nodeCache) OnAdd(obj interface{}) {
	c.onAdd(obj)
}

func (c *nodeCache) OnUpdate(oldObj, newObj interface{}) {
	c.onAdd(newObj)
}

func (c *nodeCache) OnDelete(obj interface{}) {
	if node, ok := obj.(*v1.Node); ok {
		c.graph.Lock()
		if nodeNode := c.graph.GetNode(graph.Identifier(node.GetUID())); nodeNode != nil {
			c.graph.DelNode(nodeNode)
		}
		c.graph.Unlock()
	}
}

func (c *nodeCache) Start() {
	c.kubeCache.Start()
	c.hostIndexer.AddEventListener(c)
}

func (c *nodeCache) Stop() {
	c.kubeCache.Stop()
	c.hostIndexer.RemoveEventListener(c)
}

func newNodeCache(client *kubeClient, g *graph.Graph) *nodeCache {
	c := &nodeCache{
		graph:       g,
		hostIndexer: newHostIndexer(g),
		podIndexer:  newPodIndexerByHost(g),
	}
	c.kubeCache = client.getCacheFor(client.Core().RESTClient(), &v1.Node{}, "nodes", c)
	return c
}
