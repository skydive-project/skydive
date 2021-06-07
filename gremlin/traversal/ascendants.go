/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"github.com/pkg/errors"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology"
)

// AscendantsTraversalExtension describes a new extension to enhance the topology
type AscendantsTraversalExtension struct {
	AscendantsToken traversal.Token
}

// AscendantsGremlinTraversalStep rawpackets step
type AscendantsGremlinTraversalStep struct {
	context    traversal.GremlinTraversalContext
	maxDepth   int64
	edgeFilter graph.ElementMatcher
}

// NewAscendantsTraversalExtension returns a new graph traversal extension
func NewAscendantsTraversalExtension() *AscendantsTraversalExtension {
	return &AscendantsTraversalExtension{
		AscendantsToken: traversalAscendantsToken,
	}
}

// ScanIdent returns an associated graph token
func (e *AscendantsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "ASCENDANTS":
		return e.AscendantsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parses ascendants step
func (e *AscendantsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.AscendantsToken:
	default:
		return nil, nil
	}

	maxDepth := int64(1)
	edgeFilter, _ := topology.OwnershipMetadata().Filter()

	switch len(p.Params) {
	case 0:
	default:
		i := len(p.Params) / 2 * 2
		filter, err := traversal.ParamsToFilter(filters.BoolFilterOp_OR, p.Params[:i]...)
		if err != nil {
			return nil, errors.Wrap(err, "Ascendants accepts an optional number of key/value tuples and an optional depth")
		}
		edgeFilter = filter

		if i == len(p.Params) {
			break
		}

		fallthrough
	case 1:
		depth, ok := p.Params[len(p.Params)-1].(int64)
		if !ok {
			return nil, errors.New("Ascendants last argument must be the maximum depth specified as an integer")
		}
		maxDepth = depth
	}

	return &AscendantsGremlinTraversalStep{context: p, maxDepth: maxDepth, edgeFilter: graph.NewElementFilter(edgeFilter)}, nil
}

// getAscendants given a list of nodes, add them to the ascendants list, get the ascendants of that nodes and call this function with those new nodes
func getAscendants(g *graph.Graph, nodes []*graph.Node, ascendants *[]*graph.Node, currDepth, maxDepth int64, edgeFilter graph.ElementMatcher, visited map[graph.Identifier]bool) {
	var ld []*graph.Node
	for _, node := range nodes {
		if _, ok := visited[node.ID]; !ok {
			ld = append(ld, node)
			visited[node.ID] = true
		}
	}
	*ascendants = append(*ascendants, ld...)

	if maxDepth == 0 || currDepth < maxDepth {
		for _, parent := range nodes {
			parents := g.LookupParents(parent, nil, edgeFilter)
			getAscendants(g, parents, ascendants, currDepth+1, maxDepth, edgeFilter, visited)
		}
	}
}

// Exec Ascendants step
func (d *AscendantsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	var ascendants []*graph.Node

	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		tv.GraphTraversal.RLock()
		getAscendants(tv.GraphTraversal.Graph, tv.GetNodes(), &ascendants, 0, d.maxDepth, d.edgeFilter, make(map[graph.Identifier]bool))
		tv.GraphTraversal.RUnlock()

		return traversal.NewGraphTraversalV(tv.GraphTraversal, ascendants), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce Ascendants step
func (d *AscendantsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context Ascendants step
func (d *AscendantsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &d.context
}
