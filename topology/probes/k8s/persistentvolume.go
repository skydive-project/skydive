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
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func dumpPersistentVolume(pv *v1.PersistentVolume) string {
	return fmt.Sprintf("persistentvolume{Name: %s}", pv.GetName())
}

func (p *persistentVolumeProbe) newMetadata(pv *v1.PersistentVolume) graph.Metadata {
	return newMetadata("persistentvolume", pv.Namespace, pv.GetName(), pv)
}

func persistentVolumeUID(pv *v1.PersistentVolume) graph.Identifier {
	return graph.Identifier(pv.GetUID())
}

func (p *persistentVolumeProbe) OnAdd(obj interface{}) {
	if pv, ok := obj.(*v1.PersistentVolume); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		newNode(p.graph, persistentVolumeUID(pv), p.newMetadata(pv))
		logging.GetLogger().Debugf("Added %s", dumpPersistentVolume(pv))
	}
}

func (p *persistentVolumeProbe) OnUpdate(oldObj, newObj interface{}) {
	if pv, ok := newObj.(*v1.PersistentVolume); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(persistentVolumeUID(pv)); node != nil {
			addMetadata(p.graph, node, pv)
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
	p.kubeCache.Start()
}

func (p *persistentVolumeProbe) Stop() {
	p.kubeCache.Stop()
}

func newPersistentVolumeKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().CoreV1().RESTClient(), &v1.PersistentVolume{}, "persistentvolumes", handler)
}

func newPersistentVolumeProbe(g *graph.Graph) probe.Probe {
	p := &persistentVolumeProbe{
		graph: g,
	}
	p.kubeCache = newPersistentVolumeKubeCache(p)
	return p
}
