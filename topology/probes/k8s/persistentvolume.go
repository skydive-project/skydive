/*
 * Copyright (C) 2018 IBM, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package k8s

import (
	"fmt"

	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type persistentVolumeHandler struct {
	DefaultResourceHandler
}

func (h *persistentVolumeHandler) IsTopLevel() bool {
	return true
}

func (h *persistentVolumeHandler) Dump(obj interface{}) string {
	pv := obj.(*v1.PersistentVolume)
	return fmt.Sprintf("persistentvolume{Name: %s}", pv.Name)
}

func (h *persistentVolumeHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	pv := obj.(*v1.PersistentVolume)
	m := NewMetadata(Manager, "persistentvolume", pv, pv.Name, pv.Namespace)
	m.SetFieldAndNormalize("Capacity", pv.Spec.Capacity)
	m.SetFieldAndNormalize("VolumeMode", pv.Spec.VolumeMode)
	m.SetFieldAndNormalize("StorageClassName", pv.Spec.StorageClassName) // FIXME: replace by link to StorageClass
	m.SetFieldAndNormalize("Status", pv.Status.Phase)
	m.SetFieldAndNormalize("AccessModes", pv.Spec.AccessModes)

	if pv.Spec.ClaimRef != nil {
		m.SetFieldAndNormalize("ClaimRef", pv.Spec.ClaimRef.Name) // FIXME: replace by link to PersistentVolumeClaim
	}

	return graph.Identifier(pv.GetUID()), m
}

func newPersistentVolumeProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return NewResourceCache(clientset.CoreV1().RESTClient(), &v1.PersistentVolume{}, "persistentvolumes", g, &persistentVolumeHandler{})
}
