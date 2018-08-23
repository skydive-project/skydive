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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
)

type ingressProbe struct {
	DefaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*KubeCache
	graph          *graph.Graph
	serviceIndexer *graph.MetadataIndexer
}

func dumpIngress(ingress *v1beta1.Ingress) string {
	return fmt.Sprintf("ingress{Namespace: %s, Name: %s}", ingress.Namespace, ingress.Name)
}

func (p *ingressProbe) newMetadata(ingress *v1beta1.Ingress) graph.Metadata {
	m := NewMetadata(Manager, "ingress", ingress.Namespace, ingress.Name, ingress)
	m.SetFieldAndNormalize("Backend", ingress.Spec.Backend)
	m.SetFieldAndNormalize("TLS", ingress.Spec.TLS)
	m.SetFieldAndNormalize("Rules", ingress.Spec.Rules)
	return m
}

func ingressUID(ingress *v1beta1.Ingress) graph.Identifier {
	return graph.Identifier(ingress.GetUID())
}

func (p *ingressProbe) newEdgeMetadata(serviceName string, servicePort intstr.IntOrString) graph.Metadata {
	m := NewEdgeMetadata(Manager)
	m.SetField("RelationType", "ingress")
	m.SetField("ServiceName", serviceName)
	m.SetField("ServicePort", servicePort)
	return m
}

func (p *ingressProbe) updateLinksForService(ingress *v1beta1.Ingress, ingressNode, srvNode *graph.Node) {
	if ingress.Spec.Backend == nil {
		return
	}

	if ingress.Spec.Backend.ServiceName != srvNode.Metadata()["Name"] {
		return
	}

	edge := p.newEdgeMetadata(ingress.Spec.Backend.ServiceName, ingress.Spec.Backend.ServicePort)
	AddLinkTry(p.graph, ingressNode, srvNode, edge)

	// TODO: handle deletion of stale links
	// TODO: support backends defined in ingress.Spec.Rules
}

func (p *ingressProbe) updateLinks(ingress *v1beta1.Ingress, ingressNode *graph.Node) {
	srvNodes, _ := p.serviceIndexer.Get(ingress.Namespace)
	for _, srvNode := range srvNodes {
		p.updateLinksForService(ingress, ingressNode, srvNode)
	}
}

func (p *ingressProbe) OnAdd(obj interface{}) {
	if ingress, ok := obj.(*v1beta1.Ingress); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		ingressNode := NewNode(p.graph, ingressUID(ingress), p.newMetadata(ingress))
		logging.GetLogger().Debugf("Added %s", dumpIngress(ingress))
		p.updateLinks(ingress, ingressNode)
	}
}

func (p *ingressProbe) OnUpdate(oldObj, newObj interface{}) {
	if ingress, ok := newObj.(*v1beta1.Ingress); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if ingressNode := p.graph.GetNode(ingressUID(ingress)); ingressNode != nil {
			AddMetadata(p.graph, ingressNode, ingress)
			logging.GetLogger().Debugf("Updated %s", dumpIngress(ingress))
			p.updateLinks(ingress, ingressNode)
		}
	}
}

func (p *ingressProbe) OnDelete(obj interface{}) {
	if ingress, ok := obj.(*v1beta1.Ingress); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if ingressNode := p.graph.GetNode(ingressUID(ingress)); ingressNode != nil {
			p.graph.DelNode(ingressNode)
			logging.GetLogger().Debugf("Deleted %s", dumpIngress(ingress))
		}
	}
}

func (p *ingressProbe) onNodeUpdated(srvNode *graph.Node) {
	logging.GetLogger().Debugf("update links: %s", DumpNode(srvNode))

	for _, ingress := range p.KubeCache.list() {
		ingress := ingress.(*v1beta1.Ingress)
		logging.GetLogger().Debugf("refreshing %s", dumpIngress(ingress))
		ingressNode := p.graph.GetNode(ingressUID(ingress))
		if ingressNode == nil {
			logging.GetLogger().Debugf("can't find %s", dumpIngress(ingress))
			continue
		}
		p.updateLinksForService(ingress, ingressNode, srvNode)
	}
}

func (p *ingressProbe) OnNodeAdded(node *graph.Node) {
	p.onNodeUpdated(node)
}

func (p *ingressProbe) OnNodeUpdated(node *graph.Node) {
	p.onNodeUpdated(node)
}

func (p *ingressProbe) Start() {
	p.KubeCache.Start()
	p.serviceIndexer.AddEventListener(p)
	p.serviceIndexer.Start()
}

func (p *ingressProbe) Stop() {
	p.KubeCache.Stop()
	p.serviceIndexer.RemoveEventListener(p)
	p.serviceIndexer.Stop()
}

func newIngressKubeCache(handler cache.ResourceEventHandler) *KubeCache {
	return NewKubeCache(getClientset().ExtensionsV1beta1().RESTClient(), &v1beta1.Ingress{}, "ingresses", handler)
}

func newIngressProbe(g *graph.Graph) probe.Probe {
	p := &ingressProbe{
		graph:          g,
		serviceIndexer: NewObjectIndexerByNamespace(Manager, g, "service"),
	}
	p.KubeCache = newIngressKubeCache(p)
	return p
}
