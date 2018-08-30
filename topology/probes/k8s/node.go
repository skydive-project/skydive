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
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type nodeProbe struct {
	DefaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*KubeCache
	graph       *graph.Graph
	nodeIndexer *graph.MetadataIndexer
	hostIndexer *graph.MetadataIndexer
	podIndexer  *graph.MetadataIndexer
}

func newNodeIndexer(g *graph.Graph) *graph.MetadataIndexer {
	return graph.NewMetadataIndexer(g, graph.Metadata{"Type": "node"}, "Hostname")
}

func newHostIndexer(g *graph.Graph) *graph.MetadataIndexer {
	return graph.NewMetadataIndexer(g, graph.Metadata{"Type": "host"}, "Hostname")
}

func (c *nodeProbe) newMetadata(node *v1.Node) graph.Metadata {
	m := NewMetadata(Manager, "node", node.Namespace, node.Name, node)
	m.SetFieldAndNormalize("Labels", node.Labels)
	m.SetField("Cluster", node.ClusterName)
	for _, a := range node.Status.Addresses {
		switch a.Type {
		case "Hostname", "InternalIP", "ExternalIP":
			m.SetField(string(a.Type), a.Address)
		}
	}
	info := node.Status.NodeInfo
	m.SetField("Arch", info.Architecture)
	m.SetField("Kernel", info.KernelVersion)
	m.SetField("OS", info.OperatingSystem)
	return m
}

func linkNodeToHost(g *graph.Graph, host, node *graph.Node) {
	AddLinkTry(g, host, node, NewEdgeMetadata(Manager))
}

func nodeUID(node *v1.Node) graph.Identifier {
	return graph.Identifier(node.GetUID())
}

func (c *nodeProbe) onAdd(obj interface{}) {
	node := obj.(*v1.Node)

	c.graph.Lock()
	defer c.graph.Unlock()

	hostName := node.GetName()
	nodeNodes, _ := c.nodeIndexer.Get(hostName)
	var nodeNode *graph.Node
	if len(nodeNodes) == 0 {
		nodeNode = NewNode(c.graph, nodeUID(node), c.newMetadata(node))
	} else {
		nodeNode = nodeNodes[0]
		AddMetadata(c.graph, nodeNode, node)
	}

	podNodes, _ := c.podIndexer.Get(hostName)
	linkPodsToNode(c.graph, nodeNode, podNodes)

	hostNodes, _ := c.hostIndexer.Get(hostName)
	if len(hostNodes) != 0 {
		linkNodeToHost(c.graph, hostNodes[0], nodeNode)
	}
}

func (c *nodeProbe) OnAdd(obj interface{}) {
	c.onAdd(obj)
}

func (c *nodeProbe) OnUpdate(oldObj, newObj interface{}) {
	c.onAdd(newObj)
}

func (c *nodeProbe) OnDelete(obj interface{}) {
	if node, ok := obj.(*v1.Node); ok {
		c.graph.Lock()
		if nodeNode := c.graph.GetNode(nodeUID(node)); nodeNode != nil {
			c.graph.DelNode(nodeNode)
		}
		c.graph.Unlock()
	}
}

func (c *nodeProbe) OnNodeAdded(n *graph.Node) {
	if hostname, _ := n.GetFieldString("Hostname"); hostname != "" {
		// A kubernetes node already exists
		if nodes, _ := c.nodeIndexer.Get(hostname); len(nodes) > 0 {
			linkNodeToHost(c.graph, n, nodes[0])
		}
	}
}

func (c *nodeProbe) Start() {
	c.KubeCache.Start()
	c.nodeIndexer.Start()
	c.hostIndexer.AddEventListener(c)
	c.hostIndexer.Start()
	c.podIndexer.Start()
}

func (c *nodeProbe) Stop() {
	c.KubeCache.Stop()
	c.nodeIndexer.Stop()
	c.hostIndexer.Stop()
	c.podIndexer.Stop()
}

func newNodeKubeCache(handler cache.ResourceEventHandler) *KubeCache {
	return NewKubeCache(getClientset().Core().RESTClient(), &v1.Node{}, "nodes", handler)
}

func newNodeProbe(g *graph.Graph) probe.Probe {
	c := &nodeProbe{
		graph:       g,
		hostIndexer: newHostIndexer(g),
		nodeIndexer: newNodeIndexer(g),
		podIndexer:  newPodIndexerByHost(g),
	}
	c.KubeCache = newNodeKubeCache(c)
	return c
}
