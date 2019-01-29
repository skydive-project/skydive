/*
 * Copyright (C) 2018 Orange
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
	"sort"
	"testing"
)

// A simple action that we may repeat. More common actions will likely
// check a property of nodes and return false when a matching node has been
// processed.
type action struct {
	count int
}

// ProcessNode for action marks the node (set Mark to true)
func (action *action) ProcessNode(g *Graph, n *Node) bool {
	tr := g.StartMetadataTransaction(n)
	tr.AddMetadata("Mark", true)
	tr.Commit()
	if action.count > 0 {
		action.count = action.count - 1
		return true
	}
	return false
}

// testNode create basic nodes with Num Type and Attr
func testNode(g *Graph, n int, filtered bool, attr string) {
	typ := "Test"
	if !filtered {
		typ = "Other"
	}
	m := Metadata{
		"Num":  n,
		"Type": typ,
		"Attr": attr,
		"Mark": false,
	}
	g.NewNode(GenID(), m, "host")
}

// TestProcessor checks that processor will perform the right number of actions
// on the expected nodes
func TestProcessor(t *testing.T) {
	g := newGraph(t)
	processor := NewProcessor(g, g, Metadata{"Type": "Test"}, "Attr")
	processor.Start()

	testNode(g, 0, true, "A")                  // marked by action 1
	testNode(g, 1, true, "B")                  // not marked wrong attr
	testNode(g, 2, false, "A")                 // not marked wrong type
	processor.DoAction(&action{count: 0}, "A") // action 1
	testNode(g, 3, true, "A")                  // not marked found immediately
	processor.DoAction(&action{count: 0}, "C") // action 2
	testNode(g, 4, true, "B")                  // not marked wrong attr
	testNode(g, 5, false, "C")                 // not marked wrong type
	testNode(g, 6, true, "C")                  // marked action 2
	testNode(g, 7, true, "D")                  // marked action 3
	processor.DoAction(&action{count: 2}, "D") // action 3
	testNode(g, 8, true, "D")                  // marked action 3
	testNode(g, 9, true, "D")                  // marked action 3
	testNode(g, 10, true, "D")                 // not marked exhausted
	nodes := g.GetNodes(Metadata{"Mark": true})
	result := make([]int, len(nodes))
	for i, node := range nodes {
		result[i], _ = node.Metadata["Num"].(int)
	}
	sort.Ints(result)
	expected := []int{0, 6, 7, 8, 9}
	if len(result) != len(expected) {
		t.Errorf("Got %d instead of %d elements", len(result), len(expected))
	}
	for i, v := range result {
		e := -1
		if i < len(expected) {
			e = expected[i]
		}
		if v != e {
			t.Errorf("Expected %d at position %d, got %d", e, i, v)
		}
	}
}
