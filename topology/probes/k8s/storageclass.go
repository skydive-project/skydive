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

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes"
)

type storageClassHandler struct {
}

func (h *storageClassHandler) Dump(obj interface{}) string {
	sc := obj.(*v1.StorageClass)
	return fmt.Sprintf("storageclass{Namespace: %s, Name: %s}", sc.Namespace, sc.Name)
}

func (h *storageClassHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	sc := obj.(*v1.StorageClass)

	m := NewMetadataFields(&sc.ObjectMeta)
	m.SetField("Provisioner", sc.Provisioner)

	return graph.Identifier(sc.GetUID()), NewMetadata(Manager, "storageclass", m, sc, sc.Name)
}

func newStorageClassProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).StorageV1().RESTClient(), &v1.StorageClass{}, "storageclasses", g, &storageClassHandler{})
}

func storageClassPVCAreLinked(a, b interface{}) bool {
	sc := a.(*v1.StorageClass)
	pvc := b.(*corev1.PersistentVolumeClaim)
	return pvc.Spec.StorageClassName != nil && sc.Name == *pvc.Spec.StorageClassName
}

func newStorageClassPVCLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "storageclass", Manager, "persistentvolumeclaim", storageClassPVCAreLinked)
}

func storageClassPVAreLinked(a, b interface{}) bool {
	sc := a.(*v1.StorageClass)
	pv := b.(*corev1.PersistentVolume)
	return pv.Spec.StorageClassName == sc.Name
}

func newStorageClassPVLinker(g *graph.Graph) probe.Handler {
	return NewABLinker(g, Manager, "storageclass", Manager, "persistentvolume", storageClassPVAreLinked)
}
