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

package k8s

import (
	"fmt"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type serviceHandler struct {
}

func (h *serviceHandler) Dump(obj interface{}) string {
	srv := obj.(*v1.Service)
	return fmt.Sprintf("service{Namespace: %s, Name: %s}", srv.Namespace, srv.Name)
}

func (h *serviceHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	srv := obj.(*v1.Service)

	m := NewMetadataFields(&srv.ObjectMeta)
	m.SetFieldAndNormalize("Ports", srv.Spec.Ports)
	m.SetFieldAndNormalize("ClusterIP", srv.Spec.ClusterIP)
	m.SetFieldAndNormalize("ServiceType", srv.Spec.Type)
	m.SetFieldAndNormalize("SessionAffinity", srv.Spec.SessionAffinity)
	m.SetFieldAndNormalize("LoadBalancerIP", srv.Spec.LoadBalancerIP)
	m.SetFieldAndNormalize("ExternalName", srv.Spec.ExternalName)

	return graph.Identifier(srv.GetUID()), NewMetadata(Manager, "service", m, srv, srv.Name)
}

func newServiceProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.Service{}, "services", g, &serviceHandler{})
}

func servicePodAreLinked(a, b interface{}) bool {
	service := a.(*v1.Service)
	pod := b.(*v1.Pod)
	return MatchNamespace(pod, service) && matchMapSelector(pod, service.Spec.Selector, false)
}

func newServicePodLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "service", Manager, "pod", servicePodAreLinked)
}

func serviceEndpointsAreLinked(a, b interface{}) bool {
	endpoints := b.(*v1.Endpoints)
	service := a.(*v1.Service)
	return MatchNamespace(endpoints, service) && (endpoints.Name == service.Name || matchMapSelector(endpoints, service.Spec.Selector, false))
}

func newServiceEndpointsLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "service", Manager, "endpoints", serviceEndpointsAreLinked)
}
