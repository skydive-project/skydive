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

	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

type TopologyTraversalExtension struct {
	graphPathToken traversal.Token
}

type GraphPathGremlinTraversalStep struct {
}

type GraphPathTraversalStep struct {
	paths []NodePath
}

func (p *GraphPathTraversalStep) Values() []interface{} {
	s := make([]interface{}, len(p.paths))
	for i, gp := range p.paths {
		s[i] = gp.Marshal()
	}
	return s
}

func (p *GraphPathTraversalStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Values())
}

func (p *GraphPathTraversalStep) Error() error {
	return nil
}

func NewTopologyTraversalExtension() *TopologyTraversalExtension {
	return &TopologyTraversalExtension{
		graphPathToken: traversal.Token(1000),
	}
}

func (e *TopologyTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "GRAPHPATH":
		return e.graphPathToken, true
	}
	return traversal.IDENT, false
}

func (e *TopologyTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.graphPathToken:
		return &GraphPathGremlinTraversalStep{}, nil
	}

	return nil, nil
}

func (s *GraphPathGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
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

		return &GraphPathTraversalStep{paths: paths}, nil
	}

	return nil, traversal.ExecutionError
}

func (s *GraphPathGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

func (s *GraphPathGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &traversal.GremlinTraversalContext{}
}
