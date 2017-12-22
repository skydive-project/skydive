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
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "k8s.io/api/core/v1"
	networking_v1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

type networkPolicyCache struct {
	defaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*kubeCache
	graph      *graph.Graph
	podCache   *podCache
	podIndexer *graph.MetadataIndexer
}

func (n *networkPolicyCache) getMetadata(np *networking_v1.NetworkPolicy) graph.Metadata {
	return graph.Metadata{
		"Type":    "networkpolicy",
		"Manager": "k8s",
		"Name":    np.GetName(),
		"K8s":     np,
	}
}

func (n *networkPolicyCache) OnAdd(obj interface{}) {
	if policy, ok := obj.(*networking_v1.NetworkPolicy); ok {
		n.graph.Lock()
		policyNode := n.graph.NewNode(graph.Identifier(policy.GetUID()), n.getMetadata(policy))
		n.handleNetworkPolicy(policyNode, policy)
		n.graph.Unlock()
	}
}

func (n *networkPolicyCache) OnUpdate(oldObj, newObj interface{}) {
	if policy, ok := newObj.(*networking_v1.NetworkPolicy); ok {
		if policyNode := n.graph.GetNode(graph.Identifier(policy.GetUID())); policyNode != nil {
			n.graph.Lock()
			n.graph.SetMetadata(policyNode, n.getMetadata(policy))
			n.handleNetworkPolicy(policyNode, policy)
			n.graph.Unlock()
		}
	}
}

func (n *networkPolicyCache) OnDelete(obj interface{}) {
	if policy, ok := obj.(*networking_v1.NetworkPolicy); ok {
		if policyNode := n.graph.GetNode(graph.Identifier(policy.GetUID())); policyNode != nil {
			n.graph.Lock()
			n.graph.DelNode(policyNode)
			n.graph.Unlock()
		}
	}
}

func (n *networkPolicyCache) getPolicySelector(policy *networking_v1.NetworkPolicy) (podSelector labels.Selector) {
	if len(policy.Spec.PodSelector.MatchLabels) > 0 {
		podSelector, _ = metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
	}
	return
}

func (n *networkPolicyCache) filterPodsByLabels(pods []*api.Pod, selector labels.Selector) (filtered []*api.Pod) {
	for _, pod := range pods {
		if selector.Matches(labels.Set(pod.Labels)) {
			filtered = append(filtered, pod)
		}
	}
	return
}

func (n *networkPolicyCache) mapPods(pods []*api.Pod) (nodes []*graph.Node) {
	for _, pod := range pods {
		nodes = append(nodes, n.graph.GetNode(graph.Identifier(pod.GetUID())))
	}
	return
}

func (n *networkPolicyCache) handleNetworkPolicy(policyNode *graph.Node, policy *networking_v1.NetworkPolicy) {
	logging.GetLogger().Debugf("Considering network policy %s:%s", policy.GetUID(), policy.ObjectMeta.Name)

	// create links between network policy and pods
	var pods []*graph.Node
	if podSelector := n.getPolicySelector(policy); podSelector != nil {
		pods = n.mapPods(n.filterPodsByLabels(n.podCache.List(), podSelector))
	} else {
		if policy.Namespace == api.NamespaceAll {
			pods = n.mapPods(n.podCache.List())
		} else {
			pods = n.podIndexer.Get(policy.Namespace)
		}
	}

	existingChildren := make(map[graph.Identifier]*graph.Node)
	for _, container := range n.graph.LookupChildren(policyNode, nil, policyToPodMetadata) {
		existingChildren[container.ID] = container
	}

	for _, pod := range pods {
		if _, found := existingChildren[pod.ID]; found {
			delete(existingChildren, pod.ID)
		} else {
			n.graph.Link(policyNode, pod, policyToPodMetadata)
		}
	}

	for _, container := range existingChildren {
		n.graph.Unlink(policyNode, container)
	}
}

func (n *networkPolicyCache) handlePod(podNode *graph.Node) {
	podName, _ := podNode.GetFieldString("Name")
	namespace, _ := podNode.GetFieldString("Pod.Namespace")
	pod := n.podCache.GetByKey(namespace + "/" + podName)
	if pod == nil {
		logging.GetLogger().Warningf("Failed to find pod for node %s", podNode.ID)
		return
	}

	for _, policy := range n.cache.List() {
		policy := policy.(*networking_v1.NetworkPolicy)
		policyNode := n.graph.GetNode(graph.Identifier(policy.GetUID()))
		if policyNode == nil {
			logging.GetLogger().Warningf("Failed to find node for network policy %s", policy.GetName())
			continue
		}

		var toLink bool
		if podSelector := n.getPolicySelector(policy); podSelector != nil {
			toLink = podSelector.Matches(labels.Set(pod.Labels))
		} else if policy.Namespace == api.NamespaceAll || pod.Namespace == policy.Namespace {
			toLink = true
		}

		if toLink {
			n.graph.Link(policyNode, podNode, policyToPodMetadata)
		} else {
			n.graph.Unlink(policyNode, podNode)
		}
	}
}

func (n *networkPolicyCache) OnNodeAdded(node *graph.Node) {
	n.handlePod(node)
}

func (n *networkPolicyCache) OnNodeUpdated(node *graph.Node) {
	n.handlePod(node)
}

func (n *networkPolicyCache) Start() {
	n.podIndexer.AddEventListener(n)
	n.kubeCache.Start()
}

func (n *networkPolicyCache) Stop() {
	n.podIndexer.RemoveEventListener(n)
	n.kubeCache.Stop()
}

func newNetworkPolicyCache(client *kubeClient, g *graph.Graph, podCache *podCache) *networkPolicyCache {
	n := &networkPolicyCache{
		graph:      g,
		podCache:   podCache,
		podIndexer: graph.NewMetadataIndexer(g, graph.Metadata{"Type": "pod"}, "Pod.Namespace"),
	}
	n.kubeCache = client.getCacheFor(
		client.ExtensionsV1beta1().RESTClient(),
		&networking_v1.NetworkPolicy{},
		"networkpolicies",
		n)
	return n
}
