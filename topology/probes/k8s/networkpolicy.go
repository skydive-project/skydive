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
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
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

func (n *networkPolicyProbe) newMetadata(np *v1beta1.NetworkPolicy) graph.Metadata {
	m := newMetadata("networkpolicy", np.Namespace, np.Name, np)
	m.SetField("Labels", np.Labels)
	m.SetField("PodSelector", np.Spec.PodSelector)
	return m
}

func networkPolicyUID(np *v1beta1.NetworkPolicy) graph.Identifier {
	return graph.Identifier(np.GetUID())
}

func dumpNetworkPolicy(np *v1beta1.NetworkPolicy) string {
	return fmt.Sprintf("networkPolicy{Namespace: %s Name: %s}", np.GetNamespace(), np.GetName())
}

func (n *networkPolicyProbe) OnAdd(obj interface{}) {
	if np, ok := obj.(*v1beta1.NetworkPolicy); ok {
		n.graph.Lock()
		npNode := newNode(n.graph, networkPolicyUID(np), n.newMetadata(np))
		n.handleNetworkPolicyUpdate(npNode, np)
		n.graph.Unlock()
	}
}

func (n *networkPolicyProbe) OnUpdate(oldObj, newObj interface{}) {
	if np, ok := newObj.(*v1beta1.NetworkPolicy); ok {
		n.graph.Lock()
		if npNode := n.graph.GetNode(networkPolicyUID(np)); npNode != nil {
			addMetadata(n.graph, npNode, np)
			n.handleNetworkPolicyUpdate(npNode, np)
		}
		n.graph.Unlock()
	}
}

func (n *networkPolicyProbe) OnDelete(obj interface{}) {
	if np, ok := obj.(*v1beta1.NetworkPolicy); ok {
		n.graph.Lock()
		if npNode := n.graph.GetNode(networkPolicyUID((np))); npNode != nil {
			n.graph.DelNode(npNode)
		}
		n.graph.Unlock()
	}
}

func (n *networkPolicyProbe) getPodSelector(np *v1beta1.NetworkPolicy) (selector labels.Selector) {
	selector = labels.Everything()
	if len(np.Spec.PodSelector.MatchLabels) > 0 {
		selector, _ = metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
	}
	return
}

func (n *networkPolicyProbe) filterPodsByLabels(in []interface{}, selector labels.Selector) (out []interface{}) {
	for _, pod := range in {
		pod := pod.(*corev1.Pod)
		if selector.Matches(labels.Set(pod.Labels)) {
			out = append(out, pod)
		}
	}
	return
}

func (n *networkPolicyProbe) selectedPods(np *v1beta1.NetworkPolicy) (nodes []*graph.Node) {
	pods := n.podCache.listByNamespace(np.Namespace)
	pods = n.filterPodsByLabels(pods, n.getPodSelector(np))
	for _, pod := range pods {
		nodes = append(nodes, n.graph.GetNode(podUID(pod.(*corev1.Pod))))
	}
	return
}

func (n *networkPolicyProbe) isPodSelected(np *v1beta1.NetworkPolicy, pod *corev1.Pod) bool {
	return len(n.filterPodsByLabels([]interface{}{pod}, n.getPodSelector(np))) == 1
}

func (n *networkPolicyProbe) handleNetworkPolicyUpdate(npNode *graph.Node, np *v1beta1.NetworkPolicy) {
	logging.GetLogger().Debugf("Handling update of %s", dumpNetworkPolicy(np))

	existingChildren := make(map[graph.Identifier]*graph.Node)
	for _, container := range n.graph.LookupChildren(npNode, nil, newEdgeMetadata()) {
		existingChildren[container.ID] = container
	}

	for _, podNode := range n.selectedPods(np) {
		if _, found := existingChildren[podNode.ID]; found {
			delete(existingChildren, podNode.ID)
		} else {
			addLink(n.graph, npNode, podNode)
		}
	}

	for _, container := range existingChildren {
		n.graph.Unlink(npNode, container)
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

	for _, np := range n.list() {
		pod := pod.(*corev1.Pod)
		np := np.(*v1beta1.NetworkPolicy)
		npNode := n.graph.GetNode(networkPolicyUID(np))
		if npNode == nil {
			logging.GetLogger().Debugf("Failed to find node for %s", dumpNetworkPolicy(np))
			continue
		}
		syncLink(n.graph, npNode, podNode, n.isPodSelected(np, pod))
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
	n.podIndexer.Start()
	n.kubeCache.Start()
	n.podCache.Start()
}

func (n *networkPolicyProbe) Stop() {
	n.podIndexer.RemoveEventListener(n)
	n.podIndexer.Stop()
	n.kubeCache.Stop()
	n.podCache.Stop()
}

func newNetworkPolicyKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().ExtensionsV1beta1().RESTClient(), &v1beta1.NetworkPolicy{}, "networkpolicies", handler)
}

func newNetworkPolicyProbe(g *graph.Graph) probe.Probe {
	n := &networkPolicyProbe{
		graph:      g,
		podCache:   newPodKubeCache(nil),
		podIndexer: newPodIndexerByNamespace(g),
	}
	n.kubeCache = newNetworkPolicyKubeCache(n)
	return n
}
