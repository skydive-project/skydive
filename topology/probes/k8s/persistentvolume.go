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

type persistentVolumeProbe struct {
	DefaultKubeCacheEventHandler
	*KubeCache
	graph *graph.Graph
}

func dumpPersistentVolume(pv *v1.PersistentVolume) string {
	return fmt.Sprintf("persistentvolume{Name: %s}", pv.Name)
}

func persistentVolumeAccessModes(accessModes []v1.PersistentVolumeAccessMode) string {
	if len(accessModes) > 0 {
		return string(accessModes[0])
	}
	return ""
}

func persistentVolumeClaimRef(claimRef *v1.ObjectReference) string {
	if claimRef != nil {
		return claimRef.Name
	}
	return ""
}

func (p *persistentVolumeProbe) newMetadata(pv *v1.PersistentVolume) graph.Metadata {
	m := NewMetadata(Manager, "persistentvolume", pv.Namespace, pv.Name, pv)
	m.SetFieldAndNormalize("Capacity", pv.Spec.Capacity)

	m.SetFieldAndNormalize("AccessModes", persistentVolumeAccessModes(pv.Spec.AccessModes))
	m.SetFieldAndNormalize("VolumeMode", pv.Spec.VolumeMode)
	m.SetFieldAndNormalize("ClaimRef", persistentVolumeClaimRef(pv.Spec.ClaimRef)) // FIXME: replace by link to PersistentVolumeClaim
	m.SetFieldAndNormalize("StorageClassName", pv.Spec.StorageClassName)           // FIXME: replace by link to StorageClass
	m.SetFieldAndNormalize("Status", pv.Status.Phase)
	return m
}

func persistentVolumeUID(pv *v1.PersistentVolume) graph.Identifier {
	return graph.Identifier(pv.GetUID())
}

func (p *persistentVolumeProbe) OnAdd(obj interface{}) {
	if pv, ok := obj.(*v1.PersistentVolume); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		NewNode(p.graph, persistentVolumeUID(pv), p.newMetadata(pv))
		logging.GetLogger().Debugf("Added %s", dumpPersistentVolume(pv))
	}
}

func (p *persistentVolumeProbe) OnUpdate(oldObj, newObj interface{}) {
	if pv, ok := newObj.(*v1.PersistentVolume); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(persistentVolumeUID(pv)); node != nil {
			AddMetadata(p.graph, node, pv)
			logging.GetLogger().Debugf("Updated %s", dumpPersistentVolume(pv))
		}
	}
}

func (p *persistentVolumeProbe) OnDelete(obj interface{}) {
	if pv, ok := obj.(*v1.PersistentVolume); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(persistentVolumeUID(pv)); node != nil {
			p.graph.DelNode(node)
			logging.GetLogger().Debugf("Deleted %s", dumpPersistentVolume(pv))
		}
	}
}

func (p *persistentVolumeProbe) Start() {
	p.KubeCache.Start()
}

func (p *persistentVolumeProbe) Stop() {
	p.KubeCache.Stop()
}

func newPersistentVolumeKubeCache(handler cache.ResourceEventHandler) *KubeCache {
	return NewKubeCache(getClientset().CoreV1().RESTClient(), &v1.PersistentVolume{}, "persistentvolumes", handler)
}

func newPersistentVolumeProbe(g *graph.Graph) probe.Probe {
	p := &persistentVolumeProbe{
		graph: g,
	}
	p.KubeCache = newPersistentVolumeKubeCache(p)
	return p
}
