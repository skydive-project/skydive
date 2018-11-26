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
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerService contains the type specific fields
// easyjson:json
type MetadataInnerService struct {
	MetadataInner
	Ports           []v1.ServicePort
	ClusterIP       string             `skydive:"string"`
	ServiceType     v1.ServiceType     `skydive:"string"`
	SessionAffinity v1.ServiceAffinity `skydive:"string"`
	LoadBalancerIP  string             `skydive:"string"`
	ExternalName    string             `skydive:"string"`
}

// GetField implements Getter interface
func (inner *MetadataInnerService) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerService) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerService) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerService) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerServiceDecoder implements a json message raw decoder
func MetadataInnerServiceDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerService
	return GenericMetadataDecoder(&inner, raw)
}

type serviceHandler struct {
}

func (h *serviceHandler) Dump(obj interface{}) string {
	srv := obj.(*v1.Service)
	return fmt.Sprintf("service{Namespace: %s, Name: %s}", srv.Namespace, srv.Name)
}

func (h *serviceHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	srv := obj.(*v1.Service)

	inner := new(MetadataInnerService)
	inner.MetadataInner.Setup(&srv.ObjectMeta, srv)
	inner.Ports = srv.Spec.Ports
	inner.ClusterIP = srv.Spec.ClusterIP
	inner.ServiceType = srv.Spec.Type
	inner.SessionAffinity = srv.Spec.SessionAffinity
	inner.LoadBalancerIP = srv.Spec.LoadBalancerIP
	inner.ExternalName = srv.Spec.ExternalName

	return graph.Identifier(srv.GetUID()), NewMetadata(Manager, "service", inner.Name, inner)
}

func newServiceProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerServiceDecoder, "service")
	return NewResourceCache(client.(*kubernetes.Clientset).Core().RESTClient(), &v1.Service{}, "services", g, &serviceHandler{})
}

func servicePodAreLinked(a, b interface{}) bool {
	service := a.(*v1.Service)
	pod := b.(*v1.Pod)
	return MatchNamespace(pod, service) && matchMapSelector(pod, service.Spec.Selector, false)
}

func newServicePodLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "service", Manager, "pod", servicePodAreLinked)
}

func serviceEndpointsAreLinked(a, b interface{}) bool {
	endpoints := b.(*v1.Endpoints)
	service := a.(*v1.Service)
	return MatchNamespace(endpoints, service) && (endpoints.Name == service.Name || matchMapSelector(endpoints, service.Spec.Selector, false))
}

func newServiceEndpointsLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "service", Manager, "endpoints", serviceEndpointsAreLinked)
}
