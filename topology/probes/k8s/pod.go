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

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type podProbe struct {
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
		filters.NewNotNullFilter("Namespace"),
		filters.NewNotNullFilter("Name"))
	m := graph.NewGraphElementFilter(filter)
	return graph.NewMetadataIndexer(g, m, "Namespace", "Name")
}

func podUID(pod *v1.Pod) graph.Identifier {
	return graph.Identifier(pod.GetUID())
}

func dumpPod2(namespace, name string) string {
	return fmt.Sprintf("pod{Namespace: %s, Name: %s}", namespace, name)
}

func dumpPod(pod *v1.Pod) string {
	return dumpPod2(pod.GetNamespace(), pod.GetName())
}

func (p *podProbe) newMetadata(pod *v1.Pod) graph.Metadata {
	m := newMetadata("pod", pod.Namespace, pod.Name, pod)

	podIP := pod.Status.PodIP
	if podIP != "" {
		m.SetField("IP", podIP)
	}

	m.SetField("Node", pod.Spec.NodeName)

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}
	m.SetField("Status", reason)

	return m
}

func (p *podProbe) linkPodToNode(pod *v1.Pod, podNode *graph.Node) {
	nodeNodes, _ := p.nodeIndexer.Get(pod.Spec.NodeName)
	if len(nodeNodes) == 0 {
		return
	}
	linkPodToNode(p.graph, nodeNodes[0], podNode)
}

func (p *podProbe) onAdd(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	podNode := newNode(p.graph, podUID(pod), p.newMetadata(pod))

	containerNodes, _ := p.containerIndexer.Get(pod.Namespace, pod.Name)
	for _, containerNode := range containerNodes {
		addOwnershipLink(p.graph, podNode, containerNode)
	}

	p.linkPodToNode(pod, podNode)
}

func (p *podProbe) OnAdd(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	p.graph.Lock()
	defer p.graph.Unlock()

	p.onAdd(obj)
	logging.GetLogger().Debugf("Added %s", dumpPod(pod))
}

func (p *podProbe) OnUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*v1.Pod)
	newPod := newObj.(*v1.Pod)

	p.graph.Lock()
	defer p.graph.Unlock()

	podNode := p.graph.GetNode(podUID(newPod))
	if podNode == nil {
		logging.GetLogger().Debugf("Updating (re-adding) node for %s", dumpPod(newPod))
		p.onAdd(newObj)
		return
	}

	if oldPod.Spec.NodeName == "" && newPod.Spec.NodeName != "" {
		p.linkPodToNode(newPod, podNode)
	}

	addMetadata(p.graph, podNode, newPod)
	logging.GetLogger().Debugf("Updated %s", dumpPod(newPod))
}

func (p *podProbe) OnDelete(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		p.graph.Lock()
		if podNode := p.graph.GetNode(podUID(pod)); podNode != nil {
			p.graph.DelNode(podNode)
		}
		p.graph.Unlock()
		logging.GetLogger().Debugf("Deleted %s", dumpPod(pod))
	}
}

func linkPodsToNode(g *graph.Graph, host *graph.Node, pods []*graph.Node) {
	for _, pod := range pods {
		linkPodToNode(g, host, pod)
	}
}

func linkPodToNode(g *graph.Graph, node, pod *graph.Node) {
	addLink(g, node, pod)
}

func (p *podProbe) Start() {
	p.containerIndexer.AddEventListener(p)
	p.containerIndexer.Start()
	p.nodeIndexer.AddEventListener(p)
	p.nodeIndexer.Start()
	p.kubeCache.Start()
}

func (p *podProbe) Stop() {
	p.containerIndexer.RemoveEventListener(p)
	p.containerIndexer.Stop()
	p.nodeIndexer.RemoveEventListener(p)
	p.nodeIndexer.Stop()
	p.kubeCache.Stop()
}

func newPodKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().Core().RESTClient(), &v1.Pod{}, "pods", handler)
}

func newPodProbe(g *graph.Graph) probe.Probe {
	p := &podProbe{
		graph:            g,
		containerIndexer: newContainerIndexer(g),
		nodeIndexer:      newNodeIndexer(g),
	}
	p.kubeCache = newPodKubeCache(p)
	return p
}
