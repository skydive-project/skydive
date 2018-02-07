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
	"fmt"
	"sync"

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"

	api "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type podProbe struct {
	sync.RWMutex
	defaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*kubeCache
	graph            *graph.Graph
	containerIndexer *graph.MetadataIndexer
	nodeIndexer      *graph.MetadataIndexer
}

func newPodIndexerByHost(g *graph.Graph) *graph.MetadataIndexer {
	return graph.NewMetadataIndexer(g, graph.Metadata{"Type": "pod"}, nodeNameField)
}

func newPodIndexerByNamespace(g *graph.Graph) *graph.MetadataIndexer {
	return graph.NewMetadataIndexer(g, graph.Metadata{"Type": "pod"}, "Namespace")
}

func newPodIndexerByName(g *graph.Graph) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Type", "pod"),
		filters.NewNotFilter(filters.NewNullFilter("Namespace")),
		filters.NewNotFilter(filters.NewNullFilter("Name")))
	m := graph.NewGraphElementFilter(filter)
	return graph.NewMetadataIndexer(g, m, "Namespace", "Name")
}

func podUID(pod *api.Pod) graph.Identifier {
	return graph.Identifier(pod.GetUID())
}

func dumpPod2(namespace, name string) string {
	return fmt.Sprintf("pod{Namespace: %s, Name: %s}", namespace, name)
}

func dumpPod(pod *api.Pod) string {
	return dumpPod2(pod.GetNamespace(), pod.GetName())
}

func (p *podProbe) newMetadata(pod *api.Pod) graph.Metadata {
	return newMetadata("pod", pod.GetNamespace(), pod.GetName(), pod)
}

func (p *podProbe) linkPodToNode(pod *api.Pod, podNode *graph.Node) {
	nodeNodes := p.nodeIndexer.Get(pod.Spec.NodeName)
	if len(nodeNodes) == 0 {
		return
	}
	linkPodToNode(p.graph, nodeNodes[0], podNode)
}

func (p *podProbe) onAdd(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		return
	}

	podNode := newNode(p.graph, podUID(pod), p.newMetadata(pod))

	containerNodes := p.containerIndexer.Get(pod.Namespace, pod.Name)
	for _, containerNode := range containerNodes {
		addOwnershipLink(p.graph, podNode, containerNode)
	}

	p.linkPodToNode(pod, podNode)
}

func (p *podProbe) OnAdd(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		return
	}

	p.Lock()
	defer p.Unlock()

	p.graph.Lock()
	defer p.graph.Unlock()

	logging.GetLogger().Debugf("Creating node for %s", dumpPod(pod))

	p.onAdd(obj)
}

func (p *podProbe) OnUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*api.Pod)
	newPod := newObj.(*api.Pod)

	p.Lock()
	defer p.Unlock()

	p.graph.Lock()
	defer p.graph.Unlock()

	podNode := p.graph.GetNode(podUID(newPod))
	if podNode == nil {
		logging.GetLogger().Debugf("Updating (re-adding) node for %s", dumpPod(newPod))
		p.onAdd(newObj)
		return
	}

	logging.GetLogger().Debugf("Updating node for %s", dumpPod(newPod))
	if oldPod.Spec.NodeName == "" && newPod.Spec.NodeName != "" {
		p.linkPodToNode(newPod, podNode)
	}

	addMetadata(p.graph, podNode, newPod)
}

func (p *podProbe) OnDelete(obj interface{}) {
	if pod, ok := obj.(*api.Pod); ok {
		logging.GetLogger().Debugf("Deleting node for %s", dumpPod(pod))
		p.graph.Lock()
		if podNode := p.graph.GetNode(podUID(pod)); podNode != nil {
			p.graph.DelNode(podNode)
		}
		p.graph.Unlock()
	}
}

func linkPodsToNode(g *graph.Graph, host *graph.Node, pods []*graph.Node) {
	for _, pod := range pods {
		linkPodToNode(g, host, pod)
	}
}

func linkPodToNode(g *graph.Graph, node, pod *graph.Node) {
	addOwnershipLink(g, node, pod)
}

func (p *podProbe) Start() {
	p.containerIndexer.AddEventListener(p)
	p.nodeIndexer.AddEventListener(p)
	p.kubeCache.Start()
}

func (p *podProbe) Stop() {
	p.containerIndexer.RemoveEventListener(p)
	p.nodeIndexer.RemoveEventListener(p)
	p.kubeCache.Stop()
}

func newPodKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().Core().RESTClient(), &api.Pod{}, "pods", handler)
}

func newPodProbe(g *graph.Graph) *podProbe {
	p := &podProbe{
		graph:            g,
		containerIndexer: newContainerIndexer(g),
		nodeIndexer:      newNodeIndexer(g),
	}
	p.kubeCache = newPodKubeCache(p)
	return p
}
