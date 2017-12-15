/*
 * Copyright 2016 IBM Corp.
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
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type probe struct {
	graph              *graph.Graph
	client             *kubeClient
	podCache           *PodCache
	networkPolicyCache *NetworkPolicyCache
}

func (p *probe) getPodsByNamespace(namespace string) (pods []*v1.Pod) {
	for _, pod := range p.podCache.cache.List() {
		if pod := pod.(*v1.Pod); namespace == v1.NamespaceAll || pod.Namespace == namespace {
			pods = append(pods, pod)
		}
	}
	return
}

func (p *probe) getPodsByLabels(selector labels.Selector) (pods []*v1.Pod) {
	for _, pod := range p.podCache.cache.List() {
		if pod := pod.(*v1.Pod); selector.Matches(labels.Set(pod.Labels)) {
			pods = append(pods, pod)
		}
	}
	return
}

/*
func (p *probe) getNamespacesByLabels(selector labels.Selector) (namespaces []*v1.Namespace) {
	for _, namespace := range p.namespaceCache.cache.List() {
		if namespace := pod.(*v1.Namespace); selector.Matches(labels.Set(namespace.Labels)) {
			namespaces = append(namespaces, namespace)
		}
	}
	return
}
*/

// Start the k8s probe
func (p *probe) Start() {
	p.networkPolicyCache.Start()
	p.podCache.Start()
}

// Stop the k8s probe
func (p *probe) Stop() {
	p.networkPolicyCache.Stop()
	p.podCache.Stop()
}

// Newprobe for monitoring k8s events
func NewProbe(g *graph.Graph) (*probe, error) {
	client, err := newKubeClient()
	if err != nil {
		return nil, err
	}

	return &probe{
		graph:              g,
		client:             client,
		podCache:           newPodCache(client, g),
		networkPolicyCache: newNetworkPolicyCache(client, g),
	}, nil
}
