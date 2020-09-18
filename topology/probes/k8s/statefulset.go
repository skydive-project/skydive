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
	"github.com/skydive-project/skydive/probe"

	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type statefulSetHandler struct {
}

func (h *statefulSetHandler) Dump(obj interface{}) string {
	ss := obj.(*v1apps.StatefulSet)
	return fmt.Sprintf("statefulset{Namespace: %s, Name: %s}", ss.Namespace, ss.Name)
}

func (h *statefulSetHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	ss := obj.(*v1apps.StatefulSet)

	m := NewMetadataFields(&ss.ObjectMeta)
	m.SetField("DesiredReplicas", int32ValueOrDefault(ss.Spec.Replicas, 1))
	m.SetField("ServiceName", ss.Spec.ServiceName) // FIXME: replace by link to Service
	m.SetField("Replicas", ss.Status.Replicas)
	m.SetField("ReadyReplicas", ss.Status.ReadyReplicas)
	m.SetField("CurrentReplicas", ss.Status.CurrentReplicas)
	m.SetField("UpdatedReplicas", ss.Status.UpdatedReplicas)
	m.SetField("CurrentRevision", ss.Status.CurrentRevision)
	m.SetField("UpdateRevision", ss.Status.UpdateRevision)

	return graph.Identifier(ss.GetUID()), NewMetadata(Manager, "statefulset", m, ss, ss.Name)
}

func newStatefulSetProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).AppsV1().RESTClient(), &v1apps.StatefulSet{}, "statefulsets", g, &statefulSetHandler{})
}

func statefulSetPodAreLinked(a, b interface{}) bool {
	statefulset := a.(*v1apps.StatefulSet)
	pod := b.(*v1.Pod)
	return MatchNamespace(pod, statefulset) && matchLabelSelector(pod, statefulset.Spec.Selector)
}

func newStatefulSetPodLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "statefulset", Manager, "pod", statefulSetPodAreLinked)
}
