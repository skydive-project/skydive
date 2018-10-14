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
	"time"

	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type serviceHandler struct {
	DefaultResourceHandler
}

func (h *serviceHandler) Dump(obj interface{}) string {
	srv := obj.(*v1.Service)
	return fmt.Sprintf("service{Namespace: %s, Name: %s}", srv.Namespace, srv.Name)
}

func (h *serviceHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	srv := obj.(*v1.Service)

	m := NewMetadata(Manager, "service", srv, srv.Name, srv.Namespace)
	m.SetFieldAndNormalize("Ports", srv.Spec.Ports)
	m.SetFieldAndNormalize("ClusterIP", srv.Spec.ClusterIP)
	m.SetFieldAndNormalize("ServiceType", srv.Spec.Type)
	m.SetFieldAndNormalize("SessionAffinity", srv.Spec.SessionAffinity)
	m.SetFieldAndNormalize("LoadBalancerIP", srv.Spec.LoadBalancerIP)
	m.SetFieldAndNormalize("ExternalName", srv.Spec.ExternalName)

	return graph.Identifier(srv.GetUID()), m
}

func newServiceProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return NewResourceCache(clientset.Core().RESTClient(), &v1.Service{}, "services", g, &serviceHandler{})
}

type servicePodLinker struct {
	graph        *graph.Graph
	serviceCache *ResourceCache
	podCache     *ResourceCache
}

func (spl *servicePodLinker) newEdgeMetadata() graph.Metadata {
	m := newEdgeMetadata()
	m.SetField("RelationType", "service")
	return m
}

func (spl *servicePodLinker) GetABLinks(srvNode *graph.Node) (edges []*graph.Edge) {
	if srv := spl.serviceCache.getByNode(srvNode); srv != nil {
		srv := srv.(*v1.Service)
		labelSelector := &metav1.LabelSelector{MatchLabels: srv.Spec.Selector}
		selectedPods := objectsToNodes(spl.graph, spl.podCache.getBySelector(spl.graph, srv.Namespace, labelSelector))
		metadata := spl.newEdgeMetadata()
		for _, podNode := range selectedPods {
			id := graph.GenID(string(srvNode.ID), string(podNode.ID), "RelationType", "service")
			edges = append(edges, spl.graph.NewEdge(id, srvNode, podNode, metadata, ""))
		}
	}
	return
}

func (spl *servicePodLinker) GetBALinks(podNode *graph.Node) (edges []*graph.Edge) {
	namespace, _ := podNode.GetFieldString("Namespace")
	name, _ := podNode.GetFieldString("Name")
	pod := spl.podCache.getByKey(namespace, name)
	for _, srv := range spl.serviceCache.getByNamespace(namespace) {
		srv := srv.(*v1.Service)
		labelSelector := &metav1.LabelSelector{MatchLabels: srv.Spec.Selector}
		if len(filterObjectsBySelector([]interface{}{pod}, labelSelector)) != 1 {
			continue
		}
		if srvNode := spl.graph.GetNode(graph.Identifier(srv.GetUID())); srvNode != nil {
			edges = append(edges, spl.graph.CreateEdge("", srvNode, podNode, spl.newEdgeMetadata(), time.Now(), ""))
		}
	}
	return
}

func newServicePodLinker(g *graph.Graph, probes map[string]Subprobe) probe.Probe {
	serviceProbe := probes["service"]
	podProbe := probes["pod"]
	if serviceProbe == nil || podProbe == nil {
		return nil
	}

	return graph.NewResourceLinker(
		g,
		serviceProbe,
		podProbe,
		&servicePodLinker{
			graph:        g,
			serviceCache: serviceProbe.(*ResourceCache),
			podCache:     podProbe.(*ResourceCache),
		},
		graph.Metadata{"RelationType": "service"},
	)
}
