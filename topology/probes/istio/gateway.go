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
)

type gatewayHandler struct {
}

// Map graph node to k8s resource
func (h *gatewayHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	gw := obj.(*models.Gateway)
	m := k8s.NewMetadataFields(&gw.ObjectMeta)
	return graph.Identifier(gw.GetUID()), k8s.NewMetadata(Manager, "gateway", m, gw, gw.Name)
}

// Dump k8s resource
func (h *gatewayHandler) Dump(obj interface{}) string {
	gw := obj.(*models.Gateway)
	return fmt.Sprintf("gateway{Namespace: %s, Name: %s}", gw.Namespace, gw.Name)
}

func newGatewayProbe(c interface{}, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(c.(*client.Clientset).NetworkingV1alpha3().RESTClient(), &models.Gateway{}, "gateways", g, &gatewayHandler{})
}

func gatewayVirtualServiceAreLinked(a, b interface{}) bool {
	gateway := a.(*models.Gateway)
	vs := b.(*models.VirtualService)

	if !k8s.MatchNamespace(gateway, vs) {
		return false
	}

	for _, g := range vs.Spec.Gateways {
		if g == gateway.Name {
			return true
		}
	}

	return false
}

func newGatewayVirtualServiceLinker(g *graph.Graph) probe.Handler {
	return k8s.NewABLinker(g, Manager, "gateway", Manager, "virtualservice", gatewayVirtualServiceAreLinked)
}
