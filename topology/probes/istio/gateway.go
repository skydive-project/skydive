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
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/k8s"
)

type gatewayHandler struct {
}

// Map graph node to k8s resource
func (h *gatewayHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	gw := obj.(*kiali.Gateway)
	m := k8s.NewMetadataFields(&gw.ObjectMeta)
	return graph.Identifier(gw.GetUID()), k8s.NewMetadata(Manager, "gateway", m, gw, gw.Name)
}

// Dump k8s resource
func (h *gatewayHandler) Dump(obj interface{}) string {
	gw := obj.(*kiali.Gateway)
	return fmt.Sprintf("gateway{Namespace: %s, Name: %s}", gw.Namespace, gw.Name)
}

func newGatewayProbe(client interface{}, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(client.(*kiali.IstioClient).GetIstioNetworkingApi(), &kiali.Gateway{}, "gateways", g, &gatewayHandler{})
}
