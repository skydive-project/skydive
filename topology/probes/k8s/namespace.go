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

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type namespaceHandler struct {
	DefaultResourceHandler
}

func (h *namespaceHandler) IsTopLevel() bool {
	return true
}

func (h *namespaceHandler) Dump(obj interface{}) string {
	ns := obj.(*v1.Namespace)
	return fmt.Sprintf("namespace{Name: %s}", ns.Name)
}

func (h *namespaceHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	ns := obj.(*v1.Namespace)

	m := NewMetadata(Manager, "namespace", ns, ns.Name)
	m.SetFieldAndNormalize("Labels", ns.Labels)
	m.SetField("Cluster", ns.ClusterName)
	m.SetField("Status", ns.Status.Phase)

	return graph.Identifier(ns.GetUID()), m
}

func newNamespaceProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return NewResourceCache(clientset.Core().RESTClient(), &v1.Namespace{}, "namespaces", g, &namespaceHandler{})
}

func newNamespaceLinker(g *graph.Graph, subprobes map[string]Subprobe) probe.Probe {
	nsSubprobe := subprobes["namespace"]
	if nsSubprobe == nil {
		return nil
	}

	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", Manager),
		filters.NewTermStringFilter("Type", "namespace"),
		filters.NewNotNullFilter("Name"),
	)

	namespaceIndexer := graph.NewMetadataIndexer(g, nsSubprobe, graph.NewElementFilter(filter), "Name")
	namespaceIndexer.Start()

	ownedByNamespaceFilter := filters.NewOrFilter(
		filters.NewTermStringFilter("Type", "cronjob"),
		filters.NewTermStringFilter("Type", "deployment"),
		filters.NewTermStringFilter("Type", "daemonset"),
		filters.NewTermStringFilter("Type", "endpoints"),
		filters.NewTermStringFilter("Type", "ingress"),
		filters.NewTermStringFilter("Type", "job"),
		filters.NewTermStringFilter("Type", "pod"),
		filters.NewTermStringFilter("Type", "persistentvolume"),
		filters.NewTermStringFilter("Type", "persistentvolumeclaim"),
		filters.NewTermStringFilter("Type", "replicaset"),
		filters.NewTermStringFilter("Type", "replicationcontroller"),
		filters.NewTermStringFilter("Type", "service"),
		filters.NewTermStringFilter("Type", "statefulset"),
		filters.NewTermStringFilter("Type", "storageclass"),
	)

	filter = filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", Manager),
		ownedByNamespaceFilter,
	)
	objectIndexer := graph.NewMetadataIndexer(g, g, graph.NewElementFilter(filter), "Namespace")
	objectIndexer.Start()

	return graph.NewMetadataIndexerLinker(g, namespaceIndexer, objectIndexer, topology.OwnershipMetadata())
}
