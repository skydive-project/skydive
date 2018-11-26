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
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerStatfulSet contains the type specific fields
// easyjson:json
type MetadataInnerStatfulSet struct {
	MetadataInner
	DesiredReplicas int32  `skydive:"int"`
	ServiceName     string `skydive:"string"`
	Replicas        int32  `skydive:"int"`
	ReadyReplicas   int32  `skydive:"int"`
	CurrentReplicas int32  `skydive:"int"`
	UpdatedReplicas int32  `skydive:"int"`
	CurrentRevision string `skydive:"string"`
	UpdateRevision  string `skydive:"string"`
}

// GetField implements Getter interface
func (inner *MetadataInnerStatfulSet) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerStatfulSet) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerStatfulSet) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerStatfulSet) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerStatfulSetDecoder implements a json message raw decoder
func MetadataInnerStatfulSetDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerStatfulSet
	return GenericMetadataDecoder(&inner, raw)
}

type statefulSetHandler struct {
}

func (h *statefulSetHandler) Dump(obj interface{}) string {
	ss := obj.(*v1beta1.StatefulSet)
	return fmt.Sprintf("statefulset{Namespace: %s, Name: %s}", ss.Namespace, ss.Name)
}

func (h *statefulSetHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	ss := obj.(*v1beta1.StatefulSet)

	inner := new(MetadataInnerStatfulSet)
	inner.MetadataInner.Setup(&ss.ObjectMeta, ss)
	inner.DesiredReplicas = int32ValueOrDefault(ss.Spec.Replicas, 1)
	inner.ServiceName = ss.Spec.ServiceName
	inner.Replicas = ss.Status.Replicas
	inner.ReadyReplicas = ss.Status.ReadyReplicas
	inner.CurrentReplicas = ss.Status.CurrentReplicas
	inner.UpdatedReplicas = ss.Status.UpdatedReplicas
	inner.CurrentRevision = ss.Status.CurrentRevision
	inner.UpdateRevision = ss.Status.UpdateRevision

	return graph.Identifier(ss.GetUID()), NewMetadata(Manager, "statefulset", inner.Name, inner)
}

func newStatefulSetProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerStatfulSetDecoder, "statefulset")
	return NewResourceCache(client.(*kubernetes.Clientset).AppsV1beta1().RESTClient(), &v1beta1.StatefulSet{}, "statefulsets", g, &statefulSetHandler{})
}

func statefulSetPodAreLinked(a, b interface{}) bool {
	statefulset := a.(*v1beta1.StatefulSet)
	pod := b.(*v1.Pod)
	return MatchNamespace(pod, statefulset) && matchLabelSelector(pod, statefulset.Spec.Selector)
}

func newStatefulSetPodLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "statefulset", Manager, "pod", statefulSetPodAreLinked)
}
