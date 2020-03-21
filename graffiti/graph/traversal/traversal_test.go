/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package traversal

import (
	"strings"
	"testing"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
)

func newGraph(t *testing.T) *graph.Graph {
	b, err := graph.NewMemoryBackend()
	if err != nil {
		t.Error(err)
	}

	return graph.NewGraph("testhost", b, service.UnknownService)
}

func newTransversalGraph(t *testing.T) *graph.Graph {
	g := newGraph(t)

	n1, _ := g.NewNode(graph.GenID(), graph.Metadata{"Value": int64(1), "Type": "intf", "Bytes": int64(1024), "List": []string{"111", "222"}, "Map": map[string]int64{"a": 1}})
	n2, _ := g.NewNode(graph.GenID(), graph.Metadata{"Value": int64(2), "Type": "intf", "Bytes": int64(2024), "IPV4": []string{"10.0.0.1", "10.0.1.2"}})
	n3, _ := g.NewNode(graph.GenID(), graph.Metadata{"Value": int64(3), "IPV4": "192.168.0.34/24", "Map": map[string]int64{}, "MTU": 1500})
	n4, _ := g.NewNode(graph.GenID(), graph.Metadata{"Value": int64(4), "Name": "Node4", "Bytes": int64(4024), "IPV4": "192.168.1.34", "Indexes": []int64{5, 6}})

	g.Link(n1, n2, graph.Metadata{"Direction": "Left", "Name": "e1"})
	g.Link(n2, n3, graph.Metadata{"Direction": "Left", "Name": "e2"})
	g.Link(n3, n4, graph.Metadata{"Name": "e3"})
	g.Link(n1, n4, graph.Metadata{"Name": "e4"})
	g.Link(n1, n3, graph.Metadata{"Mode": "Direct", "Name": "e5"})

	return g
}

func TestBasicTraversal(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next traversal test
	tv := tr.V(ctx).Has(ctx, "Value", 1)
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V(ctx).Has(ctx, "Type", "intf")
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 2 {
		t.Fatalf("should return 2 nodes, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V(ctx).Has(ctx, "Type", "intf", "badparams")
	if tv.Error() == nil {
		t.Fatal("Has() params should be a list of key,value")
	}

	// next traversal test
	tv = tr.V(ctx).HasEither(ctx, "Type", "intf", "Name", "Node4")
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 3 {
		t.Fatalf("should return 3 nodes, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V(ctx).Has(ctx, "List", "111")
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V(ctx).Has(ctx, "Indexes", 6)
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V(ctx).Has(ctx, "Value", 1).Out(ctx).Has(ctx, "Value", 2).OutE(ctx).Has(ctx, "Direction", "Left").OutV(ctx).Out(ctx)
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	node := tv.Values()[0].(*graph.Node)
	if name, _ := node.GetFieldString("Name"); name != "Node4" {
		t.Fatalf("Should return Node4, returned: %v", tv.Values())
	}

	// next traversal test
	te := tr.V(ctx).Has(ctx, "Value", 3).BothE(ctx)
	if te.Error() != nil {
		t.Fatal(te.Error())
	}

	if len(te.Values()) != 3 {
		t.Fatalf("should return 3 edges, returned: %d", len(te.Values()))
	}

	// next traversal test
	tv = tr.V(ctx).Has(ctx, "Value", 1).Out(ctx).Has(ctx, "Value", 4)
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V(ctx).Has(ctx, "Value", 1).OutE(ctx).Has(ctx, "Mode", "Slow").OutV(ctx)
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 0 {
		t.Fatalf("should return 0 node, returned: %d", len(tv.Values()))
	}

	// next traversal test
	tv = tr.V(ctx).Has(ctx, "Value", 1).OutE(ctx).Has(ctx, "Mode", "Direct").OutV(ctx)
	if tv.Error() != nil {
		t.Fatal(tv.Error())
	}

	if len(tv.Values()) != 1 {
		t.Fatalf("should return 1 node, returned: %d", len(tv.Values()))
	}

	node = tv.Values()[0].(*graph.Node)
	if value, _ := node.GetFieldInt64("Value"); value != 3 {
		t.Fatalf("Should return Node3, returned: %v", tv.Values())
	}

	// next traversal test
	tv = tr.V(ctx).Has(ctx, "Type")
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}

	props := tr.V(ctx).PropertyKeys(ctx)
	if len(props.Values()) != 16 {
		t.Fatalf("Should return 16 properties, returned: %s", props.Values())
	}

	res := tr.V(ctx).PropertyValues(ctx, "Type")
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	res = tr.V(ctx).PropertyValues(ctx, "Map")
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 nodes, returned: %v", res.Values())
	}

	sum := tr.V(ctx).Sum(ctx, "Bytes")
	bytes, ok := sum.Values()[0].(int64)
	if ok {
		if bytes != 7072 {
			t.Fatalf("Should return 7072, instead got %v", bytes)
		}
	} else {
		t.Fatalf("Error in Sum() step: %s", sum.Error())
	}
}

func TestTraversalWithin(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.V(ctx).Has(ctx, "Value", Within(1, 2, 4))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalWithout(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.V(ctx).Has(ctx, "Value", Without(1, 3))
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalLt(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.V(ctx).Has(ctx, "Value", Lt(3))
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalGt(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.V(ctx).Has(ctx, "Value", Gt(3))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalLte(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.V(ctx).Has(ctx, "Value", Lte(3))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalGte(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.V(ctx).Has(ctx, "Value", Gte(3))
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalInside(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.V(ctx).Has(ctx, "Value", Inside(1, 4))
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalBetween(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.V(ctx).Has(ctx, "Value", Between(1, 4))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalNe(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next test
	tv := tr.V(ctx).Has(ctx, "Value", Ne(1))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "MTU", Ne(1200))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "MTU", Ne(""))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "MTU", Ne(1500))
	if len(tv.Values()) != 0 {
		t.Fatalf("Should return 0 node, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "aaaaaaaaaaa", Ne(""))
	if len(tv.Values()) != 0 {
		t.Fatalf("Shouldn't return a node for an unknown key, returned: %v", tv.Values())
	}
}

func TestTraversalNee(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next test
	tv := tr.V(ctx).Has(ctx, "Value", Nee(1))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "MTU", Nee(1200))
	if len(tv.Values()) != 4 {
		t.Fatalf("Should return 4 nodes, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "MTU", Nee(""))
	if len(tv.Values()) != 4 {
		t.Fatalf("Should return 4 nodes, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "MTU", Nee(1500))
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "aaaaaaaaaaa", Nee(""))
	if len(tv.Values()) != 4 {
		t.Fatalf("Shouldn't return all the nodes for an unknown key, returned: %v", tv.Values())
	}
}

func TestTraversalHasKey(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next test
	tv := tr.V(ctx).HasKey(ctx, "Name")
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}

	tv = tr.V(ctx).HasKey(ctx, "Unknown")
	if len(tv.Values()) != 0 {
		t.Fatalf("Should return 0 node, returned: %v", tv.Values())
	}

	tv = tr.V(ctx).HasKey(ctx, "List")
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}
}

func TestTraversalHasNot(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next test
	tv := tr.V(ctx).HasNot(ctx, "Name")
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalRegex(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next test
	tv := tr.V(ctx).Has(ctx, "Name", Regex(".*ode.*"))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "Name", Regex("ode5"))
	if len(tv.Values()) != 0 {
		t.Fatalf("Shouldn't return node, returned: %v", tv.Values())
	}
}

func TestTraversalIpv4Range(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next test
	tv := tr.V(ctx).Has(ctx, "IPV4", IPV4Range("192.168.0.0/24"))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "IPV4", IPV4Range("192.168.0.0/16"))
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}

	tv = tr.V(ctx).Has(ctx, "IPV4", IPV4Range("192.168.0.0/26"))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "IPV4", IPV4Range("192.168.0.77/26"))
	if len(tv.Values()) != 0 {
		t.Fatalf("Shouldn't return node, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "IPV4", IPV4Range("192.168.2.0/24"))
	if len(tv.Values()) != 0 {
		t.Fatalf("Shouldn't return node, returned: %v", tv.Values())
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "IPV4", IPV4Range("10.0.0.0/24"))
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", tv.Values())
	}
}

func TestTraversalBoth(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next test
	tv := tr.V(ctx).Has(ctx, "Value", 2).Both(ctx)
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalCount(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next test
	tv := tr.V(ctx).Count(ctx)
	if tv.Values()[0] != 4 {
		t.Fatalf("Should return 4 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalShortestPathTo(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.V(ctx).Has(ctx, "Value", int64(1)).ShortestPathTo(ctx, graph.Metadata{"Value": int64(3)}, nil)
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 path, returned: %v", tv.Values())
	}

	path := tv.Values()[0].([]*graph.Node)
	if len(path) != 2 {
		t.Fatalf("Should return a path len of 2, returned: %v", len(path))
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "Value", Within(int64(1), int64(2))).ShortestPathTo(ctx, graph.Metadata{"Value": int64(3)}, nil)
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 paths, returned: %v", tv.Values())
	}

	path = tv.Values()[0].([]*graph.Node)
	if len(path) != 2 {
		t.Fatalf("Should return a path len of 2, returned: %v", len(path))
	}

	// next test
	tv = tr.V(ctx).Has(ctx, "Value", int64(1)).ShortestPathTo(ctx, graph.Metadata{"Value": int64(3)}, graph.Metadata{"Direction": "Left"})
	if len(tv.Values()) != 1 {
		t.Fatalf("Should return 1 path, returned: %v", tv.Values())
	}

	path = tv.Values()[0].([]*graph.Node)
	if len(path) != 3 {
		t.Fatalf("Should return a path len of 3, returned: %v", len(path))
	}
}

func TestTraversalBothV(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	// next test
	tv := tr.E(ctx).Has(ctx, "Name", "e3").BothV(ctx)
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", tv.Values())
	}
}

func TestTraversalSubGraph(t *testing.T) {
	g := newTransversalGraph(t)
	ctx := StepContext{}

	tr := NewGraphTraversal(g, false)

	tv := tr.E(ctx).Has(ctx, "Direction", "Left").SubGraph(ctx).V(ctx)
	if len(tv.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v, %s", tv.Values(), tv.Error())
	}

	te := tr.E(ctx).Has(ctx, "Direction", "Left").SubGraph(ctx).E(ctx)
	if len(te.Values()) != 2 {
		t.Fatalf("Should return 2 edges, returned: %v, %s", te.Values(), tv.Error())
	}

	tv = tr.V(ctx).Has(ctx, "Type", "intf").SubGraph(ctx).V(ctx)
	if len(tv.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v, %s", tv.Values(), tv.Error())
	}

	te = tr.V(ctx).Has(ctx, "Type", "intf").SubGraph(ctx).E(ctx)
	if len(te.Values()) != 1 {
		t.Fatalf("Should return 1 edge, returned: %v, %s", te.Values(), tv.Error())
	}
}

func execTraversalQuery(t *testing.T, g *graph.Graph, query string) GraphTraversalStep {
	ts, err := NewGremlinTraversalParser().Parse(strings.NewReader(query))
	if err != nil {
		t.Fatalf("%s: %s", query, err)
	}

	res, err := ts.Exec(g, false)
	if err != nil {
		t.Fatalf("%s: %s", query, err)
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

	query = `G.V().HasEither("Type", "intf", "Name", "Node4")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 3, returned: %v", res.Values())
	}

	query = `G.V().HasEither("Bytes", 1024, "Bytes", 2024)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2, returned: %v", res.Values())
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
	if name, _ := node.GetFieldString("Name"); name != "Node4" {
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
	query = `G.E().Dedup("Direction")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 4 {
		t.Fatalf("Should return 4 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Dedup("Type")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", res.Values())
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
		t.Fatalf("Should return 1 nodes, returned: %v, expected %s", res.Values(), node.ID)
	}

	// next traversal test
	query = `G.V().Has("Value", 1).ShortestPathTo(Metadata("Value", 3), Metadata("Direction", "Left"))`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 path, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("MTU", Ne(1200))`
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
	query = `G.E().Has("Name", "e3").BothV()`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Name", Regex(".*ode.*"))`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Values("Type")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.E().Has("Direction", "Left").SubGraph().V()`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Type", "intf").SubGraph().V()`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("Value", 2).ShortestPathTo(Metadata("Value", 4)).SubGraph().V()`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Has("IPV4", Ipv4Range("192.168.0.0/24"))`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 node, returned: %v", res.Values())
	}
}

func TestLimit(t *testing.T) {
	g := newTransversalGraph(t)

	query := `G.V().Has("Value", NE("ZZZ")).Limit(1)`
	res := execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().HasNot("ZZZ").Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().HasKey("Value").Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().HasKey("Value").Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().Out().Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().In().Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().Both().Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().Dedup().Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().OutE().Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().InE().Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().BothE().Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().BothE().InV().Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	query = `G.V().BothE().OutV().Limit(1)`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}

func TestDedupMultiplefields(t *testing.T) {
	g := newGraph(t)

	g.NewNode(graph.GenID(), graph.Metadata{"Name": "aaa", "Type": "intf", "Value": "v1"})
	g.NewNode(graph.GenID(), graph.Metadata{"Name": "bbb", "Type": "intf", "Value": "v1"})
	g.NewNode(graph.GenID(), graph.Metadata{"Name": "aaa", "Type": "veth"})
	g.NewNode(graph.GenID(), graph.Metadata{"Name": "aaa", "Type": "intf"})

	// next traversal test
	query := `G.V().Dedup("Name")`
	res := execTraversalQuery(t, g, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Dedup("Name", "Type")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Dedup("Value")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", res.Values())
	}

	// next traversal test
	query = `G.V().Dedup("Misc", "Value")`
	res = execTraversalQuery(t, g, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 3 nodes, returned: %v", res.Values())
	}
}
