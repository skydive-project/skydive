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
	models "istio.io/client-go/pkg/apis/networking/v1alpha3"
	client "istio.io/client-go/pkg/clientset/versioned"
)

type serviceEntryHandler struct {
}

// Map graph node to k8s resource
func (h *serviceEntryHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	se := obj.(*models.ServiceEntry)
	m := k8s.NewMetadataFields(&se.ObjectMeta)
	return graph.Identifier(se.GetUID()), k8s.NewMetadata(Manager, "serviceentry", m, se, se.Name)
}

// Dump k8s resource
func (h *serviceEntryHandler) Dump(obj interface{}) string {
	se := obj.(*models.ServiceEntry)
	return fmt.Sprintf("serviceentry{Namespace: %s, Name: %s}", se.Namespace, se.Name)
}

func newServiceEntryProbe(c interface{}, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(c.(*client.Clientset).NetworkingV1alpha3().RESTClient(), &models.ServiceEntry{}, "serviceentries", g, &serviceEntryHandler{})
}
