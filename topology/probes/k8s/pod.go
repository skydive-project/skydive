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

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"

	api "k8s.io/api/core/v1"
)

type podCache struct {
	sync.RWMutex
	defaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*kubeCache
	graph      *graph.Graph
	podToGraph map[string]*graph.Node
}

func (p *podCache) OnAdd(obj interface{}) {
	if pod, ok := obj.(*api.Pod); ok {
		logging.GetLogger().Debugf("Pod %s created with name %s", pod.GetUID(), pod.ObjectMeta.Name)
		p.Lock()
		defer p.Unlock()

		if n, found := p.podToGraph[pod.GetName()]; found {
			p.handlePod(pod, n)
		}
	}
}

func (p *podCache) podMetadata(pod *api.Pod) graph.Metadata {
	return graph.Metadata{
		"Type": "pod",
		"Name": pod.GetName(),
		"Pod": map[string]interface{}{
			"ClusterName":     pod.GetClusterName(),
			"Namespace":       pod.GetNamespace(),
			"UID":             pod.GetUID(),
			"ResourceVersion": pod.GetResourceVersion(),
			"Labels":          pod.GetLabels(),
		},
	}
}

func (p *podCache) OnDelete(obj interface{}) {
	if pod, ok := obj.(*api.Pod); ok {
		if podNode := p.graph.GetNode(graph.Identifier(pod.GetUID())); podNode != nil {
			p.graph.DelNode(podNode)
		}
		delete(p.podToGraph, pod.GetName())
	}
}

func (p *podCache) handlePod(pod *api.Pod, containerNode *graph.Node) {
	podNode := p.graph.NewNode(graph.Identifier(pod.GetUID()), p.podMetadata(pod))
	p.graph.Link(containerNode, podNode, nil)
}

func (p *podCache) mapNode(n *graph.Node) {
	namespace, _ := n.GetFieldString("Docker.Labels.io.kubernetes.pod.namespace")
	podName, _ := n.GetFieldString("Docker.Labels.io.kubernetes.pod.name")
	if namespace != "" && podName != "" {
		p.Lock()
		defer p.Unlock()

		p.podToGraph[podName] = n
		if pod, found, _ := p.cache.GetByKey(namespace + "/" + podName); found {
			p.handlePod(pod.(*api.Pod), n)
		}
	}
}

func (p *podCache) OnNodeAdded(n *graph.Node) {
	p.mapNode(n)
}

func (p *podCache) OnNodeDeleted(n *graph.Node) {
	if podName, _ := n.GetFieldString("Docker.Labels.io.kubernetes.pod.name"); podName != "" {
		p.Lock()
		delete(p.podToGraph, podName)
		p.Unlock()
	}
}

func (p *podCache) Start() {
	p.graph.AddEventListener(p)
}

func (p *podCache) Stop() {
	p.graph.RemoveEventListener(p)
}

func newPodCache(client *kubeClient, g *graph.Graph) *podCache {
	p := &podCache{
		podToGraph: make(map[string]*graph.Node),
		graph:      g,
	}
	p.kubeCache = client.getCacheFor(
		client.Core().RESTClient(),
		&api.Pod{},
		"pods",
		p)
	return p
}
