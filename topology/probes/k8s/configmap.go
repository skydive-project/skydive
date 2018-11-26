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

type configMapHandler struct {
}

func (h *configMapHandler) Dump(obj interface{}) string {
	cm := obj.(*v1.ConfigMap)
	return fmt.Sprintf("configmap{Namespace: %s, Name: %s}", cm.Namespace, cm.Name)
}

func (h *configMapHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	cm := obj.(*v1.ConfigMap)
	inner := new(MetadataInner).Setup(&cm.ObjectMeta, cm)
	return graph.Identifier(cm.GetUID()), NewMetadata(Manager, "configmap", inner.Name, inner)
}

func newConfigMapProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerDecoder, "configmap")
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.ConfigMap{}, "configmaps", g, &configMapHandler{})
}
