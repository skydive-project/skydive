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

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/storage"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

const (
	FLOW_TOKEN      traversal.Token = 1001
	BANDWIDTH_TOKEN traversal.Token = 1002
	HOPS_TOKEN      traversal.Token = 1003
	NODES_TOKEN     traversal.Token = 1004
)

type FlowTraversalExtension struct {
	FlowToken      traversal.Token
	BandwidthToken traversal.Token
	HopsToken      traversal.Token
	NodesToken     traversal.Token
	TableClient    *flow.TableClient
	Storage        storage.Storage
}

type FlowGremlinTraversalStep struct {
	TableClient *flow.TableClient
	Storage     storage.Storage
	context     traversal.GremlinTraversalContext
}

type FlowTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	flowset        *flow.FlowSet
	error          error
}

type BandwidthGremlinTraversalStep struct {
	TableClient *flow.TableClient
	Storage     storage.Storage
	context     traversal.GremlinTraversalContext
}

type BandwidthTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	bandwidth      *flow.FlowSetBandwidth
}

type HopsGremlinTraversalStep struct {
	TableClient *flow.TableClient
	Storage     storage.Storage
	context     traversal.GremlinTraversalContext
}

type NodesGremlinTraversalStep struct {
	TableClient *flow.TableClient
	Storage     storage.Storage
	context     traversal.GremlinTraversalContext
}

func (f *FlowTraversalStep) Out(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, flow := range f.flowset.Flows {
		if flow.BNodeUUID != "" && flow.BNodeUUID != "*" {
			if node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.BNodeUUID)); node != nil && node.MatchMetadata(m) {
				nodes = append(nodes, node)
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) In(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, flow := range f.flowset.Flows {
		if flow.ANodeUUID != "" && flow.ANodeUUID != "*" {
			if node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.ANodeUUID)); node != nil && node.MatchMetadata(m) {
				nodes = append(nodes, node)
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) Both(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, flow := range f.flowset.Flows {
		if flow.ANodeUUID != "" && flow.ANodeUUID != "*" {
			if node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.ANodeUUID)); node != nil && node.MatchMetadata(m) {
				nodes = append(nodes, node)
			}
		}
		if flow.BNodeUUID != "" && flow.BNodeUUID != "*" {
			if node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.BNodeUUID)); node != nil && node.MatchMetadata(m) {
				nodes = append(nodes, node)
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) Nodes(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		logging.GetLogger().Critical(err)
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, flow := range f.flowset.Flows {
		if flow.NodeUUID != "" && flow.NodeUUID != "*" {
			if node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.NodeUUID)); node != nil && node.MatchMetadata(m) {
				nodes = append(nodes, node)
			}
		}
		if flow.ANodeUUID != "" && flow.ANodeUUID != "*" {
			if node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.ANodeUUID)); node != nil && node.MatchMetadata(m) {
				nodes = append(nodes, node)
			}
		}
		if flow.BNodeUUID != "" && flow.BNodeUUID != "*" {
			if node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.BNodeUUID)); node != nil && node.MatchMetadata(m) {
				nodes = append(nodes, node)
			}
		}
	}
	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) Count(s ...interface{}) *traversal.GraphTraversalValue {
	return traversal.NewGraphTraversalValue(f.GraphTraversal, len(f.flowset.Flows))
}

func paramsToFilter(s ...interface{}) (*flow.Filter, error) {
	if len(s) < 2 {
		return nil, errors.New("At least two parameters must be provided")
	}

	if len(s)%2 != 0 {
		return nil, fmt.Errorf("slice must be defined by pair k,v: %v", s)
	}

	andFilter := &flow.BoolFilter{Op: flow.BoolFilterOp_AND}

	for i := 0; i < len(s); i += 2 {
		k, ok := s[i].(string)
		if !ok {
			return nil, errors.New("keys should be of string type")
		}

		switch v := s[i+1].(type) {
		case *traversal.NEMetadataMatcher:
			notFilters := &flow.Filter{}
			switch t := v.Value().(type) {
			case string:
				notFilters.TermStringFilter = &flow.TermStringFilter{Key: k, Value: t}
			case int64:
				notFilters.TermInt64Filter = &flow.TermInt64Filter{Key: k, Value: t}
			}

			andFilter.Filters = append(andFilter.Filters,
				&flow.Filter{
					BoolFilter: &flow.BoolFilter{
						Op:      flow.BoolFilterOp_NOT,
						Filters: []*flow.Filter{notFilters},
					},
				},
			)
		case *traversal.LTMetadataMatcher:
			switch t := v.Value().(type) {
			case int64:
				andFilter.Filters = append(andFilter.Filters,
					&flow.Filter{
						LtInt64Filter: &flow.LtInt64Filter{Key: k, Value: t},
					},
				)
			default:
				return nil, errors.New("LT values should be of int64 type")
			}
		case *traversal.GTMetadataMatcher:
			switch t := v.Value().(type) {
			case int64:
				andFilter.Filters = append(andFilter.Filters,
					&flow.Filter{
						GtInt64Filter: &flow.GtInt64Filter{Key: k, Value: t},
					},
				)
			default:
				return nil, errors.New("GT values should be of int64 type")
			}
		case *traversal.GTEMetadataMatcher:
			switch t := v.Value().(type) {
			case int64:
				andFilter.Filters = append(andFilter.Filters,
					&flow.Filter{
						GteInt64Filter: &flow.GteInt64Filter{Key: k, Value: t},
					},
				)
			default:
				return nil, errors.New("GTE values should be of int64 type")
			}
		case *traversal.LTEMetadataMatcher:
			switch t := v.Value().(type) {
			case int64:
				andFilter.Filters = append(andFilter.Filters,
					&flow.Filter{
						LteInt64Filter: &flow.LteInt64Filter{Key: k, Value: t},
					},
				)
			default:
				return nil, errors.New("LTE values should be of int64 type")
			}
		case *traversal.InsideMetadataMatcher:
			from, to := v.Value()

			f64, fok := from.(int64)
			t64, tok := to.(int64)

			if !fok || !tok {
				return nil, errors.New("Inside values should be of int64 type")
			}

			andFilter.Filters = append(andFilter.Filters,
				&flow.Filter{
					BoolFilter: &flow.BoolFilter{
						Op: flow.BoolFilterOp_AND,
						Filters: []*flow.Filter{
							&flow.Filter{
								GtInt64Filter: &flow.GtInt64Filter{Key: k, Value: f64},
							},
							&flow.Filter{
								LtInt64Filter: &flow.LtInt64Filter{Key: k, Value: t64},
							},
						},
					},
				},
			)
		case *traversal.OutsideMetadataMatcher:
			from, to := v.Value()

			f64, fok := from.(int64)
			t64, tok := to.(int64)

			if !fok || !tok {
				return nil, errors.New("Outside values should be of int64 type")
			}

			andFilter.Filters = append(andFilter.Filters,
				&flow.Filter{
					BoolFilter: &flow.BoolFilter{
						Op: flow.BoolFilterOp_AND,
						Filters: []*flow.Filter{
							&flow.Filter{
								LtInt64Filter: &flow.LtInt64Filter{Key: k, Value: f64},
							},
							&flow.Filter{
								GtInt64Filter: &flow.GtInt64Filter{Key: k, Value: t64},
							},
						},
					},
				},
			)
		case *traversal.BetweenMetadataMatcher:
			from, to := v.Value()

			f64, fok := from.(int64)
			t64, tok := to.(int64)

			if !fok || !tok {
				return nil, errors.New("Between values should be of int64 type")
			}

			andFilter.Filters = append(andFilter.Filters,
				&flow.Filter{
					BoolFilter: &flow.BoolFilter{
						Op: flow.BoolFilterOp_AND,
						Filters: []*flow.Filter{
							&flow.Filter{
								GteInt64Filter: &flow.GteInt64Filter{Key: k, Value: f64},
							},
							&flow.Filter{
								LtInt64Filter: &flow.LtInt64Filter{Key: k, Value: t64},
							},
						},
					},
				},
			)
		case string:
			andFilter.Filters = append(andFilter.Filters,
				&flow.Filter{
					TermStringFilter: &flow.TermStringFilter{Key: k, Value: v},
				},
			)
		case int64:
			andFilter.Filters = append(andFilter.Filters,
				&flow.Filter{
					TermInt64Filter: &flow.TermInt64Filter{Key: k, Value: v},
				},
			)
		default:
			return nil, fmt.Errorf("value type unknown: %v", v)
		}
	}

	return &flow.Filter{BoolFilter: andFilter}, nil
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
	f.flowset.Dedup()
	return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, flowset: f.flowset}
}

func (f *FlowTraversalStep) Sort() *FlowTraversalStep {
	f.flowset.Sort()
	return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, flowset: f.flowset}
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
		if flow.ANodeUUID != "" && flow.ANodeUUID != "*" {
			node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.ANodeUUID))
			if node != nil {
				x["ANode"] = node
			}
		}

		if flow.BNodeUUID != "" && flow.BNodeUUID != "*" {
			node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.BNodeUUID))
			if node != nil {
				x["BNode"] = node
			}
		}

		node := f.GraphTraversal.Graph.GetNode(graph.Identifier(flow.NodeUUID))
		if node != nil {
			x["Node"] = node
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
		HopsToken:      HOPS_TOKEN,
		NodesToken:     NODES_TOKEN,
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
	case "HOPS":
		return e.HopsToken, true
	case "NODES":
		return e.NodesToken, true
	}
	return traversal.IDENT, false
}

func (e *FlowTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.FlowToken:
		return &FlowGremlinTraversalStep{
			TableClient: e.TableClient,
			Storage:     e.Storage,
			context:     p,
		}, nil
	case e.BandwidthToken:
		return &BandwidthGremlinTraversalStep{TableClient: e.TableClient, Storage: e.Storage}, nil
	case e.HopsToken:
		return &HopsGremlinTraversalStep{TableClient: e.TableClient, Storage: e.Storage}, nil
	case e.NodesToken:
		return &NodesGremlinTraversalStep{TableClient: e.TableClient, Storage: e.Storage}, nil
	}

	return nil, nil
}

func (s *FlowGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	var graphTraversal *traversal.GraphTraversal
	var err error
	var paramsFilter *flow.Filter

	if len(s.context.Params) > 0 {
		if paramsFilter, err = paramsToFilter(s.context.Params...); err != nil {
			return nil, err
		}
	}

	var interval *flow.Range
	if s.context.StepContext.Range != nil {
		interval = &flow.Range{From: 0, To: s.context.StepContext.Range[1]}
	}

	flowset := flow.NewFlowSet()
	switch tv := last.(type) {
	case *traversal.GraphTraversal:
		graphTraversal = tv

		if context := graphTraversal.Graph.GetContext(); context.Time != nil && s.Storage != nil {
			var flows []*flow.Flow
			if flows, err = storage.LookupFlows(s.Storage, context, paramsFilter, interval); err == nil {
				flowset.Flows = append(flowset.Flows, flows...)
			}
		} else {
			flowSearchQuery := &flow.FlowSearchQuery{
				Filter: paramsFilter,
				Range:  interval,
				Sort:   s.context.StepContext.Sort,
				Dedup:  s.context.StepContext.Dedup,
			}
			flowset, err = s.TableClient.LookupFlows(flowSearchQuery)
		}

		if r := s.context.StepContext.Range; r != nil {
			flowset.Slice(int(r[0]), int(r[1]))
		}
	case *traversal.GraphTraversalV:
		graphTraversal = tv.GraphTraversal

		hnmap := make(flow.HostNodeIDMap)
		graphTraversal.Graph.RLock()
		for _, v := range tv.Values() {
			node := v.(*graph.Node)
			if t, ok := node.Metadata()["Type"]; !ok || !common.IsCaptureAllowed(t.(string)) {
				continue
			}
			hnmap[node.Host()] = append(hnmap[node.Host()], string(node.ID))
		}
		graphTraversal.Graph.RUnlock()

		if context := graphTraversal.Graph.GetContext(); context.Time != nil && s.Storage != nil {
			var flows []*flow.Flow
			if flows, err = storage.LookupFlowsByNodes(s.Storage, context, hnmap, paramsFilter, interval); err == nil {
				flowset.Flows = append(flowset.Flows, flows...)
			}
		} else {
			flowSearchQuery := &flow.FlowSearchQuery{
				Filter: paramsFilter,
				Range:  interval,
				Sort:   s.context.StepContext.Sort,
				Dedup:  s.context.StepContext.Dedup,
			}
			flowset, err = s.TableClient.LookupFlowsByNodes(hnmap, flowSearchQuery)
		}

		if r := s.context.StepContext.Range; r != nil {
			flowset.Slice(int(r[0]), int(r[1]))
		}
	default:
		return nil, traversal.ExecutionError
	}

	if err != nil {
		logging.GetLogger().Errorf("Error while looking for flows: %s", err.Error())
		return nil, err
	}

	return &FlowTraversalStep{GraphTraversal: graphTraversal, flowset: flowset}, nil
}

func (s *FlowGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	if hasStep, ok := next.(*traversal.GremlinTraversalStepHas); ok && len(s.context.Params) == 0 {
		s.context.Params = hasStep.Params
		return s
	}

	if _, ok := next.(*traversal.GremlinTraversalStepSort); ok {
		s.context.StepContext.Sort = true
		return s
	}

	if _, ok := next.(*traversal.GremlinTraversalStepDedup); ok {
		s.context.StepContext.Dedup = true
		return s
	}

	if s.context.ReduceRange(next) {
		return s
	}

	return next
}

func (s *FlowGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
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

func (s *BandwidthGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

func (s *HopsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	var nodes []*graph.Node

	switch last.(type) {
	case *FlowTraversalStep:
		fs := last.(*FlowTraversalStep)
		graphTraversal := fs.GraphTraversal

		m, err := traversal.SliceToMetadata(s.context.Params...)
		if err != nil {
			return nil, traversal.ExecutionError
		}

		for _, f := range fs.flowset.Flows {
			if node := graphTraversal.Graph.GetNode(graph.Identifier(f.NodeUUID)); node != nil && node.MatchMetadata(m) {
				nodes = append(nodes, node)
			}
		}

		graphTraversalV := traversal.NewGraphTraversalV(graphTraversal, nodes)
		return graphTraversalV, nil
	}

	return nil, traversal.ExecutionError
}

func (s *HopsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

func (s *HopsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

func (s *NodesGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fs := last.(*FlowTraversalStep)
		return fs.Nodes(s.context.Params...), nil
	}
	return nil, traversal.ExecutionError
}

func (s *NodesGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

func (s *NodesGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}
