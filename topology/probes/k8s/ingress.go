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
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type ingressHandler struct {
}

func (h *ingressHandler) Dump(obj interface{}) string {
	ingress := obj.(*v1beta1.Ingress)
	return fmt.Sprintf("ingress{Namespace: %s, Name: %s}", ingress.Namespace, ingress.Name)
}

func (h *ingressHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	ingress := obj.(*v1beta1.Ingress)

	m := NewMetadataFields(&ingress.ObjectMeta)
	m.SetFieldAndNormalize("Backend", ingress.Spec.Backend)
	m.SetFieldAndNormalize("TLS", ingress.Spec.TLS)
	m.SetFieldAndNormalize("Rules", ingress.Spec.Rules)

	return graph.Identifier(ingress.GetUID()), NewMetadata(Manager, "ingress", m, ingress, ingress.Name)
}

func newIngressProbe(client interface{}, g *graph.Graph) Subprobe {
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

func newIngressServiceLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "ingress", Manager, "service", ingressServiceAreLinked)
}
