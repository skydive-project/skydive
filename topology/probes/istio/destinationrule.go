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
	models "istio.io/client-go/pkg/apis/networking/v1alpha3"
	client "istio.io/client-go/pkg/clientset/versioned"
	v1 "k8s.io/api/core/v1"
)

type destinationRuleHandler struct {
}

// Map graph node to k8s resource
func (h *destinationRuleHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	dr := obj.(*models.DestinationRule)
	m := k8s.NewMetadataFields(&dr.ObjectMeta)
	metadata := k8s.NewMetadata(Manager, "destinationrule", m, dr, dr.ObjectMeta.Name)
	metadata.SetField("TrafficPolicy", dr.Spec.TrafficPolicy != nil)
	metadata.SetField("HostName", dr.Spec.Host)
	return graph.Identifier(dr.ObjectMeta.GetUID()), metadata
}

// Dump k8s resource
func (h *destinationRuleHandler) Dump(obj interface{}) string {
	dr := obj.(*models.DestinationRule)
	return fmt.Sprintf("destinationrule{Namespace: %s, Name: %s}", dr.ObjectMeta.Namespace, dr.ObjectMeta.Name)
}

func newDestinationRuleProbe(c interface{}, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(c.(*client.Clientset).NetworkingV1alpha3().RESTClient(), &models.DestinationRule{}, "destinationrules", g, &destinationRuleHandler{})
}

func destinationRuleServiceAreLinked(a, b interface{}) bool {
	dr := a.(*models.DestinationRule)
	service := b.(*v1.Service)
	return k8s.MatchNamespace(dr, service) && dr.Spec.Host == service.Labels["app"]
}

func destinationRuleServiceEntryAreLinked(a, b interface{}) bool {
	dr := a.(*models.DestinationRule)
	se := b.(*models.ServiceEntry)

	if !k8s.MatchNamespace(dr, se) {
		return false
	}

	if dr.Spec.Host == "" || se.Spec.Hosts == nil {
		return false
	}

	for _, seHost := range se.Spec.Hosts {
		if seHost == dr.Spec.Host {
			return true
		}
	}

	return false
}

func newDestinationRuleServiceLinker(g *graph.Graph) probe.Handler {
	return k8s.NewABLinker(g, Manager, "destinationrule", k8s.Manager, "service", destinationRuleServiceAreLinked)
}

func newDestinationRuleServiceEntryLinker(g *graph.Graph) probe.Handler {
	return k8s.NewABLinker(g, Manager, "destinationrule", Manager, "serviceentry", destinationRuleServiceEntryAreLinked)
}
