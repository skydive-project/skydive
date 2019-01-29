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

type quotaSpecHandler struct {
}

// Map graph node to k8s resource
func (h *quotaSpecHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	qs := obj.(*kiali.QuotaSpec)
	m := k8s.NewMetadataFields(&qs.ObjectMeta)
	return graph.Identifier(qs.GetUID()), k8s.NewMetadata(Manager, "quotaspec", m, qs, qs.Name)
}

// Dump k8s resource
func (h *quotaSpecHandler) Dump(obj interface{}) string {
	qs := obj.(*kiali.QuotaSpec)
	return fmt.Sprintf("quotaspec{Namespace: %s, Name: %s}", qs.Namespace, qs.Name)
}

func newQuotaSpecProbe(client interface{}, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(client.(*kiali.IstioClient).GetIstioConfigApi(), &kiali.QuotaSpec{}, "quotaspecs", g, &quotaSpecHandler{})
}
