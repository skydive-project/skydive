/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package graph

import (
	"testing"

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

	edge := l.g.CreateEdge("", n, n2, Metadata{}, TimeUTC())
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
			edges = append(edges, l.g.CreateEdge("", node, n, Metadata{}, TimeUTC()))
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

	linker := NewResourceLinker(g, []ListenerHandler{g.eventHandler}, nil, &simpleLinker{g: g}, Metadata{"RelationType": "simpleLinker"})
	linker.Start()

	n1, _ := g.NewNode("node1", m1, "host")
	n2, _ := g.NewNode("node2", m2, "host")

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
		t.Errorf("there should be only one edge between node1 and node2, got %d", l)
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

	linker := NewResourceLinker(g, []ListenerHandler{g.eventHandler}, []ListenerHandler{g.eventHandler}, &metadataLinker{g: g}, Metadata{"RelationType": "metadataLinker"})
	linker.Start()

	n1, _ := g.NewNode("node1", m1, "host")
	n2, _ := g.NewNode("node2", m2, "host")

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

	n1, _ := g.NewNode("node1", m1, "host")
	n2, _ := g.NewNode("node2", m2, "host")
	n3, _ := g.NewNode("node3", m3, "host")

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

func TestMetadataIndexerLinkerWithArray(t *testing.T) {
	g := newGraph(t)

	vfCache := NewMetadataIndexer(g, g, nil, "VFS.MAC")
	vfCache.Start()

	macCache := NewMetadataIndexer(g, g, nil, "MAC")
	macCache.Start()

	vf1 := Metadata{
		"Name": "vf1",
		"VFS": []interface{}{
			map[string]interface{}{
				"MAC": "00:11:22:33:44:55",
			},
			map[string]interface{}{
				"MAC": "01:23:45:67:89:ab",
			},
		},
	}

	m1 := Metadata{
		"Name": "m1",
		"MAC":  "00:11:22:33:44:55",
	}

	m2 := Metadata{
		"Name": "m2",
		"MAC":  "01:23:45:67:89:ab",
	}

	linker := NewMetadataIndexerLinker(g, vfCache, macCache, Metadata{"RelationType": "metadataIndexerLinker"})
	linker.Start()

	n1, _ := g.NewNode(GenID(), vf1, "host")
	n2, _ := g.NewNode(GenID(), m1, "host")
	n3, _ := g.NewNode(GenID(), m2, "host")

	if !g.AreLinked(n1, n2, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be linked by an edge of type 'metadataIndexerLinker'")
	}

	if !g.AreLinked(n1, n3, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be linked by an edge of type 'metadataIndexerLinker'")
	}

	g.SetMetadata(n3, Metadata{
		"Name": "m2",
		"MAC":  "01:23:45:67:89:af",
	})

	if !g.AreLinked(n1, n2, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be linked by an edge of type 'metadataIndexerLinker'")
	}

	if g.AreLinked(n1, n3, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be not linked by an edge of type 'metadataIndexerLinker'")
	}

	g.SetMetadata(n1, Metadata{
		"Name": "vf1",
		"VFS": []interface{}{
			map[string]interface{}{
				"MAC": "00:11:22:33:44:59",
			},
			map[string]interface{}{
				"MAC": "01:23:45:67:89:ab",
			},
		},
	})

	if g.AreLinked(n1, n2, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be not linked by an edge of type 'metadataIndexerLinker'")
	}

	if g.AreLinked(n1, n3, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be not linked by an edge of type 'metadataIndexerLinker'")
	}

	g.SetMetadata(n1, Metadata{
		"Name": "vf1",
		"VFS": []interface{}{
			map[string]interface{}{
				"MAC": "00:11:22:33:44:59",
			},
			map[string]interface{}{
				"MAC": "01:23:45:67:89:af",
			},
		},
	})

	if g.AreLinked(n1, n2, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be not linked by an edge of type 'metadataIndexerLinker'")
	}

	if !g.AreLinked(n1, n3, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be linked by an edge of type 'metadataIndexerLinker'")
	}

	g.SetMetadata(n1, Metadata{
		"Name": "vf1",
	})

	if g.AreLinked(n1, n2, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be not linked by an edge of type 'metadataIndexerLinker'")
	}

	if g.AreLinked(n1, n3, Metadata{"RelationType": "metadataIndexerLinker"}) {
		t.Error("nodes should be not linked by an edge of type 'metadataIndexerLinker'")
	}
}
