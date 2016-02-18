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
	"testing"
)

func newGraph(t *testing.T) *Graph {
	b, err := NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	g, err := NewGraph(b)
	if err != nil {
		t.Error(err.Error())
	}

	return g
}

func TestLinks(t *testing.T) {
	g := newGraph(t)

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})

	g.NewEdge(GenID(), n1, n2, nil)
	if !g.AreLinked(n1, n2) {
		t.Error("nodes should be linked")
	}

	g.Unlink(n1, n2)
	if g.AreLinked(n1, n2) {
		t.Error("nodes shouldn't be linked")
	}

	g.Link(n1, n2)
	if !g.AreLinked(n1, n2) {
		t.Error("nodes should be linked")
	}

	g.DelNode(n2)
	if g.AreLinked(n1, n2) {
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
	g.Link(n1, n4)

	if n1.ID != g.GetNode(n1.ID).ID {
		t.Error("Wrong node returned")
	}

	if n1.ID != g.LookupFirstNode(Metadata{"Value": 1}).ID {
		t.Error("Wrong node returned")
	}

	r := g.LookupNodes(Metadata{"Type": "intf"})
	if len(r) != 2 {
		t.Error("Wrong number of nodes returned")
	}

	for i := range r {
		if !(r[i].ID == n1.ID || r[i].ID == n2.ID) {
			t.Error("Wrong nodes returned")
		}
	}

	r = g.LookupNodesFromKey("Type")
	if len(r) != 2 {
		t.Error("Wrong number of nodes returned")
	}

	for i := range r {
		if !(r[i].ID == n1.ID || r[i].ID == n2.ID) {
			t.Error("Wrong nodes returned")
		}
	}
}

func TestHierarchyLookup(t *testing.T) {
	g := newGraph(t)

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	n3 := g.NewNode(GenID(), Metadata{"Value": 3})
	n4 := g.NewNode(GenID(), Metadata{"Value": 4, "Name": "Node4"})

	g.Link(n1, n2)
	g.Link(n2, n3)
	g.Link(n3, n4)
	g.Link(n2, n4)

	r := g.LookupParentNodes(n4, nil)
	if len(r) != 2 {
		t.Error("Wrong number of nodes returned")
	}

	for i := range r {
		if !(r[i].ID == n2.ID || r[i].ID == n3.ID) {
			t.Error("Wrong nodes returned")
		}
	}

	r = g.LookupParentNodes(n4, Metadata{"Type": "intf"})
	if len(r) != 1 {
		t.Error("Wrong number of nodes returned")
	}
	if r[0].ID != n2.ID {
		t.Error("Wrong nodes returned")
	}

	r = g.LookupChildren(n2, nil)
	if len(r) != 2 {
		t.Error("Wrong number of nodes returned")
	}

	for i := range r {
		if !(r[i].ID == n3.ID || r[i].ID == n4.ID) {
			t.Error("Wrong nodes returned")
		}
	}

	r = g.LookupChildren(n2, Metadata{"Name": "Node4"})
	if len(r) != 1 {
		t.Error("Wrong number of nodes returned")
	}

	if r[0].ID != n4.ID {
		t.Error("Wrong nodes returned")
	}
}

func TestPath(t *testing.T) {
	g := newGraph(t)

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	n3 := g.NewNode(GenID(), Metadata{"Value": 3})
	n4 := g.NewNode(GenID(), Metadata{"Value": 4, "Name": "Node4"})

	g.Link(n1, n2)
	g.Link(n2, n3)
	g.Link(n3, n4)

	r, ok := g.GetAncestorsTo(n4, Metadata{"Value": 1})
	if !ok || len(r) != 4 {
		t.Error("Wrong nodes returned")
	}

	if r[0].ID != n4.ID || r[1].ID != n3.ID || r[2].ID != n2.ID || r[3].ID != n1.ID {
		t.Error("Wrong path returned")
	}

	r, ok = g.GetAncestorsTo(n4, Metadata{"Value": 2})
	if !ok || len(r) != 3 {
		t.Error("Wrong nodes returned")
	}

	if r[0].ID != n4.ID || r[1].ID != n3.ID || r[2].ID != n2.ID {
		t.Error("Wrong path returned")
	}
}

func TestMetadata(t *testing.T) {
	g := newGraph(t)

	n := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})

	g.AddMetadata(n, "Name", "Node1")
	v, ok := n.Metadata()["Name"]
	if !ok || v != "Node1" {
		t.Error("Metadata not updated")
	}

	g.AddMetadata(n, "Value", "Value1")
	v, ok = n.Metadata()["Value"]
	if !ok || v != "Value1" {
		t.Error("Metadata not updated")
	}

	g.SetMetadata(n, Metadata{"Temp": 35})
	_, ok = n.Metadata()["Value"]
	if ok {
		t.Error("Metadata should be removed")
	}
	v, ok = n.Metadata()["Temp"]
	if !ok || v != 35 {
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
