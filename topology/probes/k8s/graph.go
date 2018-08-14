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
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	managerValue = "k8s"
	hostID       = ""
)

const (
	detailsField  = "K8s"
	nodeNameField = detailsField + ".Spec.NodeName"
)

func lookupChildren(g *graph.Graph, parent, childFilter *graph.Node, edgeFilter graph.Metadata) []*graph.Node {
	return g.LookupChildren(parent, childFilter.Metadata(), edgeFilter)
}

func newMetadata(ty, namespace, name string, details interface{}) graph.Metadata {
	m := graph.Metadata{
		"Manager":    managerValue,
		"Type":       ty,
		"Namespace":  namespace,
		"Name":       name,
		detailsField: common.NormalizeValue(details),
	}
	return m
}

func addMetadata(g *graph.Graph, n *graph.Node, details interface{}) {
	tr := g.StartMetadataTransaction(n)
	tr.AddMetadata(detailsField, common.NormalizeValue(details))
	tr.Commit()
}

func newEdgeMetadata() graph.Metadata {
	m := graph.Metadata{
		"Manager":      managerValue,
		"RelationType": "association",
	}
	return m
}

func dumpGraphLink(parent, child *graph.Node, m graph.Metadata) string {
	return fmt.Sprintf("%s -> %s: %s", dumpGraphNode(parent), dumpGraphNode(child), m)
}

func addLink(g *graph.Graph, parent, child *graph.Node, m graph.Metadata) *graph.Edge {
	if e := g.GetFirstLink(parent, child, m); e != nil {
		logging.GetLogger().Debugf("Adding link: %s: exists - skipping", dumpGraphLink(parent, child, m))
		return e
	}

	logging.GetLogger().Debugf("Adding link: %s", dumpGraphLink(parent, child, m))
	return g.Link(parent, child, m, hostID)
}

func delLink(g *graph.Graph, parent, child *graph.Node, m graph.Metadata) {
	e := g.GetFirstLink(parent, child, m)
	if e == nil {
		logging.GetLogger().Debugf("Deleting link: %s: missing - skipping", dumpGraphLink(parent, child, m))
		return
	}

	logging.GetLogger().Debugf("Deleting link: %s", dumpGraphLink(parent, child, m))
	g.DelEdge(e)
}

func addOwnershipLink(g *graph.Graph, parent, child *graph.Node) *graph.Edge {
	m := graph.Metadata{
		"Manager": managerValue,
	}
	if e := topology.GetOwnershipLink(g, parent, child); e != nil {
		return e
	}
	logging.GetLogger().Debugf("Adding ownership: %s", dumpGraphLink(parent, child, m))
	return topology.AddOwnershipLink(g, parent, child, m, hostID)
}

func newNode(g *graph.Graph, i graph.Identifier, m graph.Metadata) *graph.Node {
	return g.NewNode(i, m, hostID)
}

func dumpGraphNode(n *graph.Node) string {
	ty, _ := n.GetFieldString("Type")
	namespace, _ := n.GetFieldString("Namespace")
	name, _ := n.GetFieldString("Name")
	return fmt.Sprintf("%s{Namespace: %s, Name: %s}", ty, namespace, name)
}

func newObjectIndexerByNamespace(g *graph.Graph, ty string) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", managerValue),
		filters.NewTermStringFilter("Type", ty),
		filters.NewNotNullFilter("Namespace"),
	)
	m := graph.NewGraphElementFilter(filter)
	return graph.NewMetadataIndexer(g, m, "Namespace")
}

func newObjectIndexerByNamespaceAndName(g *graph.Graph, ty string) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", managerValue),
		filters.NewTermStringFilter("Type", ty),
		filters.NewNotNullFilter("Namespace"),
		filters.NewNotNullFilter("Name"),
	)
	m := graph.NewGraphElementFilter(filter)
	return graph.NewMetadataIndexer(g, m, "Namespace", "Name")
}
