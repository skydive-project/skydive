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
	"github.com/skydive-project/skydive/probe"

	"github.com/skydive-project/skydive/graffiti/graph"

	v1 "k8s.io/api/core/v1"
	v1apps "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
)

type replicaSetHandler struct {
}

func (h *replicaSetHandler) Dump(obj interface{}) string {
	rs := obj.(*v1apps.ReplicaSet)
	return fmt.Sprintf("replicaset{Name: %s}", rs.GetName())
}

func (h *replicaSetHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	rs := obj.(*v1apps.ReplicaSet)
	m := NewMetadataFields(&rs.ObjectMeta)
	return graph.Identifier(rs.GetUID()), NewMetadata(Manager, "replicaset", m, rs, rs.Name)
}

func newReplicaSetProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).AppsV1().RESTClient(), &v1apps.ReplicaSet{}, "replicasets", g, &replicaSetHandler{})
}

func replicaSetPodAreLinked(a, b interface{}) bool {
	rc := a.(*v1apps.ReplicaSet)
	pod := b.(*v1.Pod)
	return MatchNamespace(pod, rc) && matchLabelSelector(pod, rc.Spec.Selector)
}

func newReplicaSetPodLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "replicaset", Manager, "pod", replicaSetPodAreLinked)
}
