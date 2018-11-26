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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerStorageClass contains the type specific fields
// easyjson:json
type MetadataInnerStorageClass struct {
	MetadataInner
	Provisioner string `skydive:"string"`
}

// GetField implements Getter interface
func (inner *MetadataInnerStorageClass) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerStorageClass) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerStorageClass) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerStorageClass) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerStorageClassDecoder implements a json message raw decoder
func MetadataInnerStorageClassDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerStorageClass
	return GenericMetadataDecoder(&inner, raw)
}

type storageClassHandler struct {
}

func (h *storageClassHandler) Dump(obj interface{}) string {
	sc := obj.(*v1.StorageClass)
	return fmt.Sprintf("storageclass{Namespace: %s, Name: %s}", sc.Namespace, sc.Name)
}

func (h *storageClassHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	sc := obj.(*v1.StorageClass)

	inner := new(MetadataInnerStorageClass)
	inner.MetadataInner.Setup(&sc.ObjectMeta, sc)
	inner.Provisioner = sc.Provisioner

	return graph.Identifier(sc.GetUID()), NewMetadata(Manager, "storageclass", inner.Name, inner)
}

func newStorageClassProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerStorageClassDecoder, "storageclass")
	return NewResourceCache(client.(*kubernetes.Clientset).StorageV1().RESTClient(), &v1.StorageClass{}, "storageclasses", g, &storageClassHandler{})
}

func storageClassPVCAreLinked(a, b interface{}) bool {
	sc := a.(*v1.StorageClass)
	pvc := b.(*corev1.PersistentVolumeClaim)
	return pvc.Spec.StorageClassName != nil && sc.Name == *pvc.Spec.StorageClassName
}

func newStorageClassPVCLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "storageclass", Manager, "persistentvolumeclaim", storageClassPVCAreLinked)
}

func storageClassPVAreLinked(a, b interface{}) bool {
	sc := a.(*v1.StorageClass)
	pv := b.(*corev1.PersistentVolume)
	return pv.Spec.StorageClassName == sc.Name
}

func newStorageClassPVLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "storageclass", Manager, "persistentvolume", storageClassPVAreLinked)
}
