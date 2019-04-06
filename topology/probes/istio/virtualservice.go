/*
 * Copyright (C) 2018 IBM, Inc.
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

package istio

import (
	"fmt"

	kiali "github.com/kiali/kiali/kubernetes"
	"github.com/mitchellh/mapstructure"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	"k8s.io/api/core/v1"
)

type virtualServiceHandler struct {
}

// Map graph node to k8s resource
func (h *virtualServiceHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	vs := obj.(*kiali.VirtualService)
	m := k8s.NewMetadataFields(&vs.ObjectMeta)
	return graph.Identifier(vs.GetUID()), k8s.NewMetadata(Manager, "virtualservice", m, vs, vs.Name)
}

// Dump k8s resource
func (h *virtualServiceHandler) Dump(obj interface{}) string {
	vs := obj.(*kiali.VirtualService)
	return fmt.Sprintf("virtualservice{Namespace: %s, Name: %s}", vs.Namespace, vs.Name)
}

func newVirtualServiceProbe(client interface{}, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(client.(*kiali.IstioClient).GetIstioNetworkingApi(), &kiali.VirtualService{}, "virtualservices", g, &virtualServiceHandler{})
}

type rule struct {
	Routes []struct {
		Destination struct {
			App     string `mapstructure:"host"`
			Version string `mapstructure:"subset"`
		} `mapstructure:"destination"`
		Weight int `mapstructure:"weight"`
	} `mapstructure:"route"`
}

type virtualServiceSpec struct {
	HTTP []rule `mapstructure:"http"`
	TLS  []rule `mapstructure:"tls"`
	TCP  []rule `mapstructure:"tcp"`
}

func getInstances(rules []rule, instances map[string][]string) {
	for _, vsRule := range rules {
		for _, route := range vsRule.Routes {
			app := route.Destination.App
			if app == "" {
				continue
			}
			version := route.Destination.Version
			instances[app] = append(instances[app], version)
		}
	}
}

func virtualServicePodMetadata(a, b interface{}, typeA, typeB, manager string) graph.Metadata {
	vs := a.(*kiali.VirtualService)
	pod := b.(*v1.Pod)
	vsSpec := &virtualServiceSpec{}
	if err := mapstructure.Decode(vs.Spec, vsSpec); err != nil {
		return nil
	}
	m := k8s.NewEdgeMetadata(manager, typeA)
	if pod.Labels["app"] == "" {
		return m
	}
	weight := 0
	for _, http := range vsSpec.HTTP {
		for _, route := range http.Routes {
			if route.Destination.App != pod.Labels["app"] {
				continue
			}
			if route.Destination.Version == pod.Labels["version"] || route.Destination.Version == "" {
				weight += route.Weight
			}
		}
	}
	if weight > 0 {
		m["weight"] = weight
	}
	return m
}

func virtualServicePodAreLinked(a, b interface{}) bool {
	vs := a.(*kiali.VirtualService)
	pod := b.(*v1.Pod)

	if !k8s.MatchNamespace(vs, pod) {
		return false
	}

	vsSpec := &virtualServiceSpec{}
	if err := mapstructure.Decode(vs.Spec, vsSpec); err != nil {
		return false
	}

	instances := make(map[string][]string)
	getInstances(vsSpec.HTTP, instances)
	getInstances(vsSpec.TLS, instances)
	getInstances(vsSpec.TCP, instances)

	for app, versions := range instances {
		if app == pod.Labels["app"] {
			for _, version := range versions {
				if version == "" || version == pod.Labels["version"] {
					return true
				}
			}
		}
	}
	return false
}

func newVirtualServicePodLinker(g *graph.Graph) probe.Probe {
	return k8s.NewABLinker(g, Manager, "virtualservice", k8s.Manager, "pod", virtualServicePodAreLinked, virtualServicePodMetadata)
}
