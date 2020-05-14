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

	"github.com/mitchellh/mapstructure"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/js"
)

// WorkflowTraversalExtension describes a new extension to enhance the topology
type WorkflowTraversalExtension struct {
	WorkflowToken traversal.Token
	handler       rest.Handler
	jsre          *js.Runtime
}

// WorkflowGremlinTraversalStep workflow step
type WorkflowGremlinTraversalStep struct {
	context   traversal.GremlinTraversalContext
	extension *WorkflowTraversalExtension
	idOrName  string
}

// NewWorkflowTraversalExtension returns a new graph traversal extension
func NewWorkflowTraversalExtension(handler rest.Handler, jsre *js.Runtime) *WorkflowTraversalExtension {
	return &WorkflowTraversalExtension{
		WorkflowToken: traversalWorkflowToken,
		handler:       handler,
		jsre:          jsre,
	}
}

// ScanIdent returns an associated graph token
func (e *WorkflowTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "WORKFLOW":
		return e.WorkflowToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parse metrics step
func (e *WorkflowTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.WorkflowToken:
		if len(p.Params) < 1 {
			return nil, fmt.Errorf("Workflow requires at least 1 parameter")
		}

		idOrName, ok := p.Params[0].(string)
		if !ok {
			return nil, fmt.Errorf("Workflow accepts a workflow id or name as first parameter")
		}

		return &WorkflowGremlinTraversalStep{context: p, extension: e, idOrName: idOrName}, nil
	}
	return nil, nil
}

// Exec Workflow step
func (s *WorkflowGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *traversal.GraphTraversal:
		extension := s.extension
		resource, found := extension.handler.Get(s.idOrName)
		if !found {
			for _, w := range extension.handler.Index() {
				workflow := w.(*types.Workflow)
				if workflow.Name == s.idOrName {
					resource = w
				}
			}
		}
		if resource == nil {
			return nil, fmt.Errorf("Unknown workflow '%s", s.idOrName)
		}

		workflow := resource.(*types.Workflow)
		jsre := extension.jsre

		value, err := jsre.ExecFunction(workflow.Source, s.context.Params[1:])
		if err != nil {
			return nil, fmt.Errorf("error while executing workflow: %s", err)
		}

		memory, err := graph.NewMemoryBackend()
		if err != nil {
			return nil, err
		}

		jsre.Set("result", value)
		jsre.Exec("console.log(typeof(result))")
		jsre.Exec("console.log(JSON.stringify(result))")

		ng := graph.NewGraph("", memory, "")

		obj := value.Object()
		nodes, err1 := obj.Get("nodes")
		edges, err2 := obj.Get("edges")

		if err1 == nil && err2 == nil {
			nodesObj := nodes.Object()

			for _, key := range nodesObj.Keys() {
				var n graph.Node
				node, err := nodes.Object().Get(key)
				if err == nil {
					v, err := node.Export()
					if err != nil {
						return nil, fmt.Errorf("failed to export node: %s", err)
					}
					if mapstructure.WeakDecode(v, &n); err != nil {
						return nil, fmt.Errorf("failed to decode node: %s", err)
					}
					memory.NodeAdded(&n)
				}
			}

			edgesObj := edges.Object()
			for _, key := range edgesObj.Keys() {
				var e graph.Edge
				edge, err := edges.Object().Get(key)
				if err == nil {
					v, err := edge.Export()
					if err != nil {
						return nil, fmt.Errorf("failed to export edge: %s", err)
					}
					if mapstructure.WeakDecode(v, &e); err != nil {
						return nil, fmt.Errorf("failed to decode edge: %s", err)
					}
					memory.EdgeAdded(&e)
				}
			}
		}

		return traversal.NewGraphTraversal(ng, false), nil
	}

	return nil, traversal.ErrExecutionError
}

// Reduce Workflow step
func (s *WorkflowGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context Workflow step
func (s *WorkflowGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}
