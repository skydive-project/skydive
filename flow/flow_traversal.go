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

package flow

import (
	"bytes"
	"encoding/json"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

type FlowTraversalExtension struct {
	FlowToken   graph.Token
	TableClient *TableClient
}

type FlowGremlinTraversalStep struct {
	TableClient *TableClient
}

type FlowTraversalStep struct {
	GraphTraversal *graph.GraphTraversal
	flows          []*Flow
}

func (f *FlowTraversalStep) Out(s ...interface{}) *graph.GraphTraversalV {
	var nodes []*graph.Node

	for _, flow := range f.flows {
		if flow.IfDstNodeUUID != "" && flow.IfDstNodeUUID != "*" {
			node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.IfDstNodeUUID))
			if node != nil {
				m, err := graph.SliceToMetadata(s...)
				if err != nil {
					return graph.NewGraphTraversalV(f.GraphTraversal, nodes, err)
				}

				if node.MatchMetadata(m) {
					nodes = append(nodes, node)
				}
			}
		}
	}

	return graph.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) In(s ...interface{}) *graph.GraphTraversalV {
	var nodes []*graph.Node

	for _, flow := range f.flows {
		if flow.IfSrcNodeUUID != "" && flow.IfSrcNodeUUID != "*" {
			node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.IfSrcNodeUUID))
			if node != nil {
				m, err := graph.SliceToMetadata(s...)
				if err != nil {
					return graph.NewGraphTraversalV(f.GraphTraversal, nodes, err)
				}

				if node.MatchMetadata(m) {
					nodes = append(nodes, node)
				}
			}
		}
	}

	return graph.NewGraphTraversalV(f.GraphTraversal, nodes)
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

func NewFlowTraversalExtension(client *TableClient) *FlowTraversalExtension {
	return &FlowTraversalExtension{
		FlowToken:   graph.Token(1001),
		TableClient: client,
	}
}

func (e *FlowTraversalExtension) ScanIdent(s string) (graph.Token, bool) {
	switch s {
	case "FLOWS":
		return e.FlowToken, true
	}
	return graph.IDENT, false
}

func (e *FlowTraversalExtension) ParseStep(t graph.Token, p graph.GremlinTraversalStepParams) (graph.GremlinTraversalStep, error) {
	switch t {
	case e.FlowToken:
		return &FlowGremlinTraversalStep{TableClient: e.TableClient}, nil
	}

	return nil, nil
}

func (s *FlowGremlinTraversalStep) Exec(last graph.GraphTraversalStep) (graph.GraphTraversalStep, error) {
	flows := make([]*Flow, 0)

	switch last.(type) {
	case *graph.GraphTraversalV:
		tv := last.(*graph.GraphTraversalV)
		for _, i := range tv.Values() {
			node := i.(*graph.Node)

			fs, err := s.TableClient.LookupFlowsByProbeNode(node)
			if err != nil {
				logging.GetLogger().Errorf("Error while looking for flows for node: %v, %s", node, err.Error())
				continue
			}
			flows = append(flows, fs...)
		}

		return &FlowTraversalStep{GraphTraversal: tv.GraphTraversal, flows: flows}, nil
	}

	return nil, graph.ExecutionError
}
