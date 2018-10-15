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
	"strings"
	"time"

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type networkPolicyHandler struct {
	DefaultResourceHandler
}

func (h *networkPolicyHandler) IsTopLevel() bool {
	return true
}

func (h *networkPolicyHandler) Dump(obj interface{}) string {
	np := obj.(*v1beta1.NetworkPolicy)
	return fmt.Sprintf("networkPolicy{Namespace: %s, Name: %s}", np.Namespace, np.Name)
}

func (h *networkPolicyHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	np := obj.(*v1beta1.NetworkPolicy)
	m := NewMetadata(Manager, "networkpolicy", np, np.Name, np.Namespace)
	return graph.Identifier(np.GetUID()), m
}

func newNetworkPolicyProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return NewResourceCache(clientset.ExtensionsV1beta1().RESTClient(), &v1beta1.NetworkPolicy{}, "networkpolicies", g, &networkPolicyHandler{})
}

type networkPolicyLinker struct {
	graph          *graph.Graph
	npCache        *ResourceCache
	podCache       *ResourceCache
	namespaceCache *ResourceCache
}

// PolicyType defines the policy type (ingress or egress)
type PolicyType string

// Policy types
const (
	PolicyTypeIngress PolicyType = "ingress"
	PolicyTypeEgress  PolicyType = "egress"
)

// String returns the string representation of a policy type
func (val PolicyType) String() string {
	return string(val)
}

// PolicyTarget defines whether traffic is allowed or denied
type PolicyTarget string

// Policy targets
const (
	PolicyTargetDeny  PolicyTarget = "deny"
	PolicyTargetAllow PolicyTarget = "allow"
)

// String returns the string representation of a policy target
func (val PolicyTarget) String() string {
	return string(val)
}

// PolicyPoint defines whether a policy applies to a of pods
// or if it restricts access from a set of pods
type PolicyPoint string

// PolicyPoint values
const (
	PolicyPointBegin PolicyPoint = "begin"
	PolicyPointEnd   PolicyPoint = "end"
)

// String returns the string representation of a PolicyPoint
func (val PolicyPoint) String() string {
	return string(val)
}

func isIngress(np *v1beta1.NetworkPolicy) bool {
	if len(np.Spec.Ingress) != 0 {
		return true
	}
	if len(np.Spec.PolicyTypes) == 0 {
		return true
	}
	for _, ty := range np.Spec.PolicyTypes {
		if ty == v1beta1.PolicyTypeIngress {
			return true
		}
	}
	return false
}

func isEgress(np *v1beta1.NetworkPolicy) bool {
	if len(np.Spec.Egress) != 0 {
		return true
	}
	for _, ty := range np.Spec.PolicyTypes {
		if ty == v1beta1.PolicyTypeEgress {
			return true
		}
	}
	return false
}

func isIngressDeny(np *v1beta1.NetworkPolicy) bool {
	selector, _ := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
	return selector.Empty() && len(np.Spec.Ingress) == 0
}

func isEgressDeny(np *v1beta1.NetworkPolicy) bool {
	selector, _ := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
	return selector.Empty() && len(np.Spec.Egress) == 0
}

func filterPodByPodSelector(in []interface{}, podSelector *metav1.LabelSelector, namespace string) (pods []metav1.Object) {
	selector, _ := metav1.LabelSelectorAsSelector(podSelector)
	for _, pod := range in {
		pod := pod.(metav1.Object)
		if namespace != pod.GetNamespace() {
			continue
		}
		if podSelector == nil {
			goto Append
		}
		if selector.Empty() {
			goto Append
		}
		if selector.Matches(labels.Set(pod.GetLabels())) {
			goto Append
		}
		continue
	Append:
		pods = append(pods, pod.(metav1.Object))
	}
	return
}

func filterNamespaceByNamespaceSelector(in []interface{}, namespaceSelector *metav1.LabelSelector) (namespaces []metav1.Object) {
	selector, _ := metav1.LabelSelectorAsSelector(namespaceSelector)
	for _, ns := range in {
		ns := ns.(*corev1.Namespace)
		if selector.Matches(labels.Set(ns.Labels)) {
			namespaces = append(namespaces, ns)
		}
	}
	return
}

func (npl *networkPolicyLinker) getPeerPods(peer v1beta1.NetworkPolicyPeer, namespace string) (pods []metav1.Object) {
	if podSelector := peer.PodSelector; podSelector != nil {
		pods = filterPodByPodSelector(npl.podCache.list(), podSelector, namespace)
	}

	if nsSelector := peer.NamespaceSelector; nsSelector != nil {
		if allPods := npl.podCache.list(); len(allPods) != 0 {
			for _, ns := range filterNamespaceByNamespaceSelector(npl.namespaceCache.list(), nsSelector) {
				pods = append(pods, filterPodByPodSelector(allPods, nil, ns.(*corev1.Namespace).Name)...)
			}
		}
	}
	return
}

func (npl *networkPolicyLinker) getIngressAllow(np *v1beta1.NetworkPolicy) (pods []metav1.Object) {
	for _, rule := range np.Spec.Ingress {
		for _, from := range rule.From {
			pods = append(pods, npl.getPeerPods(from, np.Namespace)...)
		}
	}
	return
}

func (npl *networkPolicyLinker) getEgressAllow(np *v1beta1.NetworkPolicy) (pods []metav1.Object) {
	for _, rule := range np.Spec.Egress {
		for _, to := range rule.To {
			pods = append(pods, npl.getPeerPods(to, np.Namespace)...)
		}
	}
	return
}

func fmtFieldPorts(ports []v1beta1.NetworkPolicyPort) string {
	strPorts := []string{}
	for _, p := range ports {
		proto := ""
		if p.Protocol != nil && *p.Protocol != corev1.ProtocolTCP {
			proto = "/" + string(*p.Protocol)
		}
		strPorts = append(strPorts, fmt.Sprintf(":%s%s", p.Port, proto))
	}
	return strings.Join(strPorts, ";")
}

func getFieldPorts(np *v1beta1.NetworkPolicy, ty PolicyType) string {
	// TODO extend logic to be able to extract the correct (per Pod object)
	// port filter, for now all we can do is extract the ports in the case
	// that there is only a single Ingress/egress rule (in which case we
	// *know* the port filter applies to the specific edge).
	ports := []v1beta1.NetworkPolicyPort{}
	switch ty {
	case PolicyTypeIngress:
		if len(np.Spec.Ingress) == 1 {
			ports = np.Spec.Ingress[0].Ports
		}
	case PolicyTypeEgress:
		if len(np.Spec.Egress) == 1 {
			ports = np.Spec.Egress[0].Ports
		}
	}
	return fmtFieldPorts(ports)
}

func (npl *networkPolicyLinker) newEdgeMetadata(ty PolicyType, target PolicyTarget, point PolicyPoint) graph.Metadata {
	m := newEdgeMetadata()
	m.SetField("RelationType", "networkpolicy")
	m.SetField("PolicyType", string(ty))
	m.SetField("PolicyTarget", string(target))
	m.SetField("PolicyPoint", string(point))
	return m
}

func (npl *networkPolicyLinker) createLinks(np *v1beta1.NetworkPolicy, npNode, filterNode *graph.Node, ty PolicyType, target PolicyTarget, point PolicyPoint, pods []metav1.Object) (edges []*graph.Edge) {
	podNodes := objectsToNodes(npl.graph, pods)
	metadata := npl.newEdgeMetadata(ty, target, point)
	for _, objNode := range podNodes {
		if filterNode == nil || filterNode.ID == objNode.ID {
			metadata.SetFieldAndNormalize("Ports", getFieldPorts(np, ty))
			fields := []string{string(npNode.ID), string(objNode.ID)}
			for k, v := range metadata {
				fields = append(fields, k, v.(string))
			}
			id := graph.GenID(fields...)
			edges = append(edges, npl.graph.CreateEdge(id, npNode, objNode, metadata, time.Now(), ""))
		}
	}
	return
}

func (npl *networkPolicyLinker) getLinks(np *v1beta1.NetworkPolicy, npNode, filterNode *graph.Node) (edges []*graph.Edge) {
	createLinks := func(ty PolicyType, target PolicyTarget, pods []metav1.Object) []*graph.Edge {
		selectedPods := filterPodByPodSelector(npl.podCache.list(), &np.Spec.PodSelector, np.Namespace)
		return append(
			npl.createLinks(np, npNode, filterNode, ty, target, PolicyPointBegin, selectedPods),
			npl.createLinks(np, npNode, filterNode, ty, target, PolicyPointEnd, pods)...,
		)
	}

	if isIngress(np) {
		if isIngressDeny(np) {
			edges = append(edges, createLinks(PolicyTypeIngress, PolicyTargetDeny, npl.getIngressAllow(np))...)
		} else {
			edges = append(edges, createLinks(PolicyTypeIngress, PolicyTargetAllow, npl.getIngressAllow(np))...)
		}
	}

	if isEgress(np) {
		if isEgressDeny(np) {
			edges = append(edges, createLinks(PolicyTypeEgress, PolicyTargetDeny, npl.getEgressAllow(np))...)
		} else {
			edges = append(edges, createLinks(PolicyTypeEgress, PolicyTargetAllow, npl.getEgressAllow(np))...)
		}
	}

	return
}

func (npl *networkPolicyLinker) GetABLinks(npNode *graph.Node) (edges []*graph.Edge) {
	if np := npl.npCache.getByNode(npNode); np != nil {
		np := np.(*v1beta1.NetworkPolicy)
		return npl.getLinks(np, npNode, nil)
	}
	return
}

func (npl *networkPolicyLinker) GetBALinks(objNode *graph.Node) (edges []*graph.Edge) {
	for _, np := range npl.npCache.list() {
		np := np.(*v1beta1.NetworkPolicy)
		npNode := npl.graph.GetNode(graph.Identifier(np.GetUID()))
		if npNode == nil {
			logging.GetLogger().Debugf("can't find networkpolicy %s", np.GetUID())
			continue
		}
		if nodeType, _ := objNode.GetFieldString("Type"); nodeType == "pod" {
			return npl.getLinks(np, npNode, objNode)
		}
		// FIXME: for type "namespace" its' more efficient to
		// loop through all related pods rather than pass nil
		return npl.getLinks(np, npNode, nil)
	}

	return
}

func newNetworkPolicyLinker(g *graph.Graph, subprobes map[string]Subprobe) probe.Probe {
	npProbe := subprobes["networkpolicy"]
	podProbe := subprobes["pod"]
	namespaceProbe := subprobes["namespace"]
	if npProbe == nil || podProbe == nil || namespaceProbe == nil {
		return nil
	}

	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", Manager),
		filters.NewOrFilter(
			filters.NewTermStringFilter("Type", "namespace"),
			filters.NewTermStringFilter("Type", "pod"),
		),
	)
	podNamespaceIndexer := graph.NewMetadataIndexer(g, g, graph.NewElementFilter(filter))
	podNamespaceIndexer.Start()

	linker := &networkPolicyLinker{
		graph:          g,
		npCache:        npProbe.(*ResourceCache),
		podCache:       podProbe.(*ResourceCache),
		namespaceCache: namespaceProbe.(*ResourceCache),
	}

	return graph.NewResourceLinker(g, npProbe, podNamespaceIndexer, linker, graph.Metadata{"RelationType": "networkpolicy"})
}
