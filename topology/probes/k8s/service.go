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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type serviceProbe struct {
	defaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*kubeCache
	podCache              *kubeCache
	graph                 *graph.Graph
	podIndexerByNamespace *graph.MetadataIndexer
}

func dumpService(srv *v1.Service) string {
	return fmt.Sprintf("service{Namespace: %s, Name: %s}", srv.Namespace, srv.Name)
}

func (p *serviceProbe) newMetadata(srv *v1.Service) graph.Metadata {
	m := newMetadata("service", srv.Namespace, srv.Name, srv)
	m.SetFieldAndNormalize("Ports", srv.Spec.Ports)
	m.SetFieldAndNormalize("ClusterIP", srv.Spec.ClusterIP)
	m.SetFieldAndNormalize("ServiceType", srv.Spec.Type)
	m.SetFieldAndNormalize("SessionAffinity", srv.Spec.SessionAffinity)
	m.SetFieldAndNormalize("LoadBalancerIP", srv.Spec.LoadBalancerIP)
	m.SetFieldAndNormalize("ExternalName", srv.Spec.ExternalName)
	return m
}

func serviceUID(srv *v1.Service) graph.Identifier {
	return graph.Identifier(srv.GetUID())
}

func (p *serviceProbe) filterPodByLabels(in []interface{}, srv *v1.Service) (out []interface{}) {
	selector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: srv.Spec.Selector})
	for _, pod := range in {
		pod := pod.(*v1.Pod)
		if srv.Namespace == pod.Namespace && selector.Matches(labels.Set(pod.Labels)) {
			out = append(out, pod)
		}
	}
	return
}

func (p *serviceProbe) selectedPods(srv *v1.Service) (nodes []*graph.Node) {
	pods := p.podCache.list()
	pods = p.filterPodByLabels(pods, srv)
	for _, pod := range pods {
		pod := pod.(*v1.Pod)
		if podNode := p.graph.GetNode(podUID(pod)); podNode != nil {
			nodes = append(nodes, podNode)
		}
	}
	logging.GetLogger().Debugf("found %d pods", len(nodes))
	return
}

func (p *serviceProbe) newEdgeMetadata() graph.Metadata {
	m := newEdgeMetadata()
	m.SetField("RelationType", "service")
	return m
}

func (p *serviceProbe) updateLinksForPod(srv *v1.Service, srvNode, podNode *graph.Node) {
	addLink(p.graph, srvNode, podNode, p.newEdgeMetadata())
	// TODO: handle deletion of stale links
	// TODO: support srv.Spec.Ports
}

func (p *serviceProbe) updateLinks(srv *v1.Service, srvNode *graph.Node) {
	for _, podNode := range p.selectedPods(srv) {
		p.updateLinksForPod(srv, srvNode, podNode)
	}
}

func (p *serviceProbe) OnAdd(obj interface{}) {
	if srv, ok := obj.(*v1.Service); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		srvNode := newNode(p.graph, serviceUID(srv), p.newMetadata(srv))
		logging.GetLogger().Debugf("Added %s", dumpService(srv))
		p.updateLinks(srv, srvNode)
	}
}

func (p *serviceProbe) OnUpdate(oldObj, newObj interface{}) {
	if srv, ok := newObj.(*v1.Service); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if srvNode := p.graph.GetNode(serviceUID(srv)); srvNode != nil {
			addMetadata(p.graph, srvNode, srv)
			logging.GetLogger().Debugf("Updated %s", dumpService(srv))
			p.updateLinks(srv, srvNode)
		}
	}
}

func (p *serviceProbe) OnDelete(obj interface{}) {
	if srv, ok := obj.(*v1.Service); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if srvNode := p.graph.GetNode(serviceUID(srv)); srvNode != nil {
			p.graph.DelNode(srvNode)
			logging.GetLogger().Debugf("Deleted %s", dumpService(srv))
		}
	}
}

func (p *serviceProbe) onNodeUpdated(podNode *graph.Node) {
	logging.GetLogger().Debugf("update links: %s", dumpGraphNode(podNode))

	for _, srv := range p.kubeCache.list() {
		srv := srv.(*v1.Service)
		logging.GetLogger().Debugf("refreshing %s", dumpService(srv))
		srvNode := p.graph.GetNode(serviceUID(srv))
		if srvNode == nil {
			logging.GetLogger().Debugf("can't find %s", dumpService(srv))
			continue
		}
		p.updateLinksForPod(srv, srvNode, podNode)
	}
}

func (p *serviceProbe) OnNodeAdded(node *graph.Node) {
	p.onNodeUpdated(node)
}

func (p *serviceProbe) OnNodeUpdated(node *graph.Node) {
	p.onNodeUpdated(node)
}

func (p *serviceProbe) Start() {
	p.kubeCache.Start()
	p.podIndexerByNamespace.AddEventListener(p)
	p.podIndexerByNamespace.Start()
}

func (p *serviceProbe) Stop() {
	p.kubeCache.Stop()
	p.podIndexerByNamespace.RemoveEventListener(p)
	p.podIndexerByNamespace.Stop()
}

func newServiceKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().Core().RESTClient(), &v1.Service{}, "services", handler)
}

func newServiceProbe(g *graph.Graph) probe.Probe {
	p := &serviceProbe{
		graph: g,
		podIndexerByNamespace: newObjectIndexerByNamespace(g, "pod"),
	}
	p.kubeCache = newServiceKubeCache(p)
	p.podCache = newPodKubeCache(p)
	return p
}
