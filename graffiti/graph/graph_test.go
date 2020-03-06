/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/skydive-project/skydive/graffiti/service"
)

func newGraph(t *testing.T) *Graph {
	b, err := NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	return NewGraph("testhost", b, service.UnknownService)
}

func TestLinks(t *testing.T) {
	g := newGraph(t)

	n1, _ := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2, _ := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})

	g.NewEdge(GenID(), n1, n2, nil)
	if !g.AreLinked(n1, n2, nil) {
		t.Error("nodes should be linked")
	}

	g.Unlink(n1, n2)
	if g.AreLinked(n1, n2, nil) {
		t.Error("nodes shouldn't be linked")
	}

	g.Link(n1, n2, nil)
	if !g.AreLinked(n1, n2, nil) {
		t.Error("nodes should be linked")
	}

	g.DelNode(n2)
	if g.AreLinked(n1, n2, nil) {
		t.Error("nodes shouldn't be linked")
	}
}

func TestAreLinkedWithMetadata(t *testing.T) {
	g := newGraph(t)

	n1, _ := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2, _ := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})

	g.Link(n1, n2, Metadata{"Type": "aaa"})
	if !g.AreLinked(n1, n2, nil) {
		t.Error("nodes should be linked")
	}

	if !g.AreLinked(n1, n2, Metadata{"Type": "aaa"}) {
		t.Error("nodes should be linked")
	}

	if g.AreLinked(n1, n2, Metadata{"Type": "bbb"}) {
		t.Error("nodes shouldn't be linked")
	}
}

func TestFirstLinkDoesExist(t *testing.T) {
	g := newGraph(t)

	n1, _ := g.NewNode(GenID(), Metadata{"Type": "intf"})
	n2, _ := g.NewNode(GenID(), Metadata{"Type": "intf"})

	g.Link(n1, n2, Metadata{})

	if g.GetFirstLink(n1, n2, nil) == nil {
		t.Error("nodes should be linked")
	}

	if g.GetFirstLink(n2, n1, nil) != nil {
		t.Error("nodes should not be linked")
	}
}

func TestFirstLinkIsCorrect(t *testing.T) {
	g := newGraph(t)

	n1, _ := g.NewNode(GenID(), Metadata{"Type": "intf"})
	n2, _ := g.NewNode(GenID(), Metadata{"Type": "intf"})

	g.Link(n2, n1, Metadata{"Field": "other"})
	expected := "me"
	g.Link(n1, n2, Metadata{"Field": expected})

	e := g.GetFirstLink(n1, n2, nil)
	if e == nil {
		t.Error("nodes should be linked")
	}

	if actual := e.Metadata["Field"]; actual != expected {
		t.Errorf("Wrong metadata['Direction'] expected '%s' got '%s'", expected, actual)
	}
}

func TestGetFirstLinkLoop(t *testing.T) {
	g := newGraph(t)

	n1, _ := g.NewNode(GenID(), Metadata{"Type": "intf"})
	n2, _ := g.NewNode(GenID(), Metadata{"Type": "intf"})

	g.Link(n2, n2, Metadata{})

	if g.GetFirstLink(n1, n2, nil) != nil {
		t.Error("nodes should not be linked")
	}

	if g.GetFirstLink(n2, n1, nil) != nil {
		t.Error("nodes should not be linked")
	}

	if g.GetFirstLink(n2, n2, nil) == nil {
		t.Error("nodes should be linked")
	}
}

func TestBasicLookup(t *testing.T) {
	g := newGraph(t)

	n1, _ := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2, _ := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	n3, _ := g.NewNode(GenID(), Metadata{"Value": 3})
	n4, _ := g.NewNode(GenID(), Metadata{"Value": 4, "Name": "Node4"})

	g.NewEdge(GenID(), n1, n2, nil)
	g.NewEdge(GenID(), n2, n3, nil)
	g.Link(n1, n4, nil)

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

func TestBasicLookupUnsupportedTypes(t *testing.T) {
	g := newGraph(t)

	n1, _ := g.NewNode(GenID(), Metadata{"Value": int32(1), "Type": float64(44.5)})

	if n := g.LookupFirstNode(Metadata{"Value": uint64(1)}); n == nil || n.ID != n1.ID {
		t.Error("Wrong node returned")
	}

	if n := g.LookupFirstNode(Metadata{"Value": 1}); n == nil || n.ID != n1.ID {
		t.Error("Wrong node returned")
	}

	if n1.ID != g.LookupFirstNode(Metadata{"Type": 44.5}).ID {
		t.Error("Wrong node returned")
	}
}

func TestHierarchyLookup(t *testing.T) {
	g := newGraph(t)

	n1, _ := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2, _ := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	n3, _ := g.NewNode(GenID(), Metadata{"Value": 3})
	n4, _ := g.NewNode(GenID(), Metadata{"Value": 4, "Name": "Node4"})

	g.Link(n1, n2, nil)
	g.Link(n2, n3, nil)
	g.Link(n3, n4, nil)
	g.Link(n2, n4, nil)

	r := g.LookupParents(n4, nil, nil)
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

	n1, _ := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2, _ := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	n3, _ := g.NewNode(GenID(), Metadata{"Value": 3})
	n4, _ := g.NewNode(GenID(), Metadata{"Value": 4, "Name": "Node4"})

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

	r = g.LookupShortestPath(n4, Metadata{"Value": 2}, Metadata{"Type": "Layer6"})
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
	n5, _ := g.NewNode(GenID(), Metadata{"Value": 5, "Name": "Node5"})
	g.Link(n4, n5, Metadata{"Type": "Layer2"})

	n11, _ := g.NewNode(GenID(), Metadata{"Value": 11, "Name": "Node11"})
	n12, _ := g.NewNode(GenID(), Metadata{"Value": 12, "Name": "Node12"})
	g.Link(n1, n11, Metadata{"Type": "Layer2"})
	g.Link(n11, n12, Metadata{"Type": "Layer2"})
	g.Link(n12, n5, Metadata{"Type": "Layer2"})

	n121, _ := g.NewNode(GenID(), Metadata{"Value": 121, "Name": "Node121"})
	n122, _ := g.NewNode(GenID(), Metadata{"Value": 122, "Name": "Node122"})

	g.Link(n12, n121, Metadata{"Type": "Layer2"})
	g.Link(n121, n122, Metadata{"Type": "Layer2"})
	g.Link(n122, n5, Metadata{"Type": "Layer2"})

	r = g.LookupShortestPath(n1, Metadata{"Value": 5}, nil)
	if len(r) == 0 || !validatePath(r, "1/11/12/5") {
		t.Errorf("Wrong nodes returned: %v", r)
	}
}

func nodeExpand(g *Graph, nodes []*Node, n int, level int) []*Node {
	var ret []*Node
	for _, node := range nodes {
		for i := 0; i < n; i++ {
			n, _ := g.NewNode(GenID(), Metadata{"Value": level*1000 + i, "Name": fmt.Sprintf("Node%d-%d", level, i)})
			g.Link(node, n, Metadata{"Type": "Layer2"})
			ret = append(ret, n)
		}
	}
	return ret
}

func nodeCollapse(g *Graph, nodes []*Node, nodeEnd *Node) {
	for _, node := range nodes {
		g.Link(node, nodeEnd, Metadata{"Type": "Layer2"})
	}
}

func TestComplexPath(t *testing.T) {
	g := newGraph(t)

	nstart, _ := g.NewNode(GenID(), Metadata{"Value": 1, "Name": "NodeStart"})
	nend, _ := g.NewNode(GenID(), Metadata{"Value": 1, "Name": "NodeEnd"})

	nprev := nodeExpand(g, []*Node{nstart}, 10, 1)
	for i := 2; i < 4; i++ { // test complexity limit, as test.timeout = 1min
		nprev = nodeExpand(g, nprev, 10, i)
	}
	nodeCollapse(g, []*Node{nprev[0]}, nend)
	t.Log("nb nodes", len(g.GetNodes(nil)))

	r := g.LookupShortestPath(nstart, nend.Metadata, nil)
	if len(r) != 5 {
		t.Errorf("Wrong nodes returned (start -> end): %d %v", len(r), r)
	}
	r = g.LookupShortestPath(nend, nstart.Metadata, nil)
	if len(r) != 5 {
		t.Errorf("Wrong nodes returned (end -> start): %d %v", len(r), r)
	}
}

func TestMetadata(t *testing.T) {
	g := newGraph(t)

	n, _ := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})

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

func TestMetadataTransaction(t *testing.T) {
	g := newGraph(t)

	n, _ := g.NewNode(GenID(), Metadata{"Type": "intf", "Section": []string{"A1", "A2"}})
	g.AddMetadata(n, "Label.List1", "EL1")
	g.AddMetadata(n, "Label.List2", "EL2")

	tr := g.StartMetadataTransaction(n)
	tr.AddMetadata("Name", "test123")

	if _, ok := n.Metadata["Name"]; ok {
		t.Error("Name field should not be in the node metadata until commit")
	}

	tr.AddMetadata("Section", []string{"B1"})
	tr.AddMetadata("Label.List3", "EL3")

	if _, err := n.GetFieldString("Label.List3"); err == nil {
		t.Errorf("Element shouldn't be present: %+v", n)
	}

	tr.Commit()

	if len(n.Metadata["Section"].([]string)) != 1 {
		t.Errorf("Should only have one element, found: %+v\n", n.Metadata["Section"])
	}

	if _, err := n.GetFieldString("Label.List1"); err != nil {
		t.Errorf("Element should be present: %+v", n)
	}

	if _, err := n.GetFieldString("Label.List3"); err != nil {
		t.Errorf("Element should be present: %+v", n)
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

	n1, _ := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	if l.lastNodeAdded.ID != n1.ID {
		t.Error("Didn't get the notification")
	}

	n2, _ := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	if l.lastNodeAdded.ID != n2.ID {
		t.Error("Didn't get the notification")
	}

	e, _ := g.NewEdge(GenID(), n1, n2, nil)
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
