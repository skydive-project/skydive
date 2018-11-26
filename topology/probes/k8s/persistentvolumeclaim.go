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

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerPersistentVolumeClaim contains the type specific fields
// easyjson:json
type MetadataInnerPersistentVolumeClaim struct {
	MetadataInner
	AccessModes      []v1.PersistentVolumeAccessMode
	VolumeName       string                   `skydive:"string"`
	StorageClassName string                   `skydive:"string"`
	VolumeMode       *v1.PersistentVolumeMode `skydive:"string"`
	Status           string                   `skydive:"string"`
}

// GetField implements Getter interface
func (inner *MetadataInnerPersistentVolumeClaim) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerPersistentVolumeClaim) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerPersistentVolumeClaim) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerPersistentVolumeClaim) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerPersistentVolumeClaimDecoder implements a json message raw decoder
func MetadataInnerPersistentVolumeClaimDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerPersistentVolumeClaim
	return GenericMetadataDecoder(&inner, raw)
}

type persistentVolumeClaimHandler struct {
}

func (h *persistentVolumeClaimHandler) Dump(obj interface{}) string {
	pvc := obj.(*v1.PersistentVolumeClaim)
	return fmt.Sprintf("persistentvolumeclaim{Name: %s}", pvc.GetName())
}

func (h *persistentVolumeClaimHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	pvc := obj.(*v1.PersistentVolumeClaim)

	inner := new(MetadataInnerPersistentVolumeClaim)
	inner.MetadataInner.Setup(&pvc.ObjectMeta, pvc)
	inner.AccessModes = pvc.Spec.AccessModes
	inner.VolumeName = pvc.Spec.VolumeName
	inner.StorageClassName = *pvc.Spec.StorageClassName
	inner.VolumeMode = pvc.Spec.VolumeMode
	inner.Status = string(pvc.Status.Phase)

	metadata := NewMetadata(Manager, "persistentvolumeclaim", inner.Name, inner)
	SetState(&metadata, pvc.Status.Phase == "Bound")

	return graph.Identifier(pvc.GetUID()), metadata
}

func newPersistentVolumeClaimProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerPersistentVolumeClaimDecoder, "persistentvolumeclaim")
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.PersistentVolumeClaim{}, "persistentvolumeclaims", g, &persistentVolumeClaimHandler{})
}

func pvPVCAreLinked(a, b interface{}) bool {
	pvc := a.(*v1.PersistentVolumeClaim)
	pv := b.(*v1.PersistentVolume)
	return pvc.Spec.VolumeName == pv.Name
}

func newPVPVCLinker(g *graph.Graph) probe.Probe {
	return NewABLinker(g, Manager, "persistentvolumeclaim", Manager, "persistentvolume", pvPVCAreLinked)
}
