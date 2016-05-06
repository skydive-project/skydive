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

package graph

import (
	"strings"
	"testing"
)

func newTrasversalGraph(t *testing.T) *Graph {
	g := newGraph(t)

	n1 := g.NewNode(GenID(), Metadata{"Value": 1, "Type": "intf"})
	n2 := g.NewNode(GenID(), Metadata{"Value": 2, "Type": "intf"})
	n3 := g.NewNode(GenID(), Metadata{"Value": 3})
	n4 := g.NewNode(GenID(), Metadata{"Value": 4, "Name": "Node4"})

	g.Link(n1, n2)
	g.Link(n2, n3, Metadata{"Direction": "Left"})
	g.Link(n3, n4)
	g.Link(n1, n4)
	g.Link(n1, n3, Metadata{"Mode": "Direct"})

	return g
}

func TestBasicTraversal(t *testing.T) {
	g := newTrasversalGraph(t)

	tr := NewGrahTraversal(g)

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

	node := tv.Values()[0].(*Node)
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

	node = tv.Values()[0].(*Node)
	if node.Metadata()["Value"] != 3 {
		t.Fatalf("Should return Node3, returned: %v", tv.Values())
	}

	// next traversal test
	tv = tr.V().Has("Type")
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalWithin(t *testing.T) {
	g := newTrasversalGraph(t)

	tr := NewGrahTraversal(g)

	tv := tr.V().Has("Value", Within(1, 2, 4))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}
}

func execTraversalQuery(t *testing.T, g *Graph, query string) GraphTraversalStep {
	ts, err := NewGraphTraversalParser(strings.NewReader(query), g).Parse()
	if err != nil {
		t.Fatal(err.Error())
	}

	res, err := ts.Exec()
	if err != nil {
		t.Fatal(err.Error())
	}

	return res
}

func TestTraversalParser(t *testing.T) {
	g := newTrasversalGraph(t)

	// next traversal test
	query := `G.V().Has("Type", "intf")`
	res := execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
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

	node := res.Values()[0].(*Node)
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
	node = res.Values()[0].(*Node)

	query = `G.V("` + string(node.ID) + `")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 || res.Values()[0].(*Node).ID != node.ID {
		t.Fatalf("Should return 1 nodes, returned: %v", res.Values())
	}
}
