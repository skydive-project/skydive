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
	graph          *graph.Graph
	podCache       *kubeCache
	namespaceCache *kubeCache
	objIndexer     *graph.MetadataIndexer
}

func newObjectIndexerByNetworkPolicy(g *graph.Graph) *graph.MetadataIndexer {
	ownedByFilter := filters.NewOrFilter(
		filters.NewTermStringFilter("Type", "namespace"),
		filters.NewTermStringFilter("Type", "pod"),
	)

	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", managerValue),
		ownedByFilter,
	)
	m := graph.NewGraphElementFilter(filter)
	return graph.NewMetadataIndexer(g, m)
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
	return fmt.Sprintf("networkPolicy{Namespace: %s, Name: %s}", np.Namespace, np.Name)
}

func (n *networkPolicyProbe) OnAdd(obj interface{}) {
	if np, ok := obj.(*v1beta1.NetworkPolicy); ok {
		logging.GetLogger().Debugf("Adding %s", dumpNetworkPolicy(np))
		n.graph.Lock()
		npNode := newNode(n.graph, networkPolicyUID(np), n.newMetadata(np))
		n.handleNetworkPolicyUpdate(npNode, np)
		n.graph.Unlock()
	}
}

func (n *networkPolicyProbe) OnUpdate(oldObj, newObj interface{}) {
	if np, ok := newObj.(*v1beta1.NetworkPolicy); ok {
		logging.GetLogger().Debugf("Updating %s", dumpNetworkPolicy(np))
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
		logging.GetLogger().Debugf("Deleting %s", dumpNetworkPolicy(np))
		n.graph.Lock()
		if npNode := n.graph.GetNode(networkPolicyUID((np))); npNode != nil {
			n.graph.DelNode(npNode)
		}
		n.graph.Unlock()
	}
}

func (n *networkPolicyProbe) getPodSelector(np *v1beta1.NetworkPolicy) labels.Selector {
	selector, _ := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
	return selector
}

func (n *networkPolicyProbe) filterPodByLabels(in []interface{}, np *v1beta1.NetworkPolicy) (out []interface{}) {
	selector := n.getPodSelector(np)
	for _, pod := range in {
		pod := pod.(*corev1.Pod)
		if np.Namespace == pod.Namespace && selector.Matches(labels.Set(pod.Labels)) {
			out = append(out, pod)
		}
	}
	return
}

func (n *networkPolicyProbe) filterNamespaceByLabels(in []interface{}, np *v1beta1.NetworkPolicy) (out []interface{}) {
	if !n.getPodSelector(np).Empty() {
		return
	}

	for _, obj := range in {
		ns := obj.(*corev1.Namespace)
		if np.Namespace == ns.Name {
			out = append(out, ns)
		}
	}
	return
}

func (n *networkPolicyProbe) selectedPods(np *v1beta1.NetworkPolicy) (nodes []*graph.Node) {
	pods := n.podCache.list()
	pods = n.filterPodByLabels(pods, np)
	for _, pod := range pods {
		pod := pod.(*corev1.Pod)
		if podNode := n.graph.GetNode(podUID(pod)); podNode != nil {
			nodes = append(nodes, podNode)
		}
	}
	logging.GetLogger().Debugf("found %d pods", len(nodes))
	return
}

func (n *networkPolicyProbe) selectedNamespaces(np *v1beta1.NetworkPolicy) (nodes []*graph.Node) {
	nss := n.namespaceCache.list()
	nss = n.filterNamespaceByLabels(nss, np)
	for _, ns := range nss {
		ns := ns.(*corev1.Namespace)
		if nsNode := n.graph.GetNode(namespaceUID(ns)); nsNode != nil {
			nodes = append(nodes, nsNode)
		}
	}
	logging.GetLogger().Debugf("found %d namespaces", len(nodes))
	return
}

func (n *networkPolicyProbe) selected(np *v1beta1.NetworkPolicy) (nodes []*graph.Node) {
	pods := n.selectedPods(np)
	nss := n.selectedNamespaces(np)
	return append(pods, nss...)
}

func (n *networkPolicyProbe) isPodSelected(np *v1beta1.NetworkPolicy, pod *corev1.Pod) bool {
	return len(n.filterPodByLabels([]interface{}{pod}, np)) == 1
}

func (n *networkPolicyProbe) isNamespaceSelected(np *v1beta1.NetworkPolicy, ns *corev1.Namespace) bool {
	return len(n.filterNamespaceByLabels([]interface{}{ns}, np)) == 1
}

func (n *networkPolicyProbe) isSelected(np *v1beta1.NetworkPolicy, obj interface{}) bool {
	switch obj := obj.(type) {
	case *corev1.Pod:
		return n.isPodSelected(np, obj)
	case *corev1.Namespace:
		return n.isNamespaceSelected(np, obj)
	default:
		return false
	}
}

func (n *networkPolicyProbe) handleNetworkPolicyUpdate(npNode *graph.Node, np *v1beta1.NetworkPolicy) {
	logging.GetLogger().Debugf("Handling update of %s", dumpNetworkPolicy(np))

	selected := n.selected(np)
	staleChilderen := make(map[graph.Identifier]*graph.Node)
	childFilter := graph.Metadata{
		"Manager": managerValue,
	}

	for _, child := range n.graph.LookupChildren(npNode, childFilter, newEdgeMetadata()) {
		logging.GetLogger().Debugf("found child %s", dumpGraphNode(child))
		staleChilderen[child.ID] = child
	}

	for _, objNode := range selected {
		if _, found := staleChilderen[objNode.ID]; found {
			delete(staleChilderen, objNode.ID)
		}
	}

	for _, child := range staleChilderen {
		delLink(n.graph, npNode, child)
	}

	for _, objNode := range selected {
		addLink(n.graph, npNode, objNode)
	}
}

func (n *networkPolicyProbe) getObjByNode(node *graph.Node) interface{} {
	var cache *kubeCache

	ty, _ := node.GetFieldString("Type")
	switch ty {
	case "pod":
		cache = n.podCache
	case "namespace":
		cache = n.namespaceCache
	default:
		return nil
	}

	namespace, _ := node.GetFieldString("Namespace")
	name, _ := node.GetFieldString("Name")
	obj := cache.getByKey(namespace, name)
	return obj
}

func (n *networkPolicyProbe) onNodeUpdated(objNode *graph.Node) {
	logging.GetLogger().Debugf("update links: %s", dumpGraphNode(objNode))
	obj := n.getObjByNode(objNode)
	if obj == nil {
		logging.GetLogger().Debugf("can't find %s", dumpGraphNode(objNode))
		return
	}

	for _, np := range n.kubeCache.list() {
		np := np.(*v1beta1.NetworkPolicy)
		logging.GetLogger().Debugf("refreshing %s", dumpNetworkPolicy(np))
		npNode := n.graph.GetNode(networkPolicyUID(np))
		if npNode == nil {
			logging.GetLogger().Debugf("can't find %s", dumpNetworkPolicy(np))
			continue
		}
		syncLink(n.graph, npNode, objNode, n.isSelected(np, obj))
	}
}

func (n *networkPolicyProbe) OnNodeAdded(node *graph.Node) {
	n.onNodeUpdated(node)
}

func (n *networkPolicyProbe) OnNodeUpdated(node *graph.Node) {
	n.onNodeUpdated(node)
}

func (n *networkPolicyProbe) Start() {
	n.objIndexer.AddEventListener(n)
	n.objIndexer.Start()
	n.kubeCache.Start()
	n.podCache.Start()
	n.namespaceCache.Start()
}

func (n *networkPolicyProbe) Stop() {
	n.objIndexer.RemoveEventListener(n)
	n.objIndexer.Stop()
	n.kubeCache.Stop()
	n.podCache.Stop()
	n.namespaceCache.Stop()
}

func newNetworkPolicyKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().ExtensionsV1beta1().RESTClient(), &v1beta1.NetworkPolicy{}, "networkpolicies", handler)
}

func newNetworkPolicyProbe(g *graph.Graph) probe.Probe {
	n := &networkPolicyProbe{
		graph:          g,
		podCache:       newPodKubeCache(nil),
		namespaceCache: newNamespaceKubeCache(nil),
		objIndexer:     newObjectIndexerByNetworkPolicy(g),
	}
	n.kubeCache = newNetworkPolicyKubeCache(n)
	return n
}
