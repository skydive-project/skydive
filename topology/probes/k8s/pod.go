/*
 * Copyright (C) 2017 Red Hat, Inc.
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

	"github.com/mohae/deepcopy"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerPod contains the type specific fields
// easyjson:json
type MetadataInnerPod struct {
	MetadataInner
	Node   string `skydive:"string"`
	IP     string `skydive:"string"`
	Status string `skydive:"string"`
}

// GetField implements Getter interface
func (inner *MetadataInnerPod) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerPod) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerPod) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerPod) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerPodDecoder implements a json message raw decoder
func MetadataInnerPodDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerPod
	return GenericMetadataDecoder(&inner, raw)
}

type podHandler struct {
	graph.DefaultGraphListener
	graph *graph.Graph
	cache *ResourceCache
}

func (h *podHandler) Dump(obj interface{}) string {
	pod := obj.(*v1.Pod)
	return fmt.Sprintf("pod{Namespace: %s, Name: %s}", pod.Namespace, pod.Name)
}

func (h *podHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	// we make a copy of pod before modifaying the pod object so
	// that don't interfere with the container subprobe
	pod := deepcopy.Copy(obj).(*v1.Pod)

	pod.Spec.Containers = nil

	inner := new(MetadataInnerPod)
	inner.MetadataInner.Setup(&pod.ObjectMeta, pod)
	inner.Node = pod.Spec.NodeName
	inner.IP = pod.Status.PodIP
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}
	inner.Status = reason

	metadata := NewMetadata(Manager, "pod", inner.Name, inner)
	SetState(&metadata, reason == "Running")

	return graph.Identifier(pod.GetUID()), metadata
}

func newPodProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerPodDecoder, "pod")
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.Pod{}, "pods", g, &podHandler{graph: g})
}

func podPVCAreLinked(a, b interface{}) bool {
	pod := a.(*v1.Pod)
	pvc := b.(*v1.PersistentVolumeClaim)

	if !MatchNamespace(pod, pvc) {
		return false
	}

	for _, vol := range pod.Spec.Volumes {
		if vol.VolumeSource.PersistentVolumeClaim != nil && vol.VolumeSource.PersistentVolumeClaim.ClaimName == pvc.Name {
			return true
		}
	}
	return false
}

func newPodPVCLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "pod", Manager, "persistentvolumeclaim", podPVCAreLinked)
}
