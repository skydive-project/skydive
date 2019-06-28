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

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type endpointsHandler struct {
}

func (h *endpointsHandler) Dump(obj interface{}) string {
	endpoints := obj.(*v1.Endpoints)
	return fmt.Sprintf("endpoints{Namespace: %s, Name: %s}", endpoints.Namespace, endpoints.Name)
}

func (h *endpointsHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	endpoints := obj.(*v1.Endpoints)
	m := NewMetadataFields(&endpoints.ObjectMeta)
	return graph.Identifier(endpoints.GetUID()), NewMetadata(Manager, "endpoints", m, endpoints, endpoints.Name)
}

func newEndpointsProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.Endpoints{}, "endpoints", g, &endpointsHandler{})
}
