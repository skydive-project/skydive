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

type endpointsProbe struct {
	DefaultKubeCacheEventHandler
	*KubeCache
	graph *graph.Graph
}

func dumpEndpoints(srv *v1.Endpoints) string {
	return fmt.Sprintf("endpoints{Name: %s}", srv.GetName())
}

func (p *endpointsProbe) newMetadata(srv *v1.Endpoints) graph.Metadata {
	return NewMetadata(Manager, "endpoints", srv.Namespace, srv.GetName(), srv)
}

func endpointsUID(srv *v1.Endpoints) graph.Identifier {
	return graph.Identifier(srv.GetUID())
}

func (p *endpointsProbe) OnAdd(obj interface{}) {
	if srv, ok := obj.(*v1.Endpoints); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		NewNode(p.graph, endpointsUID(srv), p.newMetadata(srv))
		logging.GetLogger().Debugf("Added %s", dumpEndpoints(srv))
	}
}

func (p *endpointsProbe) OnUpdate(oldObj, newObj interface{}) {
	if srv, ok := newObj.(*v1.Endpoints); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(endpointsUID(srv)); nsNode != nil {
			AddMetadata(p.graph, nsNode, srv)
			logging.GetLogger().Debugf("Updated %s", dumpEndpoints(srv))
		}
	}
}

func (p *endpointsProbe) OnDelete(obj interface{}) {
	if srv, ok := obj.(*v1.Endpoints); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(endpointsUID(srv)); nsNode != nil {
			p.graph.DelNode(nsNode)
			logging.GetLogger().Debugf("Deleted %s", dumpEndpoints(srv))
		}
	}
}

func (p *endpointsProbe) Start() {
	p.KubeCache.Start()
}

func (p *endpointsProbe) Stop() {
	p.KubeCache.Stop()
}

func newEndpointsKubeCache(handler cache.ResourceEventHandler) *KubeCache {
	return NewKubeCache(getClientset().Core().RESTClient(), &v1.Endpoints{}, "endpoints", handler)
}

func newEndpointsProbe(g *graph.Graph) probe.Probe {
	p := &endpointsProbe{
		graph: g,
	}
	p.KubeCache = newEndpointsKubeCache(p)
	return p
}
