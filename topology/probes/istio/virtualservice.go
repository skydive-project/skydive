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

type virtualServiceSpec struct {
	HTTP []struct {
		Route []struct {
			Destination struct {
				App     string `mapstructure:"host"`
				Version string `mapstructure:"subset"`
			} `mapstructure:"destination"`
		} `mapstructure:"route"`
	} `mapstructure:"http"`
}

func (vsSpec virtualServiceSpec) getAppsVersions() map[string][]string {
	appsVersions := make(map[string][]string)
	for _, http := range vsSpec.HTTP {
		for _, route := range http.Route {
			app := route.Destination.App
			version := route.Destination.Version
			appsVersions[app] = append(appsVersions[app], version)
		}
	}
	return appsVersions
}

func virtualServicePodAreLinked(a, b interface{}) bool {
	vs := a.(*kiali.VirtualService)
	pod := b.(*v1.Pod)
	vsSpec := &virtualServiceSpec{}
	if err := mapstructure.Decode(vs.Spec, vsSpec); err != nil {
		return false
	}
	vsAppsVersions := vsSpec.getAppsVersions()
	for app, versions := range vsAppsVersions {
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
	return k8s.NewABLinker(g, Manager, "virtualservice", k8s.Manager, "pod", virtualServicePodAreLinked)
}
