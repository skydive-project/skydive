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

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	api "istio.io/api/networking/v1alpha3"
	models "istio.io/client-go/pkg/apis/networking/v1alpha3"
	client "istio.io/client-go/pkg/clientset/versioned"
	v1 "k8s.io/api/core/v1"
)

type virtualServiceHandler struct {
}

// Map graph node to k8s resource
func (h *virtualServiceHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	vs := obj.(*models.VirtualService)
	m := k8s.NewMetadataFields(&vs.ObjectMeta)
	return graph.Identifier(vs.GetUID()), k8s.NewMetadata(Manager, "virtualservice", m, vs, vs.Name)
}

// Dump k8s resource
func (h *virtualServiceHandler) Dump(obj interface{}) string {
	vs := obj.(*models.VirtualService)
	return fmt.Sprintf("virtualservice{Namespace: %s, Name: %s}", vs.Namespace, vs.Name)
}

func newVirtualServiceProbe(c interface{}, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(c.(*client.Clientset).NetworkingV1alpha3().RESTClient(), &models.VirtualService{}, "virtualservices", g, &virtualServiceHandler{})
}

type instanceMap map[string][]string

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

func newInstanceMapFromHTTPRoutes(routes []*api.HTTPRoute) instanceMap {
	im := make(map[string][]string)
	for _, vsRule := range routes {
		for _, route := range vsRule.Route {
			if app := route.Destination.Host; app != "" {
				im[app] = append(im[app], route.Destination.Subset)
			}
		}
	}
	return im
}

func newInstanceMapFromTLSRoutes(routes []*api.TLSRoute) instanceMap {
	im := make(map[string][]string)
	for _, vsRule := range routes {
		for _, route := range vsRule.Route {
			if app := route.Destination.Host; app != "" {
				im[app] = append(im[app], route.Destination.Subset)
			}
		}
	}
	return im
}

func newInstanceMapFromTCPRoutes(routes []*api.TCPRoute) instanceMap {
	im := make(map[string][]string)
	for _, vsRule := range routes {
		for _, route := range vsRule.GetRoute() {
			if app := route.Destination.Host; app != "" {
				im[app] = append(im[app], route.Destination.Subset)
			}
		}
	}
	return im
}

func matchRoute(r *api.HTTPRouteDestination, app, version string) bool {
	if r.Destination.Host != app {
		return false
	}

	return r.Destination.Subset == version || r.Destination.Subset == ""
}

func virtualServicePodMetadata(a, b interface{}, typeA, typeB, manager string) graph.Metadata {
	vs := a.(*models.VirtualService)
	pod := b.(*v1.Pod)

	m := k8s.NewEdgeMetadata(manager, typeA)
	if pod.Labels["app"] == "" {
		return m
	}

	app, version := pod.Labels["app"], pod.Labels["version"]

	for _, http := range vs.Spec.Http {
		if http.Match != nil {
			// weights calculation - only for the default unconditional rule
			continue
		}

		if len(http.Route) == 1 && matchRoute(http.Route[0], app, version) {
			m["Weight"] = 100
			break
		}

		for _, route := range http.Route {
			if matchRoute(route, app, version) {
				m["Weight"] = route.Weight
				break
			}
		}

	}

	protocols := []string{}

	if newInstanceMapFromHTTPRoutes(vs.Spec.Http).has(app, version) {
		protocols = append(protocols, "HTTP")
	}
	if newInstanceMapFromTLSRoutes(vs.Spec.Tls).has(app, version) {
		protocols = append(protocols, "TLS")
	}
	if newInstanceMapFromTCPRoutes(vs.Spec.Tcp).has(app, version) {
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

func virtualServicePodAreLinked(a, b interface{}) bool {
	vs := a.(*models.VirtualService)
	pod := b.(*v1.Pod)

	if !k8s.MatchNamespace(vs, pod) {
		return false
	}

	app, version := pod.Labels["app"], pod.Labels["version"]

	return newInstanceMapFromHTTPRoutes(vs.Spec.Http).has(app, version) ||
		newInstanceMapFromTLSRoutes(vs.Spec.Tls).has(app, version) ||
		newInstanceMapFromTCPRoutes(vs.Spec.Tcp).has(app, version)
}

func findMissingGateways(g *graph.Graph) {
	cache := k8s.GetSubprobe(Manager, "virtualservice")
	if cache == nil {
		return
	}
	vsCache := cache.(*k8s.ResourceCache)
	vsList := vsCache.List()
	for _, vs := range vsList {
		vs := vs.(*models.VirtualService)
		if vsNode := g.GetNode(graph.Identifier(vs.GetUID())); vsNode != nil {
			k8s.SetState(&vsNode.Metadata, true)
			for _, vsGateway := range vs.Spec.Gateways {
				if isGatewayMissing(vsGateway) {
					k8s.SetState(&vsNode.Metadata, false)
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
		gateway := gateway.(*models.Gateway)
		if gateway.Name == name {
			return false
		}
	}
	return true
}

func newVirtualServicePodLinker(g *graph.Graph) probe.Handler {
	return k8s.NewABLinker(g, Manager, "virtualservice", k8s.Manager, "pod", virtualServicePodAreLinked, virtualServicePodMetadata)
}

func newVirtualServiceGatewayVerifier(g *graph.Graph) *resourceVerifier {
	vsProbe := k8s.GetSubprobe(Manager, "virtualservice")
	gatewayProbe := k8s.GetSubprobe(Manager, "gateway")
	handlers := []graph.ListenerHandler{vsProbe, gatewayProbe}
	return newResourceVerifier(g, handlers, findMissingGateways)
}
