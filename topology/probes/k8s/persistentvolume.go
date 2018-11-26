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

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerPersistentVolume contains the type specific fields
// easyjson:json
type MetadataInnerPersistentVolume struct {
	MetadataInner
	Capacity         v1.ResourceList
	VolumeMode       *v1.PersistentVolumeMode `skydive:"string"`
	StorageClassName string                   `skydive:"string"`
	Status           string                   `skydive:"string"`
	AccessModes      []v1.PersistentVolumeAccessMode
	ClaimRef         string
}

// GetField implements Getter interface
func (inner *MetadataInnerPersistentVolume) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerPersistentVolume) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerPersistentVolume) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerPersistentVolume) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerPersistentVolumeDecoder implements a json message raw decoder
func MetadataInnerPersistentVolumeDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerPersistentVolume
	return GenericMetadataDecoder(&inner, raw)
}

type persistentVolumeHandler struct {
}

func (h *persistentVolumeHandler) Dump(obj interface{}) string {
	pv := obj.(*v1.PersistentVolume)
	return fmt.Sprintf("persistentvolume{Name: %s}", pv.Name)
}

func (h *persistentVolumeHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	pv := obj.(*v1.PersistentVolume)

	inner := new(MetadataInnerPersistentVolume)
	inner.MetadataInner.Setup(&pv.ObjectMeta, pv)
	inner.Capacity = pv.Spec.Capacity
	inner.VolumeMode = pv.Spec.VolumeMode
	inner.StorageClassName = pv.Spec.StorageClassName
	inner.Status = string(pv.Status.Phase)
	inner.AccessModes = pv.Spec.AccessModes
	if pv.Spec.ClaimRef != nil {
		inner.ClaimRef = pv.Spec.ClaimRef.Name
	}

	metadata := NewMetadata(Manager, "persistentvolume", inner.Name, inner)
	SetState(&metadata, pv.Status.Phase != "Failed")

	return graph.Identifier(pv.GetUID()), metadata
}

func newPersistentVolumeProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerPersistentVolumeDecoder, "persistentvolume")
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.PersistentVolume{}, "persistentvolumes", g, &persistentVolumeHandler{})
}
