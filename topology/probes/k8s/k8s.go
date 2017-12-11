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

	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type K8SProbe struct {
	graph              *graph.Graph
	client             *kubeClient
	podCache           *podCache
	networkPolicyCache *networkPolicyCache
}

func (k8s *K8SProbe) getPodsByNamespace(namespace string) (pods []*api.Pod) {
	for _, pod := range k8s.podCache.cache.List() {
		if pod := pod.(*api.Pod); namespace == api.NamespaceAll || pod.Namespace == namespace {
			pods = append(pods, pod)
		}
	}
	return
}

func (k8s *K8SProbe) getPodsByLabels(selector labels.Selector) (pods []*api.Pod) {
	for _, pod := range k8s.podCache.cache.List() {
		if pod := pod.(*api.Pod); selector.Matches(labels.Set(pod.Labels)) {
			pods = append(pods, pod)
		}
	}
	return
}

/*
func (k8s *K8SProbe) getNamespacesByLabels(selector labels.Selector) (namespaces []*api.Namespace) {
	for _, namespace := range k8s.namespaceCache.cache.List() {
		if namespace := pod.(*api.Namespace); selector.Matches(labels.Set(namespace.Labels)) {
			namespaces = append(namespaces, namespace)
		}
	}
	return
}
*/

func (k8s *K8SProbe) Start() {
	k8s.networkPolicyCache.Start()
	k8s.podCache.Start()
}

func (k8s *K8SProbe) Stop() {
	k8s.networkPolicyCache.Stop()
	k8s.podCache.Stop()
}

func NewK8SProbe(g *graph.Graph) (k8s *K8SProbe, err error) {
	client, err := newKubeClient()
	if err != nil {
		return nil, err
	}

	return &K8SProbe{
		graph:              g,
		client:             client,
		podCache:           newPodCache(client, g),
		networkPolicyCache: newNetworkPolicyCache(client, g),
	}, nil
}
