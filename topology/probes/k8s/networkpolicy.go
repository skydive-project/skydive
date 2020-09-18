/*
 * Copyright (C) 2017 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package k8s

import (
	"fmt"
	"strings"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type networkPolicyHandler struct {
}

func (h *networkPolicyHandler) Dump(obj interface{}) string {
	np := obj.(*netv1.NetworkPolicy)
	return fmt.Sprintf("networkPolicy{Namespace: %s, Name: %s}", np.Namespace, np.Name)
}

func (h *networkPolicyHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	np := obj.(*netv1.NetworkPolicy)
	m := NewMetadataFields(&np.ObjectMeta)
	return graph.Identifier(np.GetUID()), NewMetadata(Manager, "networkpolicy", m, np, np.Name)
}

func newNetworkPolicyProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).NetworkingV1().RESTClient(), &netv1.NetworkPolicy{}, "networkpolicies", g, &networkPolicyHandler{})
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

func isIngress(np *netv1.NetworkPolicy) bool {
	if len(np.Spec.Ingress) != 0 {
		return true
	}
	if len(np.Spec.PolicyTypes) == 0 {
		return true
	}
	for _, ty := range np.Spec.PolicyTypes {
		if ty == netv1.PolicyTypeIngress {
			return true
		}
	}
	return false
}

func isEgress(np *netv1.NetworkPolicy) bool {
	if len(np.Spec.Egress) != 0 {
		return true
	}
	for _, ty := range np.Spec.PolicyTypes {
		if ty == netv1.PolicyTypeEgress {
			return true
		}
	}
	return false
}

func getIngressTarget(np *netv1.NetworkPolicy) PolicyTarget {
	selector, _ := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
	if selector.Empty() && len(np.Spec.Ingress) == 0 {
		return PolicyTargetDeny
	}
	return PolicyTargetAllow
}

func getEgressTarget(np *netv1.NetworkPolicy) PolicyTarget {
	selector, _ := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
	if selector.Empty() && len(np.Spec.Egress) == 0 {
		return PolicyTargetDeny
	}
	return PolicyTargetAllow
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

func (npl *networkPolicyLinker) getPeerPods(peer netv1.NetworkPolicyPeer, namespace string) (pods []metav1.Object) {
	if podSelector := peer.PodSelector; podSelector != nil {
		pods = filterPodByPodSelector(npl.podCache.List(), podSelector, namespace)
	}

	if nsSelector := peer.NamespaceSelector; nsSelector != nil {
		if allPods := npl.podCache.List(); len(allPods) != 0 {
			for _, ns := range filterObjectsBySelector(npl.namespaceCache.List(), nsSelector) {
				pods = append(pods, filterPodByPodSelector(allPods, nil, ns.GetName())...)
			}
		}
	}
	return
}

func (npl *networkPolicyLinker) getIngressAllow(np *netv1.NetworkPolicy) (pods []metav1.Object) {
	for _, rule := range np.Spec.Ingress {
		for _, from := range rule.From {
			pods = append(pods, npl.getPeerPods(from, np.Namespace)...)
		}
	}
	return
}

func (npl *networkPolicyLinker) getEgressAllow(np *netv1.NetworkPolicy) (pods []metav1.Object) {
	for _, rule := range np.Spec.Egress {
		for _, to := range rule.To {
			pods = append(pods, npl.getPeerPods(to, np.Namespace)...)
		}
	}
	return
}

func fmtFieldPorts(ports []netv1.NetworkPolicyPort) string {
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

func getFieldPorts(np *netv1.NetworkPolicy, ty PolicyType) string {
	// TODO extend logic to be able to extract the correct (per Pod object)
	// port filter, for now all we can do is extract the ports in the case
	// that there is only a single Ingress/egress rule (in which case we
	// *know* the port filter applies to the specific edge).
	ports := []netv1.NetworkPolicyPort{}
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
	m := NewEdgeMetadata(Manager, "networkpolicy")
	m.SetField("PolicyType", string(ty))
	m.SetField("PolicyTarget", string(target))
	m.SetField("PolicyPoint", string(point))
	return m
}

func (npl *networkPolicyLinker) create1SideLinks(np *netv1.NetworkPolicy, npNode, filterNode *graph.Node, ty PolicyType, target PolicyTarget, point PolicyPoint, pods []metav1.Object) (edges []*graph.Edge) {
	podNodes := objectsToNodes(npl.graph, pods)
	metadata := npl.newEdgeMetadata(ty, target, point)
	for _, podNode := range podNodes {
		if filterNode == nil || filterNode.ID == podNode.ID {
			fields := []string{string(npNode.ID), string(podNode.ID)}
			for k, v := range metadata {
				fields = append(fields, k, v.(string))
			}
			id := graph.GenID(fields...)
			metadata.SetField("PolicyPorts", getFieldPorts(np, ty))
			edges = append(edges, npl.graph.CreateEdge(id, npNode, podNode, metadata, graph.TimeUTC(), ""))
		}
	}
	return
}

func (npl *networkPolicyLinker) create2SideLinks(np *netv1.NetworkPolicy, npNode, filterNode *graph.Node, ty PolicyType, target PolicyTarget, pods []metav1.Object) []*graph.Edge {
	selectedPods := filterObjectsBySelector(npl.podCache.List(), &np.Spec.PodSelector, np.Namespace)
	return append(
		npl.create1SideLinks(np, npNode, filterNode, ty, target, PolicyPointBegin, selectedPods),
		npl.create1SideLinks(np, npNode, filterNode, ty, target, PolicyPointEnd, pods)...,
	)
}

func (npl *networkPolicyLinker) getLinks(np *netv1.NetworkPolicy, npNode, filterNode *graph.Node) (edges []*graph.Edge) {
	if isIngress(np) {
		edges = append(edges, npl.create2SideLinks(np, npNode, filterNode, PolicyTypeIngress, getIngressTarget(np), npl.getIngressAllow(np))...)
	}
	if isEgress(np) {
		edges = append(edges, npl.create2SideLinks(np, npNode, filterNode, PolicyTypeEgress, getEgressTarget(np), npl.getEgressAllow(np))...)
	}
	return
}

func (npl *networkPolicyLinker) GetABLinks(npNode *graph.Node) (edges []*graph.Edge) {
	if np := npl.npCache.GetByNode(npNode); np != nil {
		np := np.(*netv1.NetworkPolicy)
		edges = append(edges, npl.getLinks(np, npNode, nil)...)
	}
	return
}

func (npl *networkPolicyLinker) GetBALinks(podNode *graph.Node) (edges []*graph.Edge) {
	for _, np := range npl.npCache.List() {
		np := np.(*netv1.NetworkPolicy)
		if npNode := npl.graph.GetNode(graph.Identifier(np.GetUID())); npNode != nil {
			edges = append(edges, npl.getLinks(np, npNode, podNode)...)
		}
	}
	return
}

func newNetworkPolicyLinker(g *graph.Graph) probe.Handler {
	npProbe := GetSubprobe(Manager, "networkpolicy")
	podProbe := GetSubprobe(Manager, "pod")
	namespaceProbe := GetSubprobe(Manager, "namespace")
	if npProbe == nil || podProbe == nil || namespaceProbe == nil {
		return nil
	}

	npLinker := &networkPolicyLinker{
		graph:          g,
		npCache:        npProbe.(*ResourceCache),
		podCache:       podProbe.(*ResourceCache),
		namespaceCache: namespaceProbe.(*ResourceCache),
	}

	rl := graph.NewResourceLinker(g, []graph.ListenerHandler{npProbe}, []graph.ListenerHandler{podProbe},
		npLinker, graph.Metadata{"RelationType": "networkpolicy"})

	linker := &Linker{
		ResourceLinker: rl,
	}
	rl.AddEventListener(linker)

	return linker
}
