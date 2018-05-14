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

type ingressProbe struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func dumpIngress(ingress *v1beta1.Ingress) string {
	return fmt.Sprintf("ingress{Name: %s}", ingress.GetName())
}

func (p *ingressProbe) newMetadata(ingress *v1beta1.Ingress) graph.Metadata {
	return newMetadata("ingress", ingress.Namespace, ingress.GetName(), ingress)
}

func ingressUID(ingress *v1beta1.Ingress) graph.Identifier {
	return graph.Identifier(ingress.GetUID())
}

func (p *ingressProbe) OnAdd(obj interface{}) {
	if ingress, ok := obj.(*v1beta1.Ingress); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		newNode(p.graph, ingressUID(ingress), p.newMetadata(ingress))
		logging.GetLogger().Debugf("Added %s", dumpIngress(ingress))
	}
}

func (p *ingressProbe) OnUpdate(oldObj, newObj interface{}) {
	if ingress, ok := newObj.(*v1beta1.Ingress); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(ingressUID(ingress)); nsNode != nil {
			addMetadata(p.graph, nsNode, ingress)
			logging.GetLogger().Debugf("Updated %s", dumpIngress(ingress))
		}
	}
}

func (p *ingressProbe) OnDelete(obj interface{}) {
	if ingress, ok := obj.(*v1beta1.Ingress); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if nsNode := p.graph.GetNode(ingressUID(ingress)); nsNode != nil {
			p.graph.DelNode(nsNode)
			logging.GetLogger().Debugf("Deleted %s", dumpIngress(ingress))
		}
	}
}

func (p *ingressProbe) Start() {
	p.kubeCache.Start()
}

func (p *ingressProbe) Stop() {
	p.kubeCache.Stop()
}

func newIngressKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().ExtensionsV1beta1().RESTClient(), &v1beta1.Ingress{}, "ingresses", handler)
}

func newIngressProbe(g *graph.Graph) probe.Probe {
	p := &ingressProbe{
		graph: g,
	}
	p.kubeCache = newIngressKubeCache(p)
	return p
}
