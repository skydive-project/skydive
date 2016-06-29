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
	"bytes"
	"encoding/json"

	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/storage"
	"github.com/redhat-cip/skydive/topology/graph"
	"github.com/redhat-cip/skydive/topology/graph/traversal"
)

type FlowTraversalExtension struct {
	FlowToken   traversal.Token
	TableClient *flow.TableClient
	Storage     storage.Storage
}

type FlowGremlinTraversalStep struct {
	TableClient *flow.TableClient
	Storage     storage.Storage
}

type FlowTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	flows          []*flow.Flow
}

func (f *FlowTraversalStep) Out(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	for _, flow := range f.flows {
		if flow.IfDstNodeUUID != "" && flow.IfDstNodeUUID != "*" {
			node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.IfDstNodeUUID))
			if node != nil {
				m, err := traversal.SliceToMetadata(s...)
				if err != nil {
					return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
				}

				if node.MatchMetadata(m) {
					nodes = append(nodes, node)
				}
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) In(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	for _, flow := range f.flows {
		if flow.IfSrcNodeUUID != "" && flow.IfSrcNodeUUID != "*" {
			node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.IfSrcNodeUUID))
			if node != nil {
				m, err := traversal.SliceToMetadata(s...)
				if err != nil {
					return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
				}

				if node.MatchMetadata(m) {
					nodes = append(nodes, node)
				}
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) Values() []interface{} {
	a := make([]interface{}, len(f.flows))
	for i, flow := range f.flows {
		a[i] = flow
	}
	return a
}

func (f *FlowTraversalStep) MarshalJSON() ([]byte, error) {
	a := make([]interface{}, len(f.flows))
	for i, flow := range f.flows {
		b, err := json.Marshal(flow)
		if err != nil {
			logging.GetLogger().Errorf("Error while converting flow to JSON: %v", flow)
			continue
		}

		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()

		var x map[string]interface{}
		err = d.Decode(&x)
		if err != nil {
			logging.GetLogger().Errorf("Error while converting flow to JSON: %v", flow)
			continue
		}

		// substitute UUID by the node
		if flow.IfSrcNodeUUID != "" && flow.IfSrcNodeUUID != "*" {
			node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.IfSrcNodeUUID))
			if node != nil {
				x["IfSrcNode"] = node
			}
		}

		if flow.IfDstNodeUUID != "" && flow.IfDstNodeUUID != "*" {
			node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.IfDstNodeUUID))
			if node != nil {
				x["IfDstNode"] = node
			}
		}

		node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.ProbeNodeUUID))
		if node != nil {
			x["ProbeNode"] = node
		}

		a[i] = x
	}
	return json.Marshal(a)
}

func (p *FlowTraversalStep) Error() error {
	return nil
}

func NewFlowTraversalExtension(client *flow.TableClient, storage storage.Storage) *FlowTraversalExtension {
	return &FlowTraversalExtension{
		FlowToken:   traversal.Token(1001),
		TableClient: client,
		Storage:     storage,
	}
}

func (e *FlowTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "FLOWS":
		return e.FlowToken, true
	}
	return traversal.IDENT, false
}

func (e *FlowTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalStepParams) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.FlowToken:
		return &FlowGremlinTraversalStep{TableClient: e.TableClient, Storage: e.Storage}, nil
	}

	return nil, nil
}

func (s *FlowGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *traversal.GraphTraversalV:
		tv := last.(*traversal.GraphTraversalV)

		nodes := make([]*graph.Node, len(tv.Values()))
		for i, v := range tv.Values() {
			nodes[i] = v.(*graph.Node)
		}

		var flows []*flow.Flow
		var err error

		context := tv.GraphTraversal.Graph.GetContext()
		if context.Time != nil && s.Storage != nil {
			filters := storage.NewFilters()
			filters.Range["Statistics.Start"] = storage.RangeFilter{Lte: context.Time.Unix()}
			filters.Range["Statistics.Last"] = storage.RangeFilter{Gte: context.Time.Unix()}

			filters.Term.Op = storage.OR
			for _, node := range nodes {
				filters.Term.Terms["ProbeNodeUUID"] = node.ID
				filters.Term.Terms["IfSrcNodeUUID"] = node.ID
				filters.Term.Terms["IfDstNodeUUID"] = node.ID

				f, err := s.Storage.SearchFlows(filters)
				if err != nil {
					return nil, traversal.ExecutionError
				}
				flows = append(flows, f...)
			}
		} else {
			flows, err = s.TableClient.LookupFlowsByNode(nodes...)
		}
		if err != nil {
			logging.GetLogger().Errorf("Error while looking for flows for nodes: %v, %s", nodes, err.Error())
			return nil, traversal.ExecutionError
		}
		return &FlowTraversalStep{GraphTraversal: tv.GraphTraversal, flows: flows}, nil
	}

	return nil, traversal.ExecutionError
}
