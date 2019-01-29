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

type replicationControllerHandler struct {
}

func (h *replicationControllerHandler) Dump(obj interface{}) string {
	rc := obj.(*v1.ReplicationController)
	return fmt.Sprintf("replicationController{Name: %s}", rc.GetName())
}

func (h *replicationControllerHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	rc := obj.(*v1.ReplicationController)
	m := NewMetadataFields(&rc.ObjectMeta)
	return graph.Identifier(rc.GetUID()), NewMetadata(Manager, "replicationcontroller", m, rc, rc.Name)
}

func newReplicationControllerProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.ReplicationController{}, "replicationcontrollers", g, &replicationControllerHandler{})
}
