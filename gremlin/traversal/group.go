/*
 * Copyright (C) 2019 Red Hat, Inc.
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
	"encoding/json"
	"errors"
	"strconv"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
)

// GroupTraversalExtension describes a new extension to enhance the topology
type GroupTraversalExtension struct {
	MoreThanToken traversal.Token
}

// GroupGremlinTraversalStep group flows step
type GroupGremlinTraversalStep struct {
	traversal.GremlinTraversalContext
}

// MoreThanGremlinTraversalStep group flows step
type MoreThanGremlinTraversalStep struct {
	traversal.GremlinTraversalContext
}

// GroupTraversalStep group flows step
type GroupTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	flowGroupSet   map[string][]*flow.Flow
	error          error
}

// NewGroupTraversalExtension returns a new graph traversal extension
func NewGroupTraversalExtension() *GroupTraversalExtension {
	return &GroupTraversalExtension{
		MoreThanToken: traversalMoreThanToken,
	}
}

// ScanIdent returns an associated graph token
func (e *GroupTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "MORETHAN":
		return e.MoreThanToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parse metrics step
func (e *GroupTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.MoreThanToken:
		return &MoreThanGremlinTraversalStep{GremlinTraversalContext: p}, nil
	}
	return nil, nil
}

// Exec Group step
func (g *GroupGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fs := last.(*FlowTraversalStep)
		return fs.Group(g.StepContext), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce Group step
func (g *GroupGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context Group step
func (g *GroupGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &g.GremlinTraversalContext
}

// Exec MoreThan step
func (g *MoreThanGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *GroupTraversalStep:
		group := last.(*GroupTraversalStep)
		return group.MoreThan(g.StepContext), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce MoreThan step
func (g *MoreThanGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context MoreThan step
func (g *MoreThanGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &g.GremlinTraversalContext
}

// Values returns list of raw packets
func (ts *GroupTraversalStep) Values() []interface{} {
	ts.GraphTraversal.RLock()
	defer ts.GraphTraversal.RUnlock()
	if len(ts.flowGroupSet) == 0 {
		return []interface{}{}
	}
	return []interface{}{ts.flowGroupSet}
}

// MarshalJSON serialize in JSON
func (ts *GroupTraversalStep) MarshalJSON() ([]byte, error) {
	values := ts.Values()
	ts.GraphTraversal.RLock()
	defer ts.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

// Error returns traversal error
func (ts *GroupTraversalStep) Error() error {
	return ts.error
}

// MoreThan returns only group that contain more than one flow
func (ts *GroupTraversalStep) MoreThan(ctx traversal.StepContext, s ...interface{}) *GroupTraversalStep {
	if ts.error != nil {
		return &GroupTraversalStep{error: ts.error}
	}

	if len(s) > 1 {
		return &GroupTraversalStep{error: errors.New("MoreThan do require 0 or 1 parameter")}
	}
	moreThan := 1
	if len(s) > 0 {
		var err error
		moreThan, err = strconv.Atoi(s[0].(string))
		if err != nil {
			return &GroupTraversalStep{error: err}
		}
		if moreThan < 0 {
			return &GroupTraversalStep{error: errors.New("MoreThan parameter must be positive")}
		}
	}

	for key, flows := range ts.flowGroupSet {
		if len(flows) <= moreThan {
			delete(ts.flowGroupSet, key)
		}
	}

	return &GroupTraversalStep{GraphTraversal: ts.GraphTraversal, flowGroupSet: ts.flowGroupSet}
}
