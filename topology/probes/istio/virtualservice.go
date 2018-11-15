/*
 * Copyright (C) 2018 IBM, Inc.
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

package istio

import (
	"fmt"

	kiali "github.com/kiali/kiali/kubernetes"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	"k8s.io/api/core/v1"
)

type virtualServiceHandler struct {
}

// Map graph node to k8s resource
func (h *virtualServiceHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	vs := obj.(*kiali.VirtualService)
	return graph.Identifier(vs.GetUID()), k8s.NewMetadata(Manager, "virtualservice", vs, vs.Name, vs.Namespace)
}

// Dump k8s resource
func (h *virtualServiceHandler) Dump(obj interface{}) string {
	vs := obj.(*kiali.VirtualService)
	return fmt.Sprintf("virtualservice{Namespace: %s, Name: %s}", vs.Namespace, vs.Name)
}

func newVirtualServiceProbe(client *kiali.IstioClient, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(client.GetIstioNetworkingApi(), &kiali.VirtualService{}, "virtualservices", g, &virtualServiceHandler{})
}

type virtualServiceLinker struct {
	graph    *graph.Graph
	vsCache  *k8s.ResourceCache
	drCache  *k8s.ResourceCache
	podCache *k8s.ResourceCache
}

func (vsl *virtualServiceLinker) getHostsByVirtualService(vs *kiali.VirtualService) []string {
	hosts := []string{}
	if vs.Spec["http"] == nil {
		return hosts
	}
	https := vs.Spec["http"].([]interface{})
	for _, http := range https {
		httpMap := http.(map[string]interface{})
		if httpMap["route"] == nil {
			continue
		}
		routes := httpMap["route"].([]interface{})
		for _, route := range routes {
			routeMap := route.(map[string]interface{})
			if routeMap["destination"] == nil {
				continue
			}
			destination := routeMap["destination"].(map[string]interface{})
			hosts = append(hosts, destination["host"].(string))
		}
	}
	return hosts
}

func (vsl *virtualServiceLinker) getVirtualServiceByHost(host string) *kiali.VirtualService {
	for _, vs := range vsl.vsCache.List() {
		if vs, ok := vs.(*kiali.VirtualService); ok {
			if vs.Spec["http"] == nil {
				continue
			}
			https := vs.Spec["http"].([]interface{})
			for _, http := range https {
				httpMap := http.(map[string]interface{})
				if httpMap["route"] == nil {
					continue
				}
				routes := httpMap["route"].([]interface{})
				for _, route := range routes {
					routeMap := route.(map[string]interface{})
					if routeMap["destination"] == nil {
						continue
					}
					destination := routeMap["destination"].(map[string]interface{})
					if destination["host"] == nil {
						continue
					}
					if host == destination["host"].(string) {
						return vs
					}
				}
			}
		}
	}
	return nil
}

func (vsl *virtualServiceLinker) getVersionByPod(pod *v1.Pod) string {
	return pod.Labels["version"]
}

func (vsl *virtualServiceLinker) getAppByPod(pod *v1.Pod) string {
	return pod.Labels["app"]
}

func (vsl *virtualServiceLinker) getDestinationRulesByHosts(hosts []string) []*kiali.DestinationRule {
	drs := []*kiali.DestinationRule{}
	for _, dr := range vsl.drCache.List() {
		if dr, ok := dr.(*kiali.DestinationRule); ok {
			if dr.Spec["host"] == nil {
				continue
			}
			for _, host := range hosts {
				if host == dr.Spec["host"].(string) {
					drs = append(drs, dr)
				}
			}
		}
	}
	return drs
}

func (vsl *virtualServiceLinker) getHostByDestinationRule(dr *kiali.DestinationRule) string {
	if host := dr.Spec["host"]; host != nil {
		return host.(string)
	}
	return ""
}

func (vsl *virtualServiceLinker) getAppsAndVersionsByDestinationRules(drs []*kiali.DestinationRule) ([]string, []string) {
	apps := []string{}
	versions := []string{}
	for _, dr := range drs {
		if dr.Spec["host"] == nil || dr.Spec["subsets"] == nil {
			continue
		}
		subsets := dr.Spec["subsets"].([]interface{})
		for _, subset := range subsets {
			subsetMap := subset.(map[string]interface{})
			if subsetMap["labels"] == nil {
				continue
			}
			labels := subsetMap["labels"].(map[string]interface{})
			if labels["version"] == nil {
				continue
			}
			versions = append(versions, labels["version"].(string))
			apps = append(apps, dr.Spec["host"].(string))
		}
	}
	return apps, versions
}

func (vsl *virtualServiceLinker) getDestinationRuleByVersionAndApp(version, app string) *kiali.DestinationRule {
	for _, dr := range vsl.drCache.List() {
		if dr, ok := dr.(*kiali.DestinationRule); ok {
			if dr.Spec["subsets"] == nil || dr.Spec["host"] == nil {
				continue
			}
			if app != dr.Spec["host"].(string) {
				continue
			}
			subsets := dr.Spec["subsets"].([]interface{})
			for _, subset := range subsets {
				subsetMap := subset.(map[string]interface{})
				if subsetMap["labels"] == nil {
					continue
				}
				labels := subsetMap["labels"].(map[string]interface{})
				if labels["version"] == nil {
					continue
				}
				if version == labels["version"].(string) {
					return dr
				}
			}
		}
	}
	return nil
}

func (vsl *virtualServiceLinker) getPodsByAppsAndVersions(apps, versions []string) []*v1.Pod {
	pods := []*v1.Pod{}
	for _, pod := range vsl.podCache.List() {
		if pod, ok := pod.(*v1.Pod); ok {
			for i := range apps {
				if apps[i] == pod.Labels["app"] && versions[i] == pod.Labels["version"] {
					pods = append(pods, pod)
				}
			}
		}
	}
	return pods
}

func (vsl *virtualServiceLinker) GetABLinks(vsNode *graph.Node) (edges []*graph.Edge) {
	vs := vsl.vsCache.GetByNode(vsNode)
	if vs, ok := vs.(*kiali.VirtualService); ok {
		hosts := vsl.getHostsByVirtualService(vs)
		if len(hosts) == 0 {
			return
		}
		drs := vsl.getDestinationRulesByHosts(hosts)
		if len(drs) == 0 {
			return
		}
		apps, versions := vsl.getAppsAndVersionsByDestinationRules(drs)
		if len(apps) == 0 || len(versions) == 0 {
			return
		}
		pods := vsl.getPodsByAppsAndVersions(apps, versions)
		if len(pods) == 0 {
			return
		}
		for _, pod := range pods {
			if podNode := vsl.graph.GetNode(graph.Identifier(pod.GetUID())); podNode != nil {
				id := graph.GenID(string(vsNode.ID), string(podNode.ID), "RelationType", "virtualservice")
				edges = append(edges, vsl.graph.NewEdge(id, vsNode, podNode, newEdgeMetadata("virtualservice"), ""))
			}
		}
	}
	return
}

func (vsl *virtualServiceLinker) GetBALinks(podNode *graph.Node) (edges []*graph.Edge) {
	pod := vsl.podCache.GetByNode(podNode)
	if pod, ok := pod.(*v1.Pod); ok {
		version := vsl.getVersionByPod(pod)
		app := vsl.getAppByPod(pod)
		dr := vsl.getDestinationRuleByVersionAndApp(version, app)
		if dr == nil {
			return
		}
		host := vsl.getHostByDestinationRule(dr)
		if host == "" {
			return
		}
		vs := vsl.getVirtualServiceByHost(host)
		if vs == nil {
			return
		}
		if vsNode := vsl.graph.GetNode(graph.Identifier(vs.GetUID())); vsNode != nil {
			edges = append(edges, vsl.graph.CreateEdge("", vsNode, podNode, newEdgeMetadata("virtualservice"), graph.TimeUTC(), ""))
		}
	}
	return
}

func newVirtualServicePodLinker(g *graph.Graph, probes map[string]k8s.Subprobe) probe.Probe {
	vsProbe := probes["virtualservice"]
	drProbe := probes["destinationrule"]
	podProbe := k8s.Subprobes["pod"]
	if vsProbe == nil || drProbe == nil || podProbe == nil {
		return nil
	}
	return graph.NewResourceLinker(
		g,
		vsProbe,
		podProbe,
		&virtualServiceLinker{
			graph:    g,
			vsCache:  vsProbe.(*k8s.ResourceCache),
			drCache:  drProbe.(*k8s.ResourceCache),
			podCache: podProbe.(*k8s.ResourceCache),
		},
		graph.Metadata{"RelationType": "virtualservice"},
	)
}
