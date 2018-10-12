/*
 * Copyright 2017 IBM Corp.
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
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Manager is the manager value for Kubernetes
	Manager      = "k8s"
	detailsField = "K8s"
)

// NewMetadata creates a k8s node base metadata struct
func NewMetadata(manager, ty, details interface{}, name string, namespace ...string) graph.Metadata {
	m := graph.Metadata{}
	m["Manager"] = manager
	m["Type"] = ty

	if len(namespace) == 1 {
		m["Namespace"] = namespace[0]
	}
	m["Name"] = name

	if details != nil {
		m[detailsField] = common.NormalizeValue(details)
	}
	return m
}

func newEdgeMetadata() graph.Metadata {
	m := graph.Metadata{
		"Manager":      Manager,
		"RelationType": "association",
	}
	return m
}

func newObjectIndexer(g *graph.Graph, h graph.ListenerHandler, nodeType string, indexes ...string) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", Manager),
		filters.NewTermStringFilter("Type", nodeType),
		filters.NewNotNullFilter("Namespace"),
		filters.NewNotNullFilter("Name"),
	)
	m := graph.NewElementFilter(filter)
	return graph.NewMetadataIndexer(g, h, m, indexes...)
}

func newResourceLinker(g *graph.Graph, subprobes map[string]Subprobe, srcType string, srcAttrs []string, dstType string, dstAttrs []string, edgeMetadata graph.Metadata) probe.Probe {
	srcCache := subprobes[srcType]
	dstCache := subprobes[dstType]
	if srcCache == nil || dstCache == nil {
		return nil
	}

	srcIndexer := graph.NewMetadataIndexer(g, srcCache, graph.Metadata{"Type": srcType}, srcAttrs...)
	srcIndexer.Start()

	dstIndexer := graph.NewMetadataIndexer(g, dstCache, graph.Metadata{"Type": dstType}, dstAttrs...)
	dstIndexer.Start()

	return graph.NewMetadataIndexerLinker(g, srcIndexer, dstIndexer, edgeMetadata)
}

func objectToNode(g *graph.Graph, object metav1.Object) (node *graph.Node) {
	return g.GetNode(graph.Identifier(object.GetUID()))
}

func objectsToNodes(g *graph.Graph, objects []metav1.Object) (nodes []*graph.Node) {
	for _, obj := range objects {
		if node := objectToNode(g, obj); node != nil {
			nodes = append(nodes, node)
		}
	}
	return
}
