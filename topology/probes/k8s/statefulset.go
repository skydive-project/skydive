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

	"k8s.io/api/apps/v1beta1"
	"k8s.io/client-go/tools/cache"
)

type statefulSetProbe struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func dumpStatefulSet(ss *v1beta1.StatefulSet) string {
	return fmt.Sprintf("statefulset{Namespace: %s, Name: %s}", ss.Namespace, ss.Name)
}

func (p *statefulSetProbe) newMetadata(ss *v1beta1.StatefulSet) graph.Metadata {
	m := newMetadata("statefulset", ss.Namespace, ss.Name, ss)
	m.SetField("DesiredReplicas", int32ValueOrDefault(ss.Spec.Replicas, 1))
	m.SetField("ServiceName", ss.Spec.ServiceName) // FIXME: replace by link to Service
	m.SetField("Replicas", ss.Status.Replicas)
	m.SetField("ReadyReplicas", ss.Status.ReadyReplicas)
	m.SetField("CurrentReplicas", ss.Status.CurrentReplicas)
	m.SetField("UpdatedReplicas", ss.Status.UpdatedReplicas)
	m.SetField("CurrentRevision", ss.Status.CurrentRevision)
	m.SetField("UpdateRevision", ss.Status.UpdateRevision)
	return m
}

func statefulSetUID(ss *v1beta1.StatefulSet) graph.Identifier {
	return graph.Identifier(ss.GetUID())
}

func (p *statefulSetProbe) OnAdd(obj interface{}) {
	if ss, ok := obj.(*v1beta1.StatefulSet); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		newNode(p.graph, statefulSetUID(ss), p.newMetadata(ss))
		logging.GetLogger().Debugf("Added %s", dumpStatefulSet(ss))
	}
}

func (p *statefulSetProbe) OnUpdate(oldObj, newObj interface{}) {
	if ss, ok := newObj.(*v1beta1.StatefulSet); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(statefulSetUID(ss)); node != nil {
			addMetadata(p.graph, node, ss)
			logging.GetLogger().Debugf("Updated %s", dumpStatefulSet(ss))
		}
	}
}

func (p *statefulSetProbe) OnDelete(obj interface{}) {
	if ss, ok := obj.(*v1beta1.StatefulSet); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(statefulSetUID(ss)); node != nil {
			p.graph.DelNode(node)
			logging.GetLogger().Debugf("Deleted %s", dumpStatefulSet(ss))
		}
	}
}

func (p *statefulSetProbe) Start() {
	p.kubeCache.Start()
}

func (p *statefulSetProbe) Stop() {
	p.kubeCache.Stop()
}

func newStatefulSetKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().AppsV1beta1().RESTClient(), &v1beta1.StatefulSet{}, "statefulsets", handler)
}

func newStatefulSetProbe(g *graph.Graph) probe.Probe {
	p := &statefulSetProbe{
		graph: g,
	}
	p.kubeCache = newStatefulSetKubeCache(p)
	return p
}
