/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"fmt"

	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

// DescendantsTraversalExtension describes a new extension to enhance the topology
type DescendantsTraversalExtension struct {
	DescendantsToken traversal.Token
}

// DescendantsGremlinTraversalStep rawpackets step
type DescendantsGremlinTraversalStep struct {
	context  traversal.GremlinTraversalContext
	maxDepth int64
}

// NewDescendantsTraversalExtension returns a new graph traversal extension
func NewDescendantsTraversalExtension() *DescendantsTraversalExtension {
	return &DescendantsTraversalExtension{
		DescendantsToken: traversalDescendantsToken,
	}
}

// ScanIdent returns an associated graph token
func (e *DescendantsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "DESCENDANTS":
		return e.DescendantsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parses descendants step
func (e *DescendantsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.DescendantsToken:
	default:
		return nil, nil
	}

	paramErr := fmt.Errorf("Descendants requires 1 number as parameter : %v", p.Params)

	maxDepth := int64(1)
	switch len(p.Params) {
	case 0:
	case 1:
		depth, ok := p.Params[0].(int64)
		if !ok {
			return nil, paramErr
		}
		maxDepth = depth
	default:
		return nil, paramErr
	}

	return &DescendantsGremlinTraversalStep{context: p, maxDepth: maxDepth}, nil
}

func getDescendants(g *graph.Graph, parents []*graph.Node, descendants *[]*graph.Node, currDepth, maxDepth int64, visited map[graph.Identifier]bool) {
	var ld []*graph.Node
	for _, parent := range parents {
		if _, ok := visited[parent.ID]; !ok {
			ld = append(ld, parent)
			visited[parent.ID] = true
		}
	}
	*descendants = append(*descendants, ld...)

	if maxDepth == 0 || currDepth < maxDepth {
		for _, parent := range parents {
			children := g.LookupChildren(parent, nil, topology.OwnershipMetadata())
			getDescendants(g, children, descendants, currDepth+1, maxDepth, visited)
		}
	}
}

// Exec Descendants step
func (d *DescendantsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	var descendants []*graph.Node

	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		tv.GraphTraversal.RLock()
		getDescendants(tv.GraphTraversal.Graph, tv.GetNodes(), &descendants, 0, d.maxDepth, make(map[graph.Identifier]bool))
		tv.GraphTraversal.RUnlock()

		return traversal.NewGraphTraversalV(tv.GraphTraversal, descendants), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce Descendants step
func (d *DescendantsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

// Context Descendants step
func (d *DescendantsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &d.context
}
