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

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"

	api "k8s.io/api/core/v1"
	networking_v1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type networkPolicyProbe struct {
	defaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*kubeCache
	graph      *graph.Graph
	podCache   *kubeCache
	podIndexer *graph.MetadataIndexer
}

func (n *networkPolicyProbe) newMetadata(np *networking_v1.NetworkPolicy) graph.Metadata {
	return newMetadata("networkpolicy", np.GetName(), np)
}

func netpolUID(np *networking_v1.NetworkPolicy) graph.Identifier {
	return graph.Identifier(np.GetUID())
}

func dumpNetworkPolicy(np *networking_v1.NetworkPolicy) string {
	return fmt.Sprintf("networkPolicy{Namespace: %s Name: %s}", np.GetNamespace(), np.GetName())
}

func (n *networkPolicyProbe) OnAdd(obj interface{}) {
	if policy, ok := obj.(*networking_v1.NetworkPolicy); ok {
		n.graph.Lock()
		policyNode := n.graph.NewNode(netpolUID(policy), n.newMetadata(policy))
		n.handleNetworkPolicyUpdate(policyNode, policy)
		n.graph.Unlock()
	}
}

func (n *networkPolicyProbe) OnUpdate(oldObj, newObj interface{}) {
	if policy, ok := newObj.(*networking_v1.NetworkPolicy); ok {
		if policyNode := n.graph.GetNode(netpolUID(policy)); policyNode != nil {
			n.graph.Lock()
			addMetadata(n.graph, policyNode, policy)
			n.handleNetworkPolicyUpdate(policyNode, policy)
			n.graph.Unlock()
		}
	}
}

func (n *networkPolicyProbe) OnDelete(obj interface{}) {
	if policy, ok := obj.(*networking_v1.NetworkPolicy); ok {
		if policyNode := n.graph.GetNode(netpolUID((policy))); policyNode != nil {
			n.graph.Lock()
			n.graph.DelNode(policyNode)
			n.graph.Unlock()
		}
	}
}

func (n *networkPolicyProbe) getPolicySelector(policy *networking_v1.NetworkPolicy) (podSelector labels.Selector) {
	if len(policy.Spec.PodSelector.MatchLabels) > 0 {
		podSelector, _ = metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
	}
	return
}

func (n *networkPolicyProbe) filterPodsByLabels(pods []interface{}, selector labels.Selector) (filtered []interface{}) {
	for _, pod := range pods {
		pod := pod.(*api.Pod)
		if selector.Matches(labels.Set(pod.Labels)) {
			filtered = append(filtered, pod)
		}
	}
	return
}

func (n *networkPolicyProbe) mapPods(pods []interface{}) (nodes []*graph.Node) {
	for _, pod := range pods {
		nodes = append(nodes, n.graph.GetNode(podUID(pod.(*api.Pod))))
	}
	return
}

func (n *networkPolicyProbe) handleNetworkPolicyUpdate(policyNode *graph.Node, policy *networking_v1.NetworkPolicy) {
	logging.GetLogger().Debugf("Handling update of %s", dumpNetworkPolicy(policy))

	var pods []*graph.Node
	if podSelector := n.getPolicySelector(policy); podSelector != nil {
		pods = n.mapPods(n.filterPodsByLabels(n.podCache.list(), podSelector))
	} else {
		if policy.Namespace == api.NamespaceAll {
			pods = n.mapPods(n.podCache.list())
		} else {
			pods = n.podIndexer.Get(policy.Namespace)
		}
	}

	existingChildren := make(map[graph.Identifier]*graph.Node)
	for _, container := range n.graph.LookupChildren(policyNode, nil, newEdgeMetadata()) {
		existingChildren[container.ID] = container
	}

	for _, podNode := range pods {
		if _, found := existingChildren[podNode.ID]; found {
			delete(existingChildren, podNode.ID)
		} else {
			addLink(n.graph, policyNode, podNode)
		}
	}

	for _, container := range existingChildren {
		n.graph.Unlink(policyNode, container)
	}
}

func (n *networkPolicyProbe) onPodUpdated(podNode *graph.Node) {
	podName, _ := podNode.GetFieldString("Name")
	podNamespace, _ := podNode.GetFieldString("Pod.Namespace")
	pod := n.podCache.getByKey(podNamespace, podName)
	if pod == nil {
		logging.GetLogger().Debugf("Failed to find node %s", dumpPod2(podNamespace, podName))
		return
	}

	for _, policy := range n.list() {
		pod := pod.(*api.Pod)
		policy := policy.(*networking_v1.NetworkPolicy)
		policyNode := n.graph.GetNode(netpolUID(policy))
		if policyNode == nil {
			logging.GetLogger().Debugf("Failed to find node for %s", dumpNetworkPolicy(policy))
			continue
		}

		var toLink bool
		if podSelector := n.getPolicySelector(policy); podSelector != nil {
			toLink = podSelector.Matches(labels.Set(pod.Labels))
		} else if policy.Namespace == api.NamespaceAll || pod.Namespace == policy.Namespace {
			toLink = true
		}

		syncLink(n.graph, policyNode, podNode, toLink)
	}
}

func (n *networkPolicyProbe) OnNodeAdded(node *graph.Node) {
	n.onPodUpdated(node)
}

func (n *networkPolicyProbe) OnNodeUpdated(node *graph.Node) {
	n.onPodUpdated(node)
}

func (n *networkPolicyProbe) Start() {
	n.podIndexer.AddEventListener(n)
	n.kubeCache.Start()
	n.podCache.Start()
}

func (n *networkPolicyProbe) Stop() {
	n.podIndexer.RemoveEventListener(n)
	n.kubeCache.Stop()
	n.podCache.Stop()
}

func newNetworkPolicyKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().ExtensionsV1beta1().RESTClient(), &networking_v1.NetworkPolicy{}, "networkpolicies", handler)
}

func newNetworkPolicyProbe(g *graph.Graph) *networkPolicyProbe {
	n := &networkPolicyProbe{
		graph:      g,
		podCache:   newPodKubeCache(nil),
		podIndexer: newPodIndexerByNamespace(g),
	}
	n.kubeCache = newNetworkPolicyKubeCache(n)
	return n
}
