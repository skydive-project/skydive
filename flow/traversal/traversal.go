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
	"errors"
	"fmt"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/storage"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
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
	flowset        *flow.FlowSet
	error          error
}

func (f *FlowTraversalStep) Out(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	for _, flow := range f.flowset.Flows {
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

	for _, flow := range f.flowset.Flows {
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

func (f *FlowTraversalStep) Has(s ...interface{}) *FlowTraversalStep {
	if f.error != nil {
		return f
	}

	if len(s) < 2 {
		return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, error: errors.New("At least two parameters must be provided")}
	}

	if len(s)%2 != 0 {
		return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, error: fmt.Errorf("slice must be defined by pair k,v: %v", s)}
	}

	filters := storage.NewFilters()
	terms := flow.TermFilter{Op: flow.AND}

	for i := 0; i < len(s); i += 2 {
		k, ok := s[i].(string)
		if !ok {
			return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, error: errors.New("keys should be of string type")}
		}

		switch v := s[i+1].(type) {
		case *traversal.NEMetadataMatcher:
			filters.Range[k] = flow.RangeFilter{Lt: v.Value(), Gt: v.Value()}
		case *traversal.LTMetadataMatcher:
			filters.Range[k] = flow.RangeFilter{Lt: v.Value()}
		case *traversal.GTMetadataMatcher:
			filters.Range[k] = flow.RangeFilter{Gt: v.Value()}
		case *traversal.GTEMetadataMatcher:
			filters.Range[k] = flow.RangeFilter{Gte: v.Value()}
		case *traversal.LTEMetadataMatcher:
			filters.Range[k] = flow.RangeFilter{Lte: v.Value()}
		case *traversal.InsideMetadataMatcher:
			from, to := v.Value()
			filters.Range[k] = flow.RangeFilter{Gt: from, Lt: to}
		case *traversal.BetweenMetadataMatcher:
			from, to := v.Value()
			filters.Range[k] = flow.RangeFilter{Gte: from, Lt: to}
		default:
			terms.Terms = append(terms.Terms, flow.Term{Key: k, Value: v})
		}
	}

	filters.Term = terms
	fs := f.flowset.Filter(filters)
	return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, flowset: fs}
}

func (f *FlowTraversalStep) Values() []interface{} {
	a := make([]interface{}, len(f.flowset.Flows))
	for i, flow := range f.flowset.Flows {
		a[i] = flow
	}
	return a
}

func (f *FlowTraversalStep) MarshalJSON() ([]byte, error) {
	a := make([]interface{}, len(f.flowset.Flows))
	for i, flow := range f.flowset.Flows {
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

func (f *FlowTraversalStep) Error() error {
	return f.error
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

		tv.GraphTraversal.Graph.Lock()
		hnmap := make(flow.HostNodeIDMap)
		for _, v := range tv.Values() {
			node := v.(*graph.Node)
			hnmap[node.Host()] = append(hnmap[node.Host()], string(node.ID))
		}
		tv.GraphTraversal.Graph.Unlock()

		flowset := flow.NewFlowSet()
		var err error

		context := tv.GraphTraversal.Graph.GetContext()
		if context.Time != nil && s.Storage != nil {
			filters := storage.NewFilters()
			filters.Range["Statistics.Start"] = flow.RangeFilter{Lte: context.Time.Unix()}
			filters.Range["Statistics.Last"] = flow.RangeFilter{Gte: context.Time.Unix()}
			filters.Term.Op = flow.OR
			for _, ids := range hnmap {
				for _, id := range ids {
					filters.Term.Terms = []flow.Term{
						{Key: "ProbeNodeUUID", Value: id},
						{Key: "IfSrcNodeUUID", Value: id},
						{Key: "IfDstNodeUUID", Value: id},
					}

					f, err := s.Storage.SearchFlows(filters)
					if err != nil {
						return nil, traversal.ExecutionError
					}
					flowset.Flows = append(flowset.Flows, f...)
				}
			}
		} else {
			flowset, err = s.TableClient.LookupFlowsByNodes(hnmap)
		}
		if err != nil {
			logging.GetLogger().Errorf("Error while looking for flows for nodes: %v, %s", hnmap, err.Error())
			return nil, traversal.ExecutionError
		}
		return &FlowTraversalStep{GraphTraversal: tv.GraphTraversal, flowset: flowset}, nil
	}

	return nil, traversal.ExecutionError
}

func (s *FlowGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}
