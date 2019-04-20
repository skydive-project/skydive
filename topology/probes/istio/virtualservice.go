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

type route struct {
	Destination struct {
		App     string `mapstructure:"host"`
		Version string `mapstructure:"subset"`
	} `mapstructure:"destination"`
	Weight int `mapstructure:"weight"`
}

type rule struct {
	Match  interface{} `mapstructure:"match"`
	Routes []route     `mapstructure:"route"`
}

type virtualServiceSpec struct {
	HTTP []rule `mapstructure:"http"`
	TLS  []rule `mapstructure:"tls"`
	TCP  []rule `mapstructure:"tcp"`
}

type instanceMap map[string][]string

func newInstanceMap() instanceMap {
	return make(map[string][]string)
}

func (im instanceMap) addInstancesByRules(rules []rule) {
	for _, vsRule := range rules {
		for _, route := range vsRule.Routes {
			app := route.Destination.App
			if app == "" {
				continue
			}
			version := route.Destination.Version
			im[app] = append(im[app], version)
		}
	}
}

func (im instanceMap) has(instanceApp, instanceVersion string) bool {
	for app, versions := range im {
		if app == instanceApp {
			for _, version := range versions {
				if version == "" || version == instanceVersion {
					return true
				}
			}
		}
	}
	return false
}

func matchRoute(r route, app, version string) bool {
	if r.Destination.App != app {
		return false
	}

	return r.Destination.Version == version || r.Destination.Version == ""
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

	app, version := pod.Labels["app"], pod.Labels["version"]

	for _, http := range vsSpec.HTTP {
		if http.Match != nil {
			// weights calculation - only for the default unconditional rule
			continue
		}

		if len(http.Routes) == 1 && matchRoute(http.Routes[0], app, version) {
			m["Weight"] = 100
			break
		}

		for _, route := range http.Routes {
			if matchRoute(route, app, version) {
				m["Weight"] = route.Weight
				break
			}
		}

	}

	protocols := []string{}

	if findInstance(vsSpec.HTTP, app, version) {
		protocols = append(protocols, "HTTP")
	}
	if findInstance(vsSpec.TLS, app, version) {
		protocols = append(protocols, "TLS")
	}
	if findInstance(vsSpec.TCP, app, version) {
		protocols = append(protocols, "TCP")
	}

	p := arrayToString(protocols)
	m.SetField("Protocol", p)

	return m
}

func arrayToString(arr []string) string {
	s := arr[0]
	for _, elem := range arr[1:] {
		s = s + ", " + elem
	}
	return s
}

func findInstance(r []rule, app, version string) bool {
	instances := newInstanceMap()
	instances.addInstancesByRules(r)
	return instances.has(app, version)
}

func virtualServicePodAreLinked(a, b interface{}) bool {
	vs := a.(*kiali.VirtualService)
	pod := b.(*v1.Pod)

	if !k8s.MatchNamespace(vs, pod) {
		return false
	}

	app, version := pod.Labels["app"], pod.Labels["version"]

	vsSpec := &virtualServiceSpec{}
	if err := mapstructure.Decode(vs.Spec, vsSpec); err != nil {
		return false
	}

	return findInstance(vsSpec.HTTP, app, version) || findInstance(vsSpec.TLS, app, version) || findInstance(vsSpec.TCP, app, version)
}

func findMissingGateways(g *graph.Graph) {
	cache := k8s.GetSubprobe(Manager, "virtualservice")
	if cache == nil {
		return
	}
	vsCache := cache.(*k8s.ResourceCache)
	vsList := vsCache.List()
	for _, vs := range vsList {
		vs := vs.(*kiali.VirtualService)
		if vsNode := g.GetNode(graph.Identifier(vs.GetUID())); vsNode != nil {
			k8s.SetState(&vsNode.Metadata, true)
			if vs.Spec["gateways"] != nil {
				vsGateways := vs.Spec["gateways"].([]interface{})
				for _, vsGateway := range vsGateways {
					if isGatewayMissing(vsGateway.(string)) {
						k8s.SetState(&vsNode.Metadata, false)
					}
				}
			}
		}
	}
}

func isGatewayMissing(name string) bool {
	cache := k8s.GetSubprobe(Manager, "gateway")
	if cache == nil {
		return true
	}
	gatewayCache := cache.(*k8s.ResourceCache)
	gatewaysList := gatewayCache.List()
	for _, gateway := range gatewaysList {
		gateway := gateway.(*kiali.Gateway)
		if gateway.Name == name {
			return false
		}
	}
	return true
}

func newVirtualServicePodLinker(g *graph.Graph) probe.Probe {
	return k8s.NewABLinker(g, Manager, "virtualservice", k8s.Manager, "pod", virtualServicePodAreLinked, virtualServicePodMetadata)
}

func newVirtualServiceGatewayVerifier(g *graph.Graph) *resourceVerifier {
	vsProbe := k8s.GetSubprobe(Manager, "virtualservice")
	gatewayProbe := k8s.GetSubprobe(Manager, "gateway")
	handlers := []graph.ListenerHandler{vsProbe, gatewayProbe}
	return newResourceVerifier(g, handlers, findMissingGateways)
}
