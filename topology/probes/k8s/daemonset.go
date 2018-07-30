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

type dsProbe struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func dumpDaemonSet(ds *v1beta1.DaemonSet) string {
	return fmt.Sprintf("daemonset{Namespace: %s, Name: %s}", ds.Namespace, ds.Name)
}

func (p *dsProbe) newMetadata(ds *v1beta1.DaemonSet) graph.Metadata {
	m := newMetadata("daemonset", ds.Namespace, ds.Name, ds)
	m.SetFieldAndNormalize("Labels", ds.Labels)
	m.SetField("DesiredNumberScheduled", ds.Status.DesiredNumberScheduled)
	m.SetField("CurrentNumberScheduled", ds.Status.CurrentNumberScheduled)
	m.SetField("NumberMisscheduled", ds.Status.NumberMisscheduled)
	return m
}

func dsUID(ds *v1beta1.DaemonSet) graph.Identifier {
	return graph.Identifier(ds.GetUID())
}

func (p *dsProbe) OnAdd(obj interface{}) {
	if ds, ok := obj.(*v1beta1.DaemonSet); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		newNode(p.graph, dsUID(ds), p.newMetadata(ds))
		logging.GetLogger().Debugf("Added %s", dumpDaemonSet(ds))
	}
}

func (p *dsProbe) OnUpdate(oldObj, newObj interface{}) {
	if ds, ok := newObj.(*v1beta1.DaemonSet); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(dsUID(ds)); nsNode != nil {
			addMetadata(p.graph, nsNode, ds)
			logging.GetLogger().Debugf("Updated %s", dumpDaemonSet(ds))
		}
	}
}

func (p *dsProbe) OnDelete(obj interface{}) {
	if ds, ok := obj.(*v1beta1.DaemonSet); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(dsUID(ds)); nsNode != nil {
			p.graph.DelNode(nsNode)
			logging.GetLogger().Debugf("Deleted %s", dumpDaemonSet(ds))
		}
	}
}

func (p *dsProbe) Start() {
	p.kubeCache.Start()
}

func (p *dsProbe) Stop() {
	p.kubeCache.Stop()
}

func newDaemonSetKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().ExtensionsV1beta1().RESTClient(), &v1beta1.DaemonSet{}, "daemonsets", handler)
}

func newDaemonSetProbe(g *graph.Graph) probe.Probe {
	p := &dsProbe{
		graph: g,
	}
	p.kubeCache = newDaemonSetKubeCache(p)
	return p
}
