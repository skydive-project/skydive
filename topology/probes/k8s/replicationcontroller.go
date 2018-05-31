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

type replicationControllerProbe struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func dumpReplicationController(rc *v1.ReplicationController) string {
	return fmt.Sprintf("replicationController{Name: %s}", rc.GetName())
}

func (p *replicationControllerProbe) newMetadata(rc *v1.ReplicationController) graph.Metadata {
	return newMetadata("replicationcontroller", rc.Namespace, rc.GetName(), rc)
}

func replicationControllerUID(rc *v1.ReplicationController) graph.Identifier {
	return graph.Identifier(rc.GetUID())
}

func (p *replicationControllerProbe) OnAdd(obj interface{}) {
	if rc, ok := obj.(*v1.ReplicationController); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		newNode(p.graph, replicationControllerUID(rc), p.newMetadata(rc))
		logging.GetLogger().Debugf("Added %s", dumpReplicationController(rc))
	}
}

func (p *replicationControllerProbe) OnUpdate(oldObj, newObj interface{}) {
	if rc, ok := newObj.(*v1.ReplicationController); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(replicationControllerUID(rc)); node != nil {
			addMetadata(p.graph, node, rc)
			logging.GetLogger().Debugf("Updated %s", dumpReplicationController(rc))
		}
	}
}

func (p *replicationControllerProbe) OnDelete(obj interface{}) {
	if rc, ok := obj.(*v1.ReplicationController); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(replicationControllerUID(rc)); node != nil {
			p.graph.DelNode(node)
			logging.GetLogger().Debugf("Deleted %s", dumpReplicationController(rc))
		}
	}
}

func (p *replicationControllerProbe) Start() {
	p.kubeCache.Start()
}

func (p *replicationControllerProbe) Stop() {
	p.kubeCache.Stop()
}

func newReplicationControllerKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().CoreV1().RESTClient(), &v1.ReplicationController{}, "replicationcontrollers", handler)
}

func newReplicationControllerProbe(g *graph.Graph) probe.Probe {
	p := &replicationControllerProbe{
		graph: g,
	}
	p.kubeCache = newReplicationControllerKubeCache(p)
	return p
}
