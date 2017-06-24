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
	"encoding/json"
	"strings"

	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

// TopologyTraversalExtension describes a new extension to enhance the topology
type TopologyTraversalExtension struct {
	graphPathToken traversal.Token
}

type graphPathGremlinTraversalStep struct {
	traversal.GremlinTraversalStep
}

type graphPathTraversalStep struct {
	traversal.GraphTraversalStep
	paths []NodePath
}

func (p *graphPathTraversalStep) Values() []interface{} {
	s := make([]interface{}, len(p.paths))
	for i, gp := range p.paths {
		s[i] = gp.Marshal()
	}
	return s
}

func (p *graphPathTraversalStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Values())
}

func (p *graphPathTraversalStep) Error() error {
	return nil
}

// NewTopologyTraversalExtension returns a new graph traversal mechanism externsion
func NewTopologyTraversalExtension() *TopologyTraversalExtension {
	return &TopologyTraversalExtension{
		graphPathToken: traversal.Token(1000),
	}
}

// ScanIdent returns an associated graph token
func (e *TopologyTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "GRAPHPATH":
		return e.graphPathToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parse the current step
func (e *TopologyTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.graphPathToken:
		return &graphPathGremlinTraversalStep{}, nil
	}

	return nil, nil
}

func (s *graphPathGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	paths := []NodePath{}

	switch last.(type) {
	case *traversal.GraphTraversalV:
		tv := last.(*traversal.GraphTraversalV)
		for _, i := range tv.Values() {
			node := i.(*graph.Node)

			nodes := tv.GraphTraversal.Graph.LookupShortestPath(node, graph.Metadata{"Type": "host"}, graph.Metadata{"RelationType": "ownership"})
			if len(nodes) > 0 {
				paths = append(paths, NodePath(nodes))
			}
		}

		return &graphPathTraversalStep{paths: paths}, nil
	}

	return nil, traversal.ErrExecutionError
}

func (s *graphPathGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

func (s *graphPathGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &traversal.GremlinTraversalContext{}
}

// ExecuteGremlinQuery run a gremlin query on the graph g
func ExecuteGremlinQuery(g *graph.Graph, query string) (traversal.GraphTraversalStep, error) {
	tr := traversal.NewGremlinTraversalParser(g)
	tr.AddTraversalExtension(NewTopologyTraversalExtension())
	ts, err := tr.Parse(strings.NewReader(query), false)
	if err != nil {
		return nil, err
	}

	return ts.Exec()
}
