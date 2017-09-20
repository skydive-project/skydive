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
	"reflect"
	"strings"
	"testing"

	"github.com/skydive-project/skydive/topology/graph"
)

func TestOrStepEmpty(t *testing.T) {
	g := newTransversalGraph(t)
	_, err := NewGremlinTraversalParser(g).Parse(strings.NewReader(`G.V().Or()`), false)
	if err == nil {
		t.Fatal("Expected error, got some result")
	}
}

func TestOrStepSimple(t *testing.T) {
	g := newTransversalGraph(t)

	expected := []interface{}{
		queryForValues(`G.V().Has('Value', 1)`, g, count(1), t),
		queryForValues(`G.V().Has('Value', 2)`, g, count(1), t),
	}

	actual := queryForValues(`G.V().Or(Has('Value', 1), Or(Has('Value', 2)))`, g, count(2), t)

	if reflect.DeepEqual(actual, expected) {
		t.Fatalf("Expected %s value(s), actual: %s", expected, actual)
	}
}

func TestOrStepNested(t *testing.T) {
	g := newTransversalGraph(t)

	actual := queryForValues(`G.V().Or(Has('Value', 1), Or(Has('Value', 2), Has('Value', 3)))`, g, count(3), t)

	expected := []interface{}{
		queryForValues(`G.V().Has('Value', 1)`, g, count(1), t),
		queryForValues(`G.V().Has('Value', 2)`, g, count(1), t),
		queryForValues(`G.V().Has('Value', 3)`, g, count(1), t),
	}

	if reflect.DeepEqual(actual, expected) {
		t.Fatalf("Expected %s value(s), actual: %s", expected, actual)
	}
}

// OR step should work on previous selection, not from start
// so for query 'has(type=intf).or(Value=2, Value=3)' we only want n2 (and not n3):
// n2 := g.NewNode(graph.GenID(), graph.Metadata{"Value": 2, "Type": "intf", "Bytes": 2024})
// n3 := g.NewNode(graph.GenID(), graph.Metadata{"Value": 3})
func TestOrStepStartsWithPreviousStep(t *testing.T) {
	expectedCount := 1
	queryForValues(`G.V().Has('Type', 'intf').Or(Has('Value', 2), Has('Value', 3))`, newTransversalGraph(t), count(expectedCount), t)
}

func TestOrStepWithDifferentNestedSteps(t *testing.T) {
	g := newTransversalGraph(t)

	actual := queryForValues(`G.V().Or(Has('Value', 2).In(), Has('Value', 3).Out())`, g, count(2), t)

	expected := []interface{}{
		queryForValues(`G.V().Has('Value', 1)`, g, count(1), t), // inbound for 2
		queryForValues(`G.V().Has('Value', 4)`, g, count(1), t), // outbound for 3
	}

	if reflect.DeepEqual(actual, expected) {
		t.Fatalf("Expected %s value(s), actual: %s", expected, actual)
	}
}

// --------------------------------------------------------------------------------
// helper methods
// todo: use unit-testing library

func count(expectedCount int) func(s GraphTraversalStep, t *testing.T) {
	return func(s GraphTraversalStep, t *testing.T) {
		if len(s.Values()) != expectedCount {
			t.Fatalf("Expected %v value(s), actual: %v", expectedCount, len(s.Values()))
		}
	}
}

// todo: use in all tests
func queryForValues(query string, g *graph.Graph, check func(GraphTraversalStep, *testing.T), t *testing.T) []interface{} {
	res := execTraversalQuery(t, g, query)
	check(res, t)
	return res.Values()
}
