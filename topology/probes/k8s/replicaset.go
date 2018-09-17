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

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

type replicaSetProbe struct {
	DefaultKubeCacheEventHandler
	*KubeCache
	graph *graph.Graph
}

func dumpReplicaSet(rs *v1beta1.ReplicaSet) string {
	return fmt.Sprintf("replicaset{Name: %s}", rs.GetName())
}

func (p *replicaSetProbe) newMetadata(rs *v1beta1.ReplicaSet) graph.Metadata {
	return NewMetadata(Manager, "replicaset", rs.Namespace, rs.GetName(), rs)
}

func replicaSetUID(rs *v1beta1.ReplicaSet) graph.Identifier {
	return graph.Identifier(rs.GetUID())
}

func (p *replicaSetProbe) OnAdd(obj interface{}) {
	if rs, ok := obj.(*v1beta1.ReplicaSet); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		NewNode(p.graph, replicaSetUID(rs), p.newMetadata(rs))
		logging.GetLogger().Debugf("Added %s", dumpReplicaSet(rs))
	}
}

func (p *replicaSetProbe) OnUpdate(oldObj, newObj interface{}) {
	if rs, ok := newObj.(*v1beta1.ReplicaSet); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(replicaSetUID(rs)); node != nil {
			AddMetadata(p.graph, node, rs)
			logging.GetLogger().Debugf("Updated %s", dumpReplicaSet(rs))
		}
	}
}

func (p *replicaSetProbe) OnDelete(obj interface{}) {
	if rs, ok := obj.(*v1beta1.ReplicaSet); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(replicaSetUID(rs)); node != nil {
			p.graph.DelNode(node)
			logging.GetLogger().Debugf("Deleted %s", dumpReplicaSet(rs))
		}
	}
}

func (p *replicaSetProbe) Start() {
	p.KubeCache.Start()
}

func (p *replicaSetProbe) Stop() {
	p.KubeCache.Stop()
}

func newReplicaSetKubeCache(handler cache.ResourceEventHandler) *KubeCache {
	return NewKubeCache(getClientset().ExtensionsV1beta1().RESTClient(), &v1beta1.ReplicaSet{}, "replicasets", handler)
}

func newReplicaSetProbe(g *graph.Graph) probe.Probe {
	p := &replicaSetProbe{
		graph: g,
	}
	p.KubeCache = newReplicaSetKubeCache(p)
	return p
}
