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

	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type namespaceProbe struct {
	defaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*kubeCache
	graph            *graph.Graph
	objectIndexer    *graph.MetadataIndexer
	namespaceIndexer *graph.MetadataIndexer
}

func newObjectIndexer(g *graph.Graph) *graph.MetadataIndexer {
	ownedByNamespaceFilter := filters.NewOrFilter(
		filters.NewTermStringFilter("Type", "deployment"),
		filters.NewTermStringFilter("Type", "daemonset"),
		filters.NewTermStringFilter("Type", "ingress"),
		filters.NewTermStringFilter("Type", "job"),
		filters.NewTermStringFilter("Type", "pod"),
		filters.NewTermStringFilter("Type", "persistentvolume"),
		filters.NewTermStringFilter("Type", "persistentvolumeclaim"),
		filters.NewTermStringFilter("Type", "replicaset"),
		filters.NewTermStringFilter("Type", "replicationcontroller"),
		filters.NewTermStringFilter("Type", "service"),
		filters.NewTermStringFilter("Type", "statefulset"),
	)

	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", managerValue),
		ownedByNamespaceFilter,
	)
	m := graph.NewGraphElementFilter(filter)
	return graph.NewMetadataIndexer(g, m, "Namespace")
}

func newNamespaceIndexerByName(g *graph.Graph) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", managerValue),
		filters.NewTermStringFilter("Type", "namespace"),
		filters.NewNotNullFilter("Name"),
	)
	m := graph.NewGraphElementFilter(filter)
	return graph.NewMetadataIndexer(g, m, "Name")
}

func dumpNamespace(ns *v1.Namespace) string {
	return fmt.Sprintf("namespace{Name: %s}", ns.GetName())
}

func (p *namespaceProbe) newMetadata(ns *v1.Namespace) graph.Metadata {
	m := newMetadata("namespace", "", ns.GetName(), ns)
	m.SetField("Labels", ns.Labels)
	m.SetField("Cluster", ns.ClusterName)
	m.SetField("Status", ns.Status.Phase)
	return m
}

func namespaceUID(ns *v1.Namespace) graph.Identifier {
	return graph.Identifier(ns.GetUID())
}

func (p *namespaceProbe) linkObject(objNode, nsNode *graph.Node) {
	addOwnershipLink(p.graph, nsNode, objNode)
}

func (p *namespaceProbe) OnAdd(obj interface{}) {
	if ns, ok := obj.(*v1.Namespace); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		nsNode := newNode(p.graph, namespaceUID(ns), p.newMetadata(ns))
		logging.GetLogger().Debugf("Added %s", dumpNamespace(ns))

		objNodes, _ := p.objectIndexer.Get(ns.GetName())
		for _, objNode := range objNodes {
			p.linkObject(objNode, nsNode)
		}
	}
}

func (p *namespaceProbe) OnUpdate(oldObj, newObj interface{}) {
	if ns, ok := newObj.(*v1.Namespace); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(namespaceUID(ns)); nsNode != nil {
			addMetadata(p.graph, nsNode, ns)
			logging.GetLogger().Debugf("Updated %s", dumpNamespace(ns))
		}
	}
}

func (p *namespaceProbe) OnDelete(obj interface{}) {
	if ns, ok := obj.(*v1.Namespace); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(namespaceUID(ns)); nsNode != nil {
			p.graph.DelNode(nsNode)
			logging.GetLogger().Debugf("Deleted %s", dumpNamespace(ns))
		}
	}
}

func (p *namespaceProbe) OnNodeAdded(objNode *graph.Node) {
	logging.GetLogger().Debugf("Got event on adding %s", dumpGraphNode(objNode))
	objNamespace, _ := objNode.GetFieldString("Namespace")
	nsNodes, _ := p.namespaceIndexer.Get(objNamespace)
	if len(nsNodes) > 0 {
		p.linkObject(objNode, nsNodes[0])
	}
}

func (p *namespaceProbe) OnNodeUpdated(objNode *graph.Node) {
	logging.GetLogger().Debugf("Got event on updating %s", dumpGraphNode(objNode))
	objNamespace, _ := objNode.GetFieldString("Namespace")
	nsNodes, _ := p.namespaceIndexer.Get(objNamespace)
	if len(nsNodes) > 0 {
		p.linkObject(objNode, nsNodes[0])
	}
}

func (p *namespaceProbe) Start() {
	p.kubeCache.Start()
	p.namespaceIndexer.Start()
	p.objectIndexer.AddEventListener(p)
	p.objectIndexer.Start()
}

func (p *namespaceProbe) Stop() {
	p.kubeCache.Stop()
	p.namespaceIndexer.Stop()
	p.objectIndexer.RemoveEventListener(p)
	p.objectIndexer.Stop()
}

func newNamespaceKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().Core().RESTClient(), &v1.Namespace{}, "namespaces", handler)
}

func newNamespaceProbe(g *graph.Graph) probe.Probe {
	p := &namespaceProbe{
		graph:            g,
		objectIndexer:    newObjectIndexer(g),
		namespaceIndexer: newNamespaceIndexerByName(g),
	}
	p.kubeCache = newNamespaceKubeCache(p)
	return p
}
