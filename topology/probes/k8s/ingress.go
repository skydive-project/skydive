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
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerIngress contains the type specific fields
// easyjson:json
type MetadataInnerIngress struct {
	MetadataInner
	Backend *v1beta1.IngressBackend `skydive:"string"`
	TLS     []v1beta1.IngressTLS
	Rules   []v1beta1.IngressRule
}

// GetField implements Getter interface
func (inner *MetadataInnerIngress) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerIngress) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerIngress) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerIngress) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerIngressDecoder implements a json message raw decoder
func MetadataInnerIngressDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerIngress
	return GenericMetadataDecoder(&inner, raw)
}

type ingressHandler struct {
}

func (h *ingressHandler) Dump(obj interface{}) string {
	ingress := obj.(*v1beta1.Ingress)
	return fmt.Sprintf("ingress{Namespace: %s, Name: %s}", ingress.Namespace, ingress.Name)
}

func (h *ingressHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	ingress := obj.(*v1beta1.Ingress)

	inner := new(MetadataInnerIngress)
	inner.MetadataInner.Setup(&ingress.ObjectMeta, ingress)
	inner.Backend = ingress.Spec.Backend
	inner.TLS = ingress.Spec.TLS
	inner.Rules = ingress.Spec.Rules

	return graph.Identifier(ingress.GetUID()), NewMetadata(Manager, "ingress", inner.Name, inner)
}

func newIngressProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerIngressDecoder, "ingress")
	return NewResourceCache(client.(*kubernetes.Clientset).ExtensionsV1beta1().RESTClient(), &v1beta1.Ingress{}, "ingresses", g, &ingressHandler{})
}

func ingressServiceAreLinked(a, b interface{}) bool {
	ingress := a.(*v1beta1.Ingress)
	service := b.(*v1.Service)

	if !MatchNamespace(ingress, service) {
		return false
	}

	if ingress.Spec.Backend != nil && ingress.Spec.Backend.ServiceName == service.Name {
		return true
	}

	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				if path.Backend.ServiceName == service.Name {
					return true
				}
			}
		}
	}
	return false
}

func newIngressServiceLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "ingress", Manager, "service", ingressServiceAreLinked)
}
