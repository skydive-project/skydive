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

type serviceProbe struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func dumpService(srv *v1.Service) string {
	return fmt.Sprintf("service{Name: %s}", srv.GetName())
}

func (p *serviceProbe) newMetadata(srv *v1.Service) graph.Metadata {
	return newMetadata("service", srv.Namespace, srv.GetName(), srv)
}

func serviceUID(srv *v1.Service) graph.Identifier {
	return graph.Identifier(srv.GetUID())
}

func (p *serviceProbe) OnAdd(obj interface{}) {
	if srv, ok := obj.(*v1.Service); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		newNode(p.graph, serviceUID(srv), p.newMetadata(srv))
		logging.GetLogger().Debugf("Added %s", dumpService(srv))
	}
}

func (p *serviceProbe) OnUpdate(oldObj, newObj interface{}) {
	if srv, ok := newObj.(*v1.Service); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(serviceUID(srv)); nsNode != nil {
			addMetadata(p.graph, nsNode, srv)
			logging.GetLogger().Debugf("Updated %s", dumpService(srv))
		}
	}
}

func (p *serviceProbe) OnDelete(obj interface{}) {
	if srv, ok := obj.(*v1.Service); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(serviceUID(srv)); nsNode != nil {
			p.graph.DelNode(nsNode)
			logging.GetLogger().Debugf("Deleted %s", dumpService(srv))
		}
	}
}

func (p *serviceProbe) Start() {
	p.kubeCache.Start()
}

func (p *serviceProbe) Stop() {
	p.kubeCache.Stop()
}

func newServiceKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().Core().RESTClient(), &v1.Service{}, "services", handler)
}

func newServiceProbe(g *graph.Graph) probe.Probe {
	p := &serviceProbe{
		graph: g,
	}
	p.kubeCache = newServiceKubeCache(p)
	return p
}
