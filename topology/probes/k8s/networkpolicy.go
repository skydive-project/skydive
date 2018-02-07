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
	return newMetadata("networkpolicy", np.GetNamespace(), np.GetName(), np)
}

func networkPolicyUID(np *networking_v1.NetworkPolicy) graph.Identifier {
	return graph.Identifier(np.GetUID())
}

func dumpNetworkPolicy(np *networking_v1.NetworkPolicy) string {
	return fmt.Sprintf("networkPolicy{Namespace: %s Name: %s}", np.GetNamespace(), np.GetName())
}

func (n *networkPolicyProbe) OnAdd(obj interface{}) {
	if policy, ok := obj.(*networking_v1.NetworkPolicy); ok {
		n.graph.Lock()
		policyNode := newNode(n.graph, networkPolicyUID(policy), n.newMetadata(policy))
		n.handleNetworkPolicyUpdate(policyNode, policy)
		n.graph.Unlock()
	}
}

func (n *networkPolicyProbe) OnUpdate(oldObj, newObj interface{}) {
	if policy, ok := newObj.(*networking_v1.NetworkPolicy); ok {
		n.graph.Lock()
		if policyNode := n.graph.GetNode(networkPolicyUID(policy)); policyNode != nil {
			addMetadata(n.graph, policyNode, policy)
			n.handleNetworkPolicyUpdate(policyNode, policy)
		}
		n.graph.Unlock()
	}
}

func (n *networkPolicyProbe) OnDelete(obj interface{}) {
	if policy, ok := obj.(*networking_v1.NetworkPolicy); ok {
		n.graph.Lock()
		if policyNode := n.graph.GetNode(networkPolicyUID((policy))); policyNode != nil {
			n.graph.DelNode(policyNode)
		}
		n.graph.Unlock()
	}
}

func (n *networkPolicyProbe) getPodSelector(policy *networking_v1.NetworkPolicy) (selector labels.Selector) {
	selector = labels.Everything()
	if len(policy.Spec.PodSelector.MatchLabels) > 0 {
		selector, _ = metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
	}
	return
}

func (n *networkPolicyProbe) filterPodsByLabels(in []interface{}, selector labels.Selector) (out []interface{}) {
	for _, pod := range in {
		pod := pod.(*api.Pod)
		if selector.Matches(labels.Set(pod.Labels)) {
			out = append(out, pod)
		}
	}
	return
}

func (n *networkPolicyProbe) selectedPods(policy *networking_v1.NetworkPolicy) (nodes []*graph.Node) {
	pods := n.podCache.listByNamespace(policy.Namespace)
	pods = n.filterPodsByLabels(pods, n.getPodSelector(policy))
	for _, pod := range pods {
		nodes = append(nodes, n.graph.GetNode(podUID(pod.(*api.Pod))))
	}
	return
}

func (n *networkPolicyProbe) isPodSelected(policy *networking_v1.NetworkPolicy, pod *api.Pod) bool {
	return len(n.filterPodsByLabels([]interface{}{pod}, n.getPodSelector(policy))) == 1
}

func (n *networkPolicyProbe) handleNetworkPolicyUpdate(policyNode *graph.Node, policy *networking_v1.NetworkPolicy) {
	logging.GetLogger().Debugf("Handling update of %s", dumpNetworkPolicy(policy))

	existingChildren := make(map[graph.Identifier]*graph.Node)
	for _, container := range n.graph.LookupChildren(policyNode, nil, newEdgeMetadata()) {
		existingChildren[container.ID] = container
	}

	for _, podNode := range n.selectedPods(policy) {
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
	podNamespace, _ := podNode.GetFieldString("Namespace")
	pod := n.podCache.getByKey(podNamespace, podName)
	if pod == nil {
		logging.GetLogger().Debugf("Failed to find node %s", dumpPod2(podNamespace, podName))
		return
	}

	for _, policy := range n.list() {
		pod := pod.(*api.Pod)
		policy := policy.(*networking_v1.NetworkPolicy)
		policyNode := n.graph.GetNode(networkPolicyUID(policy))
		if policyNode == nil {
			logging.GetLogger().Debugf("Failed to find node for %s", dumpNetworkPolicy(policy))
			continue
		}
		syncLink(n.graph, policyNode, podNode, n.isPodSelected(policy, pod))
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
