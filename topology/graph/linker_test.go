/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package graph

import (
	"testing"
	"time"

	"github.com/skydive-project/skydive/filters"
)

type simpleLinker struct {
	g *Graph
}

func (l *simpleLinker) GetABLinks(n *Node) (edges []*Edge) {
	n2 := l.g.GetNode("node2")
	if n2 == nil {
		return nil
	}

	edge := l.g.CreateEdge("", n, n2, Metadata{}, time.Now())
	return []*Edge{edge}
}

func (l *simpleLinker) GetBALinks(n *Node) []*Edge {
	return nil
}

type metadataLinker struct {
	g *Graph
}

func (l *metadataLinker) GetABLinks(n *Node) []*Edge {
	return nil
}

func (l *metadataLinker) GetBALinks(n *Node) (edges []*Edge) {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Name", "host1"),
	)
	m := NewElementFilter(filter)
	nodes := l.g.GetNodes(m)

	for _, node := range nodes {
		if node.ID != n.ID {
			edges = append(edges, l.g.CreateEdge("", node, n, Metadata{}, time.Now()))
		}
	}
	return
}

func TestSimpleLinker(t *testing.T) {
	g := newGraph(t)

	m1 := Metadata{
		"Type": "host",
		"Name": "host1",
	}

	m2 := Metadata{
		"Type": "host",
		"Name": "host2",
	}

	linker := NewResourceLinker(g, g.eventHandler, nil, &simpleLinker{g: g}, Metadata{"RelationType": "simpleLinker"})
	linker.Start()

	n1 := g.NewNode("node1", m1, "host")
	n2 := g.NewNode("node2", m2, "host")

	if g.AreLinked(n1, n2, Metadata{"RelationType": "simpleLinker"}) {
		t.Error("nodes should not be linked by an edge of type 'simpleLinker'")
	}

	g.DelNode(n1)
	g.AddNode(n1)

	if !g.AreLinked(n1, n2, Metadata{"RelationType": "simpleLinker"}) {
		t.Error("nodes should be linked by an edge of type 'simpleLinker'")
	}

	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("RelationType", "simpleLinker"),
		filters.NewTermStringFilter("Parent", "node1"),
		filters.NewTermStringFilter("Child", "node2"),
	)
	m := NewElementFilter(filter)

	if l := len(g.GetEdges(m)); l != 1 {
		t.Errorf("there should be only one node between node1 and node2, got %d", l)
	}

	g.DelNode(n1)
	g.DelNode(n2)

	g.AddNode(n2)
	g.AddNode(n1)

	if !g.AreLinked(n1, n2, Metadata{"RelationType": "simpleLinker"}) {
		t.Error("nodes should be linked by an edge of type 'simpleLinker'")
	}

	linker.Stop()

	g.DelNode(n1)
	g.DelNode(n2)
}

func TestMetadataLinker(t *testing.T) {
	g := newGraph(t)

	m1 := Metadata{
		"Type": "host",
		"Name": "host1",
	}

	m2 := Metadata{
		"Type": "host",
		"Name": "host2",
	}

	linker := NewResourceLinker(g, g.eventHandler, g.eventHandler, &metadataLinker{g: g}, Metadata{"RelationType": "metadataLinker"})
	linker.Start()

	n1 := g.NewNode("node1", m1, "host")
	n2 := g.NewNode("node2", m2, "host")

	if !g.AreLinked(n1, n2, Metadata{"RelationType": "metadataLinker"}) {
		t.Error("nodes should be linked by an edge of type 'metadataLinker'")
	}

	g.AddMetadata(n1, "Name", "unknownHost")

	if g.AreLinked(n1, n2, Metadata{"RelationType": "metadataLinker"}) {
		t.Error("nodes should not be linked by an edge of type 'metadataLinker'")
	}
}

func TestMetadataIndexerLinker(t *testing.T) {
	g := newGraph(t)

	m1 := Metadata{
		"Type": "host",
		"Name": "host1",
	}

	m2 := Metadata{
		"Type": "host",
		"Name": "host2",
	}

	m3 := Metadata{
		"Type":       "host",
		"ParentHost": "host1",
	}

	hasNameFilter := NewElementFilter(filters.NewNotNullFilter("Name"))
	indexer1 := NewMetadataIndexer(g, g, hasNameFilter, "Name")
	indexer1.Start()

	hasParentNameFilter := NewElementFilter(filters.NewNotNullFilter("ParentHost"))
	indexer2 := NewMetadataIndexer(g, g, hasParentNameFilter, "ParentHost")
	indexer2.Start()

	linker := NewMetadataIndexerLinker(g, indexer1, indexer2, Metadata{"RelationType": "metadataIndexerLinker"})
	linker.Start()

	n1 := g.NewNode("node1", m1, "host")
	n2 := g.NewNode("node2", m2, "host")
	n3 := g.NewNode("node3", m3, "host")

	if !g.AreLinked(n1, n3, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be linked by an edge of type 'metadataIndexerLinker'")
	}

	g.AddMetadata(n3, "ParentHost", "host2")

	if !g.AreLinked(n2, n3, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be linked by an edge of type 'metadataIndexerLinker'")
	}

	if g.AreLinked(n1, n3, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be not linked by an edge of type 'metadataIndexerLinker'")
	}
}
