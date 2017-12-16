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

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"

	api "k8s.io/api/core/v1"
)

type podCache struct {
	sync.RWMutex
	defaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*kubeCache
	graph            *graph.Graph
	containerIndexer *graph.MetadataIndexer
	hostIndexer      *graph.MetadataIndexer
	podIndexer       *graph.MetadataIndexer
}

func (p *podCache) getMetadata(pod *api.Pod) graph.Metadata {
	return graph.Metadata{
		"Type":       "pod",
		"Manager":    "k8s",
		"Name":       pod.GetName(),
		"UID":        pod.GetUID(),
		"ObjectMeta": pod.ObjectMeta,
		"Spec":       pod.Spec,
	}
}

func (p *podCache) linkPodToHost(pod *api.Pod, podNode *graph.Node) {
	hostNodes := p.hostIndexer.Get(pod.Spec.NodeName)
	if len(hostNodes) == 0 {
		return
	}
	topology.AddOwnershipLink(p.graph, hostNodes[0], podNode, nil)
}

func (p *podCache) OnAdd(obj interface{}) {
	if pod, ok := obj.(*api.Pod); ok {
		p.Lock()
		defer p.Unlock()

		p.graph.Lock()
		defer p.graph.Unlock()

		logging.GetLogger().Debugf("Creating node for pod %s", pod.GetUID())
		podNode := p.graph.NewNode(graph.Identifier(pod.GetUID()), p.getMetadata(pod))

		containerNodes := p.containerIndexer.Get(pod.Namespace, pod.Name)
		for _, containerNode := range containerNodes {
			p.graph.Link(podNode, containerNode, PodToContainerMetadata)
		}

		p.linkPodToHost(pod, podNode)
	}
}

func (p *podCache) OnUpdate(obj, new interface{}) {
	oldPod := obj.(*api.Pod)
	newPod := new.(*api.Pod)
	podNode := p.graph.GetNode(graph.Identifier(newPod.GetUID()))

	p.graph.Lock()
	defer p.graph.Unlock()

	if oldPod.Spec.NodeName == "" && newPod.Spec.NodeName != "" {
		p.linkPodToHost(newPod, podNode)
	}

	p.graph.SetMetadata(podNode, p.getMetadata(newPod))
}

func (p *podCache) OnDelete(obj interface{}) {
	if pod, ok := obj.(*api.Pod); ok {
		p.graph.Lock()
		if podNode := p.graph.GetNode(graph.Identifier(pod.GetUID())); podNode != nil {
			p.graph.DelNode(podNode)
		}
		p.graph.Unlock()
	}
}

func (p *podCache) OnNodeAdded(n *graph.Node) {
	nodeType, _ := n.GetFieldString("Type")
	switch nodeType {
	case "container":
		namespace, _ := n.GetFieldString("Docker.Labels.io.kubernetes.pod.namespace")
		podName, _ := n.GetFieldString("Docker.Labels.io.kubernetes.pod.name")
		if namespace != "" && podName != "" {
			p.Lock()
			defer p.Unlock()

			if pod := p.GetByKey(namespace + "/" + podName); pod != nil {
				// We already saw the pod through Kubernetes API
				podNode := p.graph.GetNode(graph.Identifier(pod.GetUID()))
				if podNode == nil {
					logging.GetLogger().Warningf("Failed to find node for pod %s", pod.GetUID())
					return
				}
				p.graph.Link(podNode, n, PodToContainerMetadata)
			}
		}
	case "host":
		for _, pod := range p.podIndexer.Get(n.Host()) {
			topology.AddOwnershipLink(p.graph, n, pod, nil)
		}
	}
}

func (p *podCache) List() (pods []*api.Pod) {
	for _, pod := range p.cache.List() {
		pods = append(pods, pod.(*api.Pod))
	}
	return
}

func (p *podCache) GetByKey(key string) *api.Pod {
	if pod, found, _ := p.cache.GetByKey(key); found {
		return pod.(*api.Pod)
	}
	return nil
}

func (p *podCache) Start() {
	p.containerIndexer.AddEventListener(p)
	p.hostIndexer.AddEventListener(p)
	p.kubeCache.Start()
}

func (p *podCache) Stop() {
	p.containerIndexer.RemoveEventListener(p)
	p.hostIndexer.RemoveEventListener(p)
	p.kubeCache.Stop()
}

func newPodCache(client *kubeClient, g *graph.Graph) *podCache {
	p := &podCache{
		graph: g,
		containerIndexer: graph.NewMetadataIndexer(g, graph.Metadata{
			"Type": "container",
			"Docker.Labels.io.kubernetes.pod.namespace": filters.NewNotFilter(filters.NewNullFilter("Docker.Labels.io.kubernetes.pod.namespace")),
			"Docker.Labels.io.kubernetes.pod.name":      filters.NewNotFilter(filters.NewNullFilter("Docker.Labels.io.kubernetes.pod.name")),
		}, "Docker.Labels.io.kubernetes.pod.namespace", "Docker.Labels.io.kubernetes.pod.name"),
		hostIndexer: graph.NewMetadataIndexer(g, graph.Metadata{"Type": "host"}, "Name"),
		podIndexer:  graph.NewMetadataIndexer(g, graph.Metadata{"Type": "pod"}, "Pod.NodeName"),
	}
	p.kubeCache = client.getCacheFor(client.Core().RESTClient(), &api.Pod{}, "pods", p)
	return p
}
