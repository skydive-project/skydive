/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"strconv"
	"strings"
	"testing"
)

func newGraph(t *testing.T) *Graph {
	b, err := NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	return NewGraphFromConfig(b)
}

func TestLinks(t *testing.T) {
	g := newGraph(t)

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})

	g.NewEdge(GenID(), n1, n2, nil)
	if !g.AreLinked(n1, n2, Metadata{}) {
		t.Error("nodes should be linked")
	}

	g.Unlink(n1, n2)
	if g.AreLinked(n1, n2, Metadata{}) {
		t.Error("nodes shouldn't be linked")
	}

	g.Link(n1, n2, Metadata{})
	if !g.AreLinked(n1, n2, Metadata{}) {
		t.Error("nodes should be linked")
	}

	g.DelNode(n2)
	if g.AreLinked(n1, n2, Metadata{}) {
		t.Error("nodes shouldn't be linked")
	}
}

func TestAreLinkedWithMetadata(t *testing.T) {
	g := newGraph(t)

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})

	g.Link(n1, n2, Metadata{"Type": "aaa"})
	if !g.AreLinked(n1, n2, Metadata{}) {
		t.Error("nodes should be linked")
	}

	if !g.AreLinked(n1, n2, Metadata{"Type": "aaa"}) {
		t.Error("nodes should be linked")
	}

	if g.AreLinked(n1, n2, Metadata{"Type": "bbb"}) {
		t.Error("nodes shouldn't be linked")
	}
}

func TestBasicLookup(t *testing.T) {
	g := newGraph(t)

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	n3 := g.NewNode(GenID(), Metadata{"Value": 3})
	n4 := g.NewNode(GenID(), Metadata{"Value": 4, "Name": "Node4"})

	g.NewEdge(GenID(), n1, n2, nil)
	g.NewEdge(GenID(), n2, n3, nil)
	g.Link(n1, n4, Metadata{})

	if n1.ID != g.GetNode(n1.ID).ID {
		t.Error("Wrong node returned")
	}

	if n1.ID != g.LookupFirstNode(Metadata{"Value": 1}).ID {
		t.Error("Wrong node returned")
	}

	r := g.GetNodes(Metadata{"Type": "intf"})
	if len(r) != 2 {
		t.Error("Wrong number of nodes returned")
	}

	for i := range r {
		if !(r[i].ID == n1.ID || r[i].ID == n2.ID) {
			t.Error("Wrong nodes returned")
		}
	}
}

func TestBasicLookupMultipleTypes(t *testing.T) {
	g := newGraph(t)

	n1 := g.NewNode(GenID(), Metadata{"Value": int32(1), "Type": float64(44.5)})

	if g.LookupFirstNode(Metadata{"Value": uint64(1)}) != nil {
		t.Error("Should return no node")
	}

	if g.LookupFirstNode(Metadata{"Value": 1}) != nil {
		t.Error("Should return no node")
	}

	if n1.ID != g.LookupFirstNode(Metadata{"Type": 44.5}).ID {
		t.Error("Wrong node returned")
	}
}

func TestHierarchyLookup(t *testing.T) {
	g := newGraph(t)

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	n3 := g.NewNode(GenID(), Metadata{"Value": 3})
	n4 := g.NewNode(GenID(), Metadata{"Value": 4, "Name": "Node4"})

	g.Link(n1, n2, Metadata{})
	g.Link(n2, n3, Metadata{})
	g.Link(n3, n4, Metadata{})
	g.Link(n2, n4, Metadata{})

	r := g.LookupParents(n4, nil, Metadata{})
	if len(r) != 2 {
		t.Error("Wrong number of nodes returned")
	}

	for i := range r {
		if !(r[i].ID == n2.ID || r[i].ID == n3.ID) {
			t.Error("Wrong nodes returned")
		}
	}

	r = g.LookupParents(n4, Metadata{"Type": "intf"}, nil)
	if len(r) != 1 {
		t.Error("Wrong number of nodes returned")
	}
	if r[0].ID != n2.ID {
		t.Error("Wrong nodes returned")
	}

	r = g.LookupChildren(n2, nil, nil)
	if len(r) != 2 {
		t.Errorf("Wrong number of nodes returned: %+v", r)
	}

	for i := range r {
		if !(r[i].ID == n3.ID || r[i].ID == n4.ID) {
			t.Error("Wrong nodes returned")
		}
	}

	r = g.LookupChildren(n2, Metadata{"Name": "Node4"}, nil)
	if len(r) != 1 {
		t.Error("Wrong number of nodes returned")
	}

	if r[0].ID != n4.ID {
		t.Error("Wrong nodes returned")
	}
}

func TestPath(t *testing.T) {
	g := newGraph(t)

	validatePath := func(nodes []*Node, expected string) bool {
		var values []string

		for _, n := range nodes {
			value, _ := n.GetFieldInt64("Value")
			values = append(values, strconv.FormatInt(value, 10))
		}

		return expected == strings.Join(values, "/")
	}

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	n3 := g.NewNode(GenID(), Metadata{"Value": 3})
	n4 := g.NewNode(GenID(), Metadata{"Value": 4, "Name": "Node4"})

	g.Link(n1, n2, Metadata{"Type": "Layer2"})
	g.Link(n2, n3, Metadata{"Type": "Layer2"})
	g.Link(n3, n4, Metadata{"Type": "Layer2"})

	r := g.LookupShortestPath(n4, Metadata{"Value": 1}, nil)
	if len(r) == 0 || !validatePath(r, "4/3/2/1") {
		t.Errorf("Wrong nodes returned: %v", r)
	}

	// add a shorter link
	g.Link(n4, n1, Metadata{"Type": "Layer2"})
	r = g.LookupShortestPath(n4, Metadata{"Value": 1}, nil)
	if len(r) == 0 || !validatePath(r, "4/1") {
		t.Errorf("Wrong nodes returned: %v", r)
	}
	g.Unlink(n4, n1)

	r = g.LookupShortestPath(n4, Metadata{"Value": 2}, nil)
	if len(r) == 0 || !validatePath(r, "4/3/2") {
		t.Errorf("Wrong nodes returned: %v", r)
	}

	r = g.LookupShortestPath(n4, Metadata{"Value": 55}, nil)
	if len(r) > 0 {
		t.Errorf("Shouldn't have true returned: %v", r)
	}

	// add a shorter link in order to validate edge validator
	g.Link(n1, n4, Metadata{"Type": "Layer3"})

	r = g.LookupShortestPath(n4, Metadata{"Value": 1}, Metadata{"Type": "Layer2"})
	if len(r) == 0 || !validatePath(r, "4/3/2/1") {
		t.Errorf("Wrong nodes returned: %v", r)
	}

	r = g.LookupShortestPath(n4, Metadata{"Value": 1}, Metadata{"Type": "Layer3"})
	if len(r) == 0 || !validatePath(r, "4/1") {
		t.Errorf("Wrong nodes returned: %v", r)
	}
	g.Unlink(n1, n4)

	// test shortestPath on the following graph
	// n1 -- n2 -- n3 -- n4 -----------
	//  \                              \
	//   \-- n11 -- n12---------------- n5
	//                \                /
	//                 \-- n121 -- n122
	n5 := g.NewNode(GenID(), Metadata{"Value": 5, "Name": "Node5"})
	g.Link(n4, n5, Metadata{"Type": "Layer2"})

	n11 := g.NewNode(GenID(), Metadata{"Value": 11, "Name": "Node11"})
	n12 := g.NewNode(GenID(), Metadata{"Value": 12, "Name": "Node12"})
	g.Link(n1, n11, Metadata{"Type": "Layer2"})
	g.Link(n11, n12, Metadata{"Type": "Layer2"})
	g.Link(n12, n5, Metadata{"Type": "Layer2"})

	n121 := g.NewNode(GenID(), Metadata{"Value": 121, "Name": "Node121"})
	n122 := g.NewNode(GenID(), Metadata{"Value": 122, "Name": "Node122"})

	g.Link(n12, n121, Metadata{"Type": "Layer2"})
	g.Link(n121, n122, Metadata{"Type": "Layer2"})
	g.Link(n122, n5, Metadata{"Type": "Layer2"})

	r = g.LookupShortestPath(n1, Metadata{"Value": 5}, nil)
	if len(r) == 0 || !validatePath(r, "1/11/12/5") {
		t.Errorf("Wrong nodes returned: %v", r)
	}
}

func TestMetadata(t *testing.T) {
	g := newGraph(t)

	n := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})

	g.AddMetadata(n, "Name", "Node1")
	v, _ := n.GetFieldString("Name")
	if v != "Node1" {
		t.Error("Metadata not updated")
	}

	g.AddMetadata(n, "Value", "Value1")
	v, _ = n.GetFieldString("Value")
	if v != "Value1" {
		t.Error("Metadata not updated")
	}

	g.SetMetadata(n, Metadata{"Temp": 35})
	_, err := n.GetFieldString("Value")
	if err == nil {
		t.Error("Metadata should be removed")
	}
	i, err := n.GetFieldInt64("Temp")
	if err != nil || i != 35 {
		t.Error("Metadata not updated")
	}
}

type FakeListener struct {
	lastNodeUpdated *Node
	lastNodeAdded   *Node
	lastNodeDeleted *Node
	lastEdgeUpdated *Edge
	lastEdgeAdded   *Edge
	lastEdgeDeleted *Edge
}

func (c *FakeListener) OnNodeUpdated(n *Node) {
	c.lastNodeUpdated = n
}

func (c *FakeListener) OnNodeAdded(n *Node) {
	c.lastNodeAdded = n
}

func (c *FakeListener) OnNodeDeleted(n *Node) {
	c.lastNodeDeleted = n
}

func (c *FakeListener) OnEdgeUpdated(e *Edge) {
	c.lastEdgeUpdated = e
}

func (c *FakeListener) OnEdgeAdded(e *Edge) {
	c.lastEdgeAdded = e
}

func (c *FakeListener) OnEdgeDeleted(e *Edge) {
	c.lastEdgeDeleted = e
}

func TestEvents(t *testing.T) {
	g := newGraph(t)

	l := &FakeListener{}
	g.AddEventListener(l)

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	if l.lastNodeAdded.ID != n1.ID {
		t.Error("Didn't get the notification")
	}

	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	if l.lastNodeAdded.ID != n2.ID {
		t.Error("Didn't get the notification")
	}

	e := g.NewEdge(GenID(), n1, n2, nil)
	if l.lastEdgeAdded.ID != e.ID {
		t.Error("Didn't get the notification")
	}

	g.AddMetadata(n1, "Name", "Node1")
	if l.lastNodeUpdated.ID != n1.ID {
		t.Error("Didn't get the notification")
	}

	g.Unlink(n1, n2)
	if l.lastEdgeDeleted.ID != e.ID {
		t.Error("Didn't get the notification")
	}

	g.DelNode(n2)
	if l.lastNodeDeleted.ID != n2.ID {
		t.Error("Didn't get the notification")
	}
}

type FakeRecursiveListener1 struct {
	DefaultGraphListener
	graph *Graph
}

type FakeRecursiveListener2 struct {
	DefaultGraphListener
	events []*Node
}

func (f *FakeRecursiveListener1) OnNodeAdded(n *Node) {
	f.graph.NewNode(GenID(), Metadata{"Value": 2})
}

func (f *FakeRecursiveListener2) OnNodeAdded(n *Node) {
	f.events = append(f.events, n)
}

func TestRecursiveEvents(t *testing.T) {
	g := newGraph(t)

	l1 := &FakeRecursiveListener1{graph: g}
	g.AddEventListener(l1)

	l2 := &FakeRecursiveListener2{}
	g.AddEventListener(l2)

	g.NewNode(GenID(), Metadata{"Value": 1})

	ev1, _ := l2.events[0].GetFieldInt64("Value")
	ev2, _ := l2.events[1].GetFieldInt64("Value")

	// check if the notification are in the right order
	if ev1 != 1 || ev2 != 2 {
		t.Error("Events are not in the right order")
	}
}
