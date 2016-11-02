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

package topology

import (
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

func TestMarshal(t *testing.T) {
	g := newGraph(t)

	n1 := g.NewNode(graph.GenID(), graph.Metadata{"Name": "N1", "Type": "T1"})
	n2 := g.NewNode(graph.GenID(), graph.Metadata{"Name": "N2", "Type": "T2"})
	n3 := g.NewNode(graph.GenID(), graph.Metadata{"Name": "N3", "Type": "T3"})

	g.Link(n1, n2)
	g.Link(n2, n3)

	r := g.LookupShortestPath(n3, graph.Metadata{"Name": "N1"})
	if len(r) == 0 {
		t.Errorf("Wrong nodes returned: %v", r)
	}

	path := NodePath(r).Marshal()
	if path != "N1[Type=T1]/N2[Type=T2]/N3[Type=T3]" {
		t.Errorf("Wrong path returned: %s", path)
	}
}
