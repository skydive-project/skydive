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

const (
	FLOW_TOKEN      traversal.Token = 1001
	BANDWIDTH_TOKEN traversal.Token = 1002
)

type FlowTraversalExtension struct {
	FlowToken      traversal.Token
	BandwidthToken traversal.Token
	TableClient    *flow.TableClient
	Storage        storage.Storage
}

type FlowGremlinTraversalStep struct {
	TableClient *flow.TableClient
	Storage     storage.Storage
	params      []interface{}
}

type FlowTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	flowset        *flow.FlowSet
	error          error
}

type BandwidthGremlinTraversalStep struct {
	TableClient *flow.TableClient
	Storage     storage.Storage
	params      []interface{}
}

type BandwidthTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	bandwidth      *flow.FlowSetBandwidth
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

func (f *FlowTraversalStep) Count(s ...interface{}) *traversal.GraphTraversalValue {
	return traversal.NewGraphTraversalValue(f.GraphTraversal, len(f.flowset.Flows))
}

func paramsToFilter(s ...interface{}) (flow.Filter, error) {
	if len(s) < 2 {
		return nil, errors.New("At least two parameters must be provided")
	}

	if len(s)%2 != 0 {
		return nil, fmt.Errorf("slice must be defined by pair k,v: %v", s)
	}

	filter := &flow.BoolFilter{Op: flow.AND}

	for i := 0; i < len(s); i += 2 {
		k, ok := s[i].(string)
		if !ok {
			return nil, errors.New("keys should be of string type")
		}

		switch v := s[i+1].(type) {
		case *traversal.NEMetadataMatcher:
			filter.Filters = append(filter.Filters, flow.BoolFilter{
				Op: flow.OR,
				Filters: []flow.Filter{
					flow.RangeFilter{Key: k, Lt: v.Value()},
					flow.RangeFilter{Key: k, Gt: v.Value()},
				},
			})
		case *traversal.LTMetadataMatcher:
			filter.Filters = append(filter.Filters, flow.RangeFilter{Key: k, Lt: v.Value()})
		case *traversal.GTMetadataMatcher:
			filter.Filters = append(filter.Filters, flow.RangeFilter{Key: k, Gt: v.Value()})
		case *traversal.GTEMetadataMatcher:
			filter.Filters = append(filter.Filters, flow.RangeFilter{Key: k, Gte: v.Value()})
		case *traversal.LTEMetadataMatcher:
			filter.Filters = append(filter.Filters, flow.RangeFilter{Key: k, Lte: v.Value()})
		case *traversal.InsideMetadataMatcher:
			from, to := v.Value()
			filter.Filters = append(filter.Filters, flow.RangeFilter{Key: k, Gt: from, Lt: to})
		case *traversal.OutsideMetadataMatcher:
			from, to := v.Value()
			filter.Filters = append(filter.Filters, flow.BoolFilter{
				Op: flow.AND,
				Filters: []flow.Filter{
					flow.RangeFilter{Key: k, Lt: from},
					flow.RangeFilter{Key: k, Gt: to},
				},
			})
		case *traversal.BetweenMetadataMatcher:
			from, to := v.Value()
			filter.Filters = append(filter.Filters, flow.RangeFilter{Key: k, Gte: from, Lt: to})
		default:
			filter.Filters = append(filter.Filters, flow.TermFilter{Key: k, Value: v})
		}
	}

	return filter, nil
}

func (f *FlowTraversalStep) Has(s ...interface{}) *FlowTraversalStep {
	if f.error != nil {
		return f
	}

	filter, err := paramsToFilter(s...)
	if err != nil {
		return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, error: err}
	}

	return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, flowset: f.flowset.Filter(filter)}
}

func (f *FlowTraversalStep) Dedup() *FlowTraversalStep {
	flowset := flow.NewFlowSet()
	flowset.Start = f.flowset.Start
	flowset.End = f.flowset.End

	uuids := make(map[string]bool)
	for _, flow := range f.flowset.Flows {
		if _, ok := uuids[flow.TrackingID]; ok {
			continue
		}

		flowset.Flows = append(flowset.Flows, flow)
		uuids[flow.TrackingID] = true
	}

	return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, flowset: flowset}
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
		FlowToken:      FLOW_TOKEN,
		BandwidthToken: BANDWIDTH_TOKEN,
		TableClient:    client,
		Storage:        storage,
	}
}

func (e *FlowTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "FLOWS":
		return e.FlowToken, true
	case "BANDWIDTH":
		return e.BandwidthToken, true
	}
	return traversal.IDENT, false
}

func (e *FlowTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalStepParams) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.FlowToken:
		return &FlowGremlinTraversalStep{
			TableClient: e.TableClient,
			Storage:     e.Storage,
			params:      p.Params(),
		}, nil
	case e.BandwidthToken:
		return &BandwidthGremlinTraversalStep{TableClient: e.TableClient, Storage: e.Storage}, nil
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

		var err error
		var paramsFilter flow.Filter
		if len(s.params) > 0 {
			if paramsFilter, err = paramsToFilter(s.params...); err != nil {
				return nil, err
			}
		}

		flowset := flow.NewFlowSet()
		context := tv.GraphTraversal.Graph.GetContext()
		if context.Time != nil && s.Storage != nil {
			var flows []*flow.Flow
			if flows, err = storage.LookupFlowsByNodes(s.Storage, context, hnmap, paramsFilter); err == nil {
				flowset.Flows = append(flowset.Flows, flows...)
			}
		} else {
			flowset, err = s.TableClient.LookupFlowsByNodes(hnmap, paramsFilter)
		}

		if err != nil {
			logging.GetLogger().Errorf("Error while looking for flows for nodes: %v, %s", hnmap, err.Error())
			return nil, err
		}

		return &FlowTraversalStep{GraphTraversal: tv.GraphTraversal, flowset: flowset}, nil
	}

	return nil, traversal.ExecutionError
}

func (s *FlowGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	if hasStep, ok := next.(*traversal.GremlinTraversalStepHas); ok && len(s.params) == 0 {
		s.params = hasStep.Params()
		return s
	}
	return next
}

func (s *FlowGremlinTraversalStep) Params() (params []interface{}) {
	return s.params
}

func (b *BandwidthTraversalStep) Values() []interface{} {
	return []interface{}{b.bandwidth}
}

func (b *BandwidthTraversalStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.Values())
}

func (b *BandwidthTraversalStep) Error() error {
	return nil
}

func (s *BandwidthGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fs := last.(*FlowTraversalStep)
		bw := fs.flowset.Bandwidth()

		return &BandwidthTraversalStep{GraphTraversal: fs.GraphTraversal, bandwidth: &bw}, nil
	}

	return nil, traversal.ExecutionError
}

func (s *BandwidthGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

func (s *BandwidthGremlinTraversalStep) Params() (params []interface{}) {
	return s.params
}
