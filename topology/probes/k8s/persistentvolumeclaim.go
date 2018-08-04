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

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type persistentVolumeClaimProbe struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func dumpPersistentVolumeClaim(pvc *v1.PersistentVolumeClaim) string {
	return fmt.Sprintf("persistentvolumeclaim{Name: %s}", pvc.GetName())
}

func (p *persistentVolumeClaimProbe) newMetadata(pvc *v1.PersistentVolumeClaim) graph.Metadata {
	m := newMetadata("persistentvolumeclaim", pvc.Namespace, pvc.GetName(), pvc)
	m.SetFieldAndNormalize("AccessModes", persistentVolumeAccessModes(pvc.Spec.AccessModes))
	m.SetFieldAndNormalize("VolumeName", pvc.Spec.VolumeName)             // FIXME: replace by link to PersistentVolume
	m.SetFieldAndNormalize("StorageClassName", pvc.Spec.StorageClassName) // FIXME: replace by link to StorageClass
	m.SetFieldAndNormalize("VolumeMode", pvc.Spec.VolumeMode)
	m.SetFieldAndNormalize("Status", pvc.Status.Phase)
	return m
}

func persistentVolumeClaimUID(pvc *v1.PersistentVolumeClaim) graph.Identifier {
	return graph.Identifier(pvc.GetUID())
}

func (p *persistentVolumeClaimProbe) OnAdd(obj interface{}) {
	if pvc, ok := obj.(*v1.PersistentVolumeClaim); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		newNode(p.graph, persistentVolumeClaimUID(pvc), p.newMetadata(pvc))
		logging.GetLogger().Debugf("Added %s", dumpPersistentVolumeClaim(pvc))
	}
}

func (p *persistentVolumeClaimProbe) OnUpdate(oldObj, newObj interface{}) {
	if pvc, ok := newObj.(*v1.PersistentVolumeClaim); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(persistentVolumeClaimUID(pvc)); node != nil {
			addMetadata(p.graph, node, pvc)
			logging.GetLogger().Debugf("Updated %s", dumpPersistentVolumeClaim(pvc))
		}
	}
}

func (p *persistentVolumeClaimProbe) OnDelete(obj interface{}) {
	if pvc, ok := obj.(*v1.PersistentVolumeClaim); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(persistentVolumeClaimUID(pvc)); node != nil {
			p.graph.DelNode(node)
			logging.GetLogger().Debugf("Deleted %s", dumpPersistentVolumeClaim(pvc))
		}
	}
}

func (p *persistentVolumeClaimProbe) Start() {
	p.kubeCache.Start()
}

func (p *persistentVolumeClaimProbe) Stop() {
	p.kubeCache.Stop()
}

func newPersistentVolumeClaimKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().CoreV1().RESTClient(), &v1.PersistentVolumeClaim{}, "persistentvolumeclaims", handler)
}

func newPersistentVolumeClaimProbe(g *graph.Graph) probe.Probe {
	p := &persistentVolumeClaimProbe{
		graph: g,
	}
	p.kubeCache = newPersistentVolumeClaimKubeCache(p)
	return p
}
