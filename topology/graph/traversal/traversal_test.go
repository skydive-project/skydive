/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package traversal

import (
	"strings"
	"testing"

	"github.com/skydive-project/skydive/topology/graph"
)

func newGraph(t *testing.T) *graph.Graph {
	b, err := graph.NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	return graph.NewGraphFromConfig(b)
}

func newTransversalGraph(t *testing.T) *graph.Graph {
	g := newGraph(t)

	n1 := g.NewNode(graph.GenID(), graph.Metadata{"Value": 1, "Type": "intf", "Bytes": 1024})
	n2 := g.NewNode(graph.GenID(), graph.Metadata{"Value": 2, "Type": "intf", "Bytes": 2024})
	n3 := g.NewNode(graph.GenID(), graph.Metadata{"Value": 3})
	n4 := g.NewNode(graph.GenID(), graph.Metadata{"Value": 4, "Name": "Node4", "Bytes": 4024})

	g.Link(n1, n2, graph.Metadata{"Direction": "Left"})
	g.Link(n2, n3, graph.Metadata{"Direction": "Left"})
	g.Link(n3, n4)
	g.Link(n1, n4)
	g.Link(n1, n3, graph.Metadata{"Mode": "Direct"})

	return g
}

func TestBasicTraversal(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	// next traversal test
	tv := tr.V().Has("Value", 1)
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V().Has("Type", "intf")
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 2 {
		t.Fatalf("should return 2 nodes, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V().Has("Value", 1).Out().Has("Value", 2).OutE().Has("Direction", "Left").OutV().Out()
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	node := tv.Values()[0].(*graph.Node)
	if node.Metadata()["Name"] != "Node4" {
		t.Fatalf("Should return Node4, returned: %v", tv.Values())
	}

	// next traversal test
	tv = tr.V().Has("Value", 1).Out().Has("Value", 4)
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V().Has("Value", 1).OutE().Has("Mode", "Slow").OutV()
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 0 {
		t.Fatalf("should return 0 node, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V().Has("Value", 1).OutE().Has("Mode", "Direct").OutV()
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	node = tv.Values()[0].(*graph.Node)
	if node.Metadata()["Value"] != 3 {
		t.Fatalf("Should return Node3, returned: %v", tv.Values())
	}

	// next traversal test
	tv = tr.V().Has("Type")
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}

	props := tr.V().PropertyKeys()
	if len(props.Values()) != 10 {
		t.Fatalf("Should return 10 properties, returned: %s", props.Values())
	}

	res := tr.V().PropertyValues("Type")
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}
	sum := tr.V().Sum("Bytes")
	bytes, ok := sum.Values()[0].(float64)
	if ok {
		if bytes != 7072 {
			t.Fatalf("Should return 7072, instead got %f", bytes)
		}
	} else {
		t.Logf("Error in Sum() step: %s", sum.Error())
	}

}

func TestTraversalWithin(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	tv := tr.V().Has("Value", Within(1, 2, 4))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalLt(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	tv := tr.V().Has("Value", Lt(3))
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalGt(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	tv := tr.V().Has("Value", Gt(3))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalLte(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	tv := tr.V().Has("Value", Lte(3))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalGte(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	tv := tr.V().Has("Value", Gte(3))
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalInside(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	tv := tr.V().Has("Value", Inside(1, 4))
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalBetween(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	tv := tr.V().Has("Value", Between(1, 4))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalNe(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	// next test
	tv := tr.V().Has("Value", Ne(1))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V().Has("Name", Ne(""))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}
}

func TestTraversalRegex(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	// next test
	tv := tr.V().Has("Name", Regex("ode"))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V().Has("Name", Regex("ode5"))
	if len(tv.Values()) != 0 {
		t.Fatalf("Shouldn't return node, returned: %v", tv.Values())
	}
}

func TestTraversalBoth(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	// next test
	tv := tr.V().Has("Value", 2).Both()
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalCount(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	// next test
	tv := tr.V().Count()
	if tv.Values()[0] != 4 {
		t.Fatalf("Should return 4 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalShortestPathTo(t *testing.T) {
	g := newTransversalGraph(t)

	tr := NewGraphTraversal(g)

	tv := tr.V().Has("Value", 1).ShortestPathTo(graph.Metadata{"Value": 3})
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 path, returned: %v", tv.Values())
	}

	path := tv.Values()[0].([]*graph.Node)
	if len(path) != 2 {
		t.Fatalf("Should return a path len of 2, returned: %v", len(path))
	}

	// next test
	tv = tr.V().Has("Value", Within(1, 2)).ShortestPathTo(graph.Metadata{"Value": 3})
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 paths, returned: %v", tv.Values())
	}

	path = tv.Values()[0].([]*graph.Node)
	if len(path) != 2 {
		t.Fatalf("Should return a path len of 2, returned: %v", len(path))
	}

	// next test
	tv = tr.V().Has("Value", 1).ShortestPathTo(graph.Metadata{"Value": 3}, graph.Metadata{"Direction": "Left"})
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 path, returned: %v", tv.Values())
	}

	path = tv.Values()[0].([]*graph.Node)
	if len(path) != 3 {
		t.Fatalf("Should return a path len of 3, returned: %v", len(path))
	}
}

func execTraversalQuery(t *testing.T, g *graph.Graph, query string) GraphTraversalStep {
	ts, err := NewGremlinTraversalParser(g).Parse(strings.NewReader(query))
	if err != nil {
		t.Fatalf("%s: %s", query, err.Error())
	}

	res, err := ts.Exec()
	if err != nil {
		t.Fatalf("%s: %s", query, err.Error())
	}

	return res
}

func TestTraversalParser(t *testing.T) {
	g := newTransversalGraph(t)

	// next traversal test
	query := `G.V().Has("Type", "intf")`
	res := execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Count()`
	res = execTraversalQuery(t, g, query)
	if res.Values()[0] != 4 {
		t.Fatalf("Should return 4, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Value", 1).Out().Has("Name", "Node4")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Value", 1).Out().Has("Value", 2).OutE().Has("Direction", "Left").OutV().Out()`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d, %v", len(res.Values()), res.Values())
	}

	node := res.Values()[0].(*graph.Node)
	if node.Metadata()["Name"] != "Node4" {
		t.Fatalf("Should return Node4, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Type")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Out().Has("Value", 4)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Out().Has("Value", 4).Dedup()`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Dedup("Type")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Value", Within(1, 2, 4))`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Value", Within(1.0, 2, 4))`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Value", 1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d, %v", len(res.Values()), res.Values())
	}
	node = res.Values()[0].(*graph.Node)

	query = `G.V("` + string(node.ID) + `")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 || res.Values()[0].(*graph.Node).ID != node.ID {
		t.Fatalf("Should return 1 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Value", 1).ShortestPathTo(Metadata("Value", 3), Metadata("Direction", "Left"))`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 path, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Name", Ne(""))`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Value", 2).Both()`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Name", Regex("ode"))`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Values("Type")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 node, returned: %v", res.Values())
	}
}
