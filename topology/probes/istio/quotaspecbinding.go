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
	"github.com/skydive-project/skydive/topology/probes/k8s"
	models "istio.io/client-go/pkg/apis/config/v1alpha2"
	client "istio.io/client-go/pkg/clientset/versioned"
)

type quotaSpecBindingHandler struct {
}

// Map graph node to k8s resource
func (h *quotaSpecBindingHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	qsb := obj.(*models.QuotaSpecBinding)
	m := k8s.NewMetadataFields(&qsb.ObjectMeta)
	return graph.Identifier(qsb.GetUID()), k8s.NewMetadata(Manager, "quotaspecbinding", m, qsb, qsb.Name)
}

// Dump k8s resource
func (h *quotaSpecBindingHandler) Dump(obj interface{}) string {
	qsb := obj.(*models.QuotaSpecBinding)
	return fmt.Sprintf("quotaspecbinding{Namespace: %s, Name: %s}", qsb.Namespace, qsb.Name)
}

func newQuotaSpecBindingProbe(c interface{}, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(c.(*client.Clientset).ConfigV1alpha2().RESTClient(), &models.QuotaSpecBinding{}, "quotaspecbindings", g, &quotaSpecBindingHandler{})
}
