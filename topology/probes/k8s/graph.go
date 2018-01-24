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
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

const manager = "k8s"

func newMetadata(typ, name string, extra interface{}) graph.Metadata {
	m := graph.Metadata{
		"Manager": manager,
		"Type":    typ,
		"Name":    name,
		"K8s":     common.NormalizeValue(extra),
	}
	return m
}

func addMetadata(g *graph.Graph, n *graph.Node, extra interface{}) {
	tr := g.StartMetadataTransaction(n)
	tr.AddMetadata("K8s", common.NormalizeValue(extra))
	tr.Commit()
}

func newEdgeMetadata() graph.Metadata {
	m := graph.Metadata{
		"Manager":      manager,
		"RelationType": "Association",
	}
	return m
}

func addLink(g *graph.Graph, parent, child *graph.Node) *graph.Edge {
	m := newEdgeMetadata()
	if e := g.GetFirstLink(parent, child, m); e != nil {
		return e
	}
	return g.Link(parent, child, m)
}

func addOwnershipLink(g *graph.Graph, parent, child *graph.Node) *graph.Edge {
	m := graph.Metadata{
		"Manager": manager,
	}
	if e := topology.GetOwnershipLink(g, parent, child, m); e != nil {
		return e
	}
	return topology.AddOwnershipLink(g, parent, child, m)
}

func syncLink(g *graph.Graph, parent, child *graph.Node, toAdd bool) {
	if !toAdd {
		g.Unlink(parent, child)
	} else {
		addLink(g, parent, child)
	}
}
