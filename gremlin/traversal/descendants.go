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

// DescendantsTraversalExtension describes a new extension to enhance the topology
type DescendantsTraversalExtension struct {
	DescendantsToken traversal.Token
}

// DescendantsGremlinTraversalStep rawpackets step
type DescendantsGremlinTraversalStep struct {
	context    traversal.GremlinTraversalContext
	maxDepth   int64
	edgeFilter graph.ElementMatcher
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

	maxDepth := int64(1)
	edgeFilter, _ := topology.OwnershipMetadata().Filter()

	switch len(p.Params) {
	case 0:
	default:
		i := len(p.Params) / 2 * 2
		filter, err := traversal.ParamsToFilter(filters.BoolFilterOp_OR, p.Params[:i]...)
		if err != nil {
			return nil, errors.Wrap(err, "Descendants accepts an optional number of key/value tuples and an optional depth")
		}
		edgeFilter = filter

		if i == len(p.Params) {
			break
		}

		fallthrough
	case 1:
		depth, ok := p.Params[len(p.Params)-1].(int64)
		if !ok {
			return nil, errors.New("Descendants last argument must be the maximum depth specified as an integer")
		}
		maxDepth = depth
	}

	return &DescendantsGremlinTraversalStep{context: p, maxDepth: maxDepth, edgeFilter: graph.NewElementFilter(edgeFilter)}, nil
}

func getDescendants(g *graph.Graph, parents []*graph.Node, descendants *[]*graph.Node, currDepth, maxDepth int64, edgeFilter graph.ElementMatcher, visited map[graph.Identifier]bool) {
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
			children := g.LookupChildren(parent, nil, edgeFilter)
			getDescendants(g, children, descendants, currDepth+1, maxDepth, edgeFilter, visited)
		}
	}
}

// Exec Descendants step
func (d *DescendantsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	var descendants []*graph.Node

	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		tv.GraphTraversal.RLock()
		getDescendants(tv.GraphTraversal.Graph, tv.GetNodes(), &descendants, 0, d.maxDepth, d.edgeFilter, make(map[graph.Identifier]bool))
		tv.GraphTraversal.RUnlock()

		return traversal.NewGraphTraversalV(tv.GraphTraversal, descendants), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce Descendants step
func (d *DescendantsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context Descendants step
func (d *DescendantsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &d.context
}
