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
	graph *graph.Graph
}

func (c *nodeCache) getMetadata(node *v1.Node) graph.Metadata {
	return graph.Metadata{
		"Type":    "host",
		"Manager": "k8s",
		"Name":    node.GetName(),
		"K8s":     node,
	}
}

func (c *nodeCache) OnAdd(obj interface{}) {
	c.OnUpdate(obj, obj)
}

func (c *nodeCache) OnUpdate(old, new interface{}) {
	newNode := new.(*v1.Node)

	c.Lock()
	defer c.Unlock()

	c.graph.Lock()
	defer c.graph.Unlock()

	graphMeta := c.getMetadata(newNode)
	hostIndexer := graph.NewMetadataIndexer(c.graph, graph.Metadata{"Type": "host"}, "Name")
	hostNodes := hostIndexer.Get(newNode.GetName())
	if len(hostNodes) == 0 {
		c.graph.NewNode(graph.Identifier(newNode.GetUID()), graphMeta)
	} else {
		if val, ok := graphMeta["Manager"]; ok && val == "k8s" {
			c.graph.SetMetadata(hostNodes[0], graphMeta)
		}
	}
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
}

func (c *nodeCache) Stop() {
	c.kubeCache.Stop()
}

func newNodeCache(client *kubeClient, g *graph.Graph) *nodeCache {
	c := &nodeCache{
		graph: g,
	}
	c.kubeCache = client.getCacheFor(client.Core().RESTClient(), &v1.Node{}, "nodes", c)
	return c
}
