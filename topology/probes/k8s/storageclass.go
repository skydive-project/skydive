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

	"k8s.io/api/storage/v1"
	"k8s.io/client-go/tools/cache"
)

type storageClassProbe struct {
	DefaultKubeCacheEventHandler
	*KubeCache
	graph *graph.Graph
}

func dumpStorageClass(sc *v1.StorageClass) string {
	return fmt.Sprintf("storageclass{Namespace: %s, Name: %s}", sc.Namespace, sc.Name)
}

func (p *storageClassProbe) newMetadata(sc *v1.StorageClass) graph.Metadata {
	m := NewMetadata(Manager, "storageclass", sc.Namespace, sc.Name, sc)
	m.SetField("Provisioner", sc.Provisioner)
	return m
}

func storageClassUID(sc *v1.StorageClass) graph.Identifier {
	return graph.Identifier(sc.GetUID())
}

func (p *storageClassProbe) OnAdd(obj interface{}) {
	if sc, ok := obj.(*v1.StorageClass); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		NewNode(p.graph, storageClassUID(sc), p.newMetadata(sc))
		logging.GetLogger().Debugf("Added %s", dumpStorageClass(sc))
	}
}

func (p *storageClassProbe) OnUpdate(oldObj, newObj interface{}) {
	if sc, ok := newObj.(*v1.StorageClass); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(storageClassUID(sc)); node != nil {
			AddMetadata(p.graph, node, sc)
			logging.GetLogger().Debugf("Updated %s", dumpStorageClass(sc))
		}
	}
}

func (p *storageClassProbe) OnDelete(obj interface{}) {
	if sc, ok := obj.(*v1.StorageClass); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(storageClassUID(sc)); node != nil {
			p.graph.DelNode(node)
			logging.GetLogger().Debugf("Deleted %s", dumpStorageClass(sc))
		}
	}
}

func (p *storageClassProbe) Start() {
	p.KubeCache.Start()
}

func (p *storageClassProbe) Stop() {
	p.KubeCache.Stop()
}

func newStorageClassKubeCache(handler cache.ResourceEventHandler) *KubeCache {
	return NewKubeCache(getClientset().StorageV1().RESTClient(), &v1.StorageClass{}, "storageclasses", handler)
}

func newStorageClassProbe(g *graph.Graph) probe.Probe {
	p := &storageClassProbe{
		graph: g,
	}
	p.KubeCache = newStorageClassKubeCache(p)
	return p
}
