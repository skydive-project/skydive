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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/storage"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

const (
	FLOW_TOKEN         traversal.Token = 1001
	HOPS_TOKEN         traversal.Token = 1002
	NODES_TOKEN        traversal.Token = 1003
	CAPTURE_NODE_TOKEN traversal.Token = 1004
	AGGREGATES_TOKEN   traversal.Token = 1005
)

type FlowTraversalExtension struct {
	FlowToken        traversal.Token
	BandwidthToken   traversal.Token
	HopsToken        traversal.Token
	NodesToken       traversal.Token
	CaptureNodeToken traversal.Token
	AggregatesToken  traversal.Token
	TableClient      *flow.TableClient
	Storage          storage.Storage
}

type FlowGremlinTraversalStep struct {
	TableClient     *flow.TableClient
	Storage         storage.Storage
	context         traversal.GremlinTraversalContext
	hasParams       []interface{}
	metricsNextStep bool
	dedup           bool
	dedupBy         string
	sort            bool
	sortBy          string
}

type FlowTraversalStep struct {
	GraphTraversal  *traversal.GraphTraversal
	Storage         storage.Storage
	flowset         *flow.FlowSet
	flowSearchQuery filters.SearchQuery
	error           error
}

type HopsGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

type NodesGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

type CaptureNodeGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

type AggregatesGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

func (f *FlowTraversalStep) Out(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, flow := range f.flowset.Flows {
		if flow.BNodeTID != "" && flow.BNodeTID != "*" {
			m["TID"] = flow.BNodeTID
			if node := f.GraphTraversal.Graph.LookupFirstNode(m); node != nil {
				nodes = append(nodes, node)
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) In(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, flow := range f.flowset.Flows {
		if flow.ANodeTID != "" && flow.ANodeTID != "*" {
			m["TID"] = flow.ANodeTID
			if node := f.GraphTraversal.Graph.LookupFirstNode(m); node != nil {
				nodes = append(nodes, node)
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) Both(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, flow := range f.flowset.Flows {
		if flow.ANodeTID != "" && flow.ANodeTID != "*" {
			m["TID"] = flow.ANodeTID
			if node := f.GraphTraversal.Graph.LookupFirstNode(m); node != nil {
				nodes = append(nodes, node)
			}
		}
		if flow.BNodeTID != "" && flow.BNodeTID != "*" {
			m["TID"] = flow.BNodeTID
			if node := f.GraphTraversal.Graph.LookupFirstNode(m); node != nil {
				nodes = append(nodes, node)
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) Nodes(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, flow := range f.flowset.Flows {
		if flow.NodeTID != "" && flow.NodeTID != "*" {
			m["TID"] = flow.NodeTID
			if node := f.GraphTraversal.Graph.LookupFirstNode(m); node != nil {
				nodes = append(nodes, node)
			}
		}
		if flow.ANodeTID != "" && flow.ANodeTID != "*" {
			m["TID"] = flow.ANodeTID
			if node := f.GraphTraversal.Graph.LookupFirstNode(m); node != nil {
				nodes = append(nodes, node)
			}
		}
		if flow.BNodeTID != "" && flow.BNodeTID != "*" {
			m["TID"] = flow.BNodeTID
			if node := f.GraphTraversal.Graph.LookupFirstNode(m); node != nil {
				nodes = append(nodes, node)
			}
		}
	}
	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) Hops(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, fl := range f.flowset.Flows {
		m["TID"] = fl.NodeTID
		if node := f.GraphTraversal.Graph.LookupFirstNode(m); node != nil {
			nodes = append(nodes, node)
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) Count(s ...interface{}) *traversal.GraphTraversalValue {
	if f.error != nil {
		return traversal.NewGraphTraversalValue(f.GraphTraversal, 0, f.error)
	}

	return traversal.NewGraphTraversalValue(f.GraphTraversal, len(f.flowset.Flows))
}

func paramsToFilter(params ...interface{}) (*filters.Filter, error) {
	if len(params) < 2 {
		return nil, errors.New("At least two parameters must be provided")
	}

	if len(params)%2 != 0 {
		return nil, fmt.Errorf("slice must be defined by pair k,v: %v", params)
	}

	var andFilters []*filters.Filter
	for i := 0; i < len(params); i += 2 {
		var filter *filters.Filter

		k, ok := params[i].(string)
		if !ok {
			return nil, errors.New("keys should be of string type")
		}

		if v, ok := params[i+1].(string); ok && (k == "Network" || k == "Link" || k == "Transport") {
			filter = filters.NewOrFilter(filters.NewTermStringFilter(k+".A", v), filters.NewTermStringFilter(k+".B", v))
		} else {
			f, err := traversal.ParamToFilter(k, params[i+1])
			if err != nil {
				return nil, err
			}
			filter = f
		}

		andFilters = append(andFilters, filter)
	}

	return filters.NewAndFilter(andFilters...), nil
}

func (f *FlowTraversalStep) Has(s ...interface{}) *FlowTraversalStep {
	if f.error != nil {
		return f
	}

	filter, err := paramsToFilter(s...)
	if err != nil {
		return &FlowTraversalStep{error: err}
	}

	return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, Storage: f.Storage, flowset: f.flowset.Filter(filter)}
}

func (f *FlowTraversalStep) Dedup(keys ...interface{}) *FlowTraversalStep {
	if f.error != nil {
		return f
	}

	var key string
	if len(keys) > 0 {
		k, ok := keys[0].(string)
		if !ok {
			return &FlowTraversalStep{error: fmt.Errorf("Dedup parameter has to be a string key")}
		}
		key = k
	}

	if err := f.flowset.Dedup(key); err != nil {
		return &FlowTraversalStep{error: err}
	}

	return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, Storage: f.Storage, flowset: f.flowset}
}

func (f *FlowTraversalStep) CaptureNode(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.SliceToMetadata(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	for _, fl := range f.flowset.Flows {
		m["TID"] = fl.NodeTID
		if node := f.GraphTraversal.Graph.LookupFirstNode(m); node != nil {
			nodes = append(nodes, node)
		}
	}
	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

func (f *FlowTraversalStep) Sort(keys ...interface{}) *FlowTraversalStep {
	if f.error != nil {
		return f
	}
	sortBy := "Last"
	switch len(keys) {
	case 0:
	case 1:
		key, ok := keys[0].(string)
		if !ok {
			return &FlowTraversalStep{error: fmt.Errorf("Sort parameter has to be a string key")}
		} else {
			sortBy = key
		}
	default:
		return &FlowTraversalStep{error: fmt.Errorf("Sort accept utmost 1 parameter")}
	}

	f.flowset.Sort(sortBy)
	return &FlowTraversalStep{GraphTraversal: f.GraphTraversal, Storage: f.Storage, flowset: f.flowset}
}

// Sum aggregates integer values mapped by 'key' cross flows
func (f *FlowTraversalStep) Sum(keys ...interface{}) *traversal.GraphTraversalValue {
	if f.error != nil {
		return traversal.NewGraphTraversalValue(f.GraphTraversal, nil, f.error)
	}

	if len(keys) != 1 {
		return traversal.NewGraphTraversalValue(f.GraphTraversal, nil, fmt.Errorf("Sum requires 1 parameter"))
	}

	key, ok := keys[0].(string)
	if !ok {
		return traversal.NewGraphTraversalValue(f.GraphTraversal, nil, fmt.Errorf("Sum parameter has to be a string key"))
	}

	var s float64
	for _, fl := range f.flowset.Flows {
		if v, err := fl.GetFieldInt64(key); err == nil {
			s += float64(v)
		} else {
			return traversal.NewGraphTraversalValue(f.GraphTraversal, s, err)
		}
	}
	return traversal.NewGraphTraversalValue(f.GraphTraversal, s, nil)
}

func (f *FlowTraversalStep) propertyInt64Values(field string) *traversal.GraphTraversalValue {
	var s []interface{}
	for _, fl := range f.flowset.Flows {
		// ignore errors as not all flows have the all layers(Link/Network) thus not all fields
		if v, err := fl.GetFieldInt64(field); err == nil {
			s = append(s, v)
		}
	}

	return traversal.NewGraphTraversalValue(f.GraphTraversal, s, nil)
}

func (f *FlowTraversalStep) propertyStringValues(field string) *traversal.GraphTraversalValue {
	var s []interface{}
	for _, fl := range f.flowset.Flows {
		// ignore errors as not all flows have the all layers(Link/Network) thus not all fields
		if v, err := fl.GetFieldString(field); err == nil {
			s = append(s, v)
		}
	}

	return traversal.NewGraphTraversalValue(f.GraphTraversal, s, nil)
}

func (f *FlowTraversalStep) PropertyValues(keys ...interface{}) *traversal.GraphTraversalValue {
	if f.error != nil {
		return traversal.NewGraphTraversalValue(f.GraphTraversal, nil, f.error)
	}

	key := keys[0].(string)

	if len(f.flowset.Flows) > 0 {
		// use the first flow to determine what is the type of the field
		if _, err := f.flowset.Flows[0].GetFieldInt64(key); err == nil {
			return f.propertyInt64Values(key)
		}
		return f.propertyStringValues(key)
	}

	return traversal.NewGraphTraversalValue(f.GraphTraversal, nil, common.ErrFieldNotFound)
}

func (f *FlowTraversalStep) PropertyKeys(keys ...interface{}) *traversal.GraphTraversalValue {
	if f.error != nil {
		return traversal.NewGraphTraversalValue(f.GraphTraversal, nil, f.error)
	}

	var s []interface{}

	if len(f.flowset.Flows) > 0 {
		// all Flow structs are the same, take the first one
		s = f.flowset.Flows[0].GetFields()
	}

	return traversal.NewGraphTraversalValue(f.GraphTraversal, s, nil)
}

func (f *FlowTraversalStep) Metrics() *traversal.MetricsTraversalStep {
	if f.error != nil {
		return traversal.NewMetricsTraversalStep(nil, nil, f.error)
	}

	var flowMetrics map[string][]*common.TimedMetric

	context := f.GraphTraversal.Graph.GetContext()
	if context.TimeSlice != nil {
		flowMetrics = make(map[string][]*common.TimedMetric)

		// two cases, either we have a flowset and we need to use it in order to filter
		// flows or we don't have flowset but we have the pre-built flowSearchQuery filter
		// if none of these cases it's an error.
		if f.flowset != nil {
			flowFilter := flow.NewFilterForFlowSet(f.flowset)
			f.flowSearchQuery.Filter = filters.NewAndFilter(f.flowSearchQuery.Filter, flowFilter)
		} else if f.flowSearchQuery.Filter == nil {
			return traversal.NewMetricsTraversalStep(nil, nil, errors.New("Unable to filter flows"))
		}

		fr := filters.Range{To: context.TimeSlice.Last}
		if context.TimeSlice.Start != context.TimeSlice.Last {
			fr.From = context.TimeSlice.Start
		}
		metricFilter := filters.NewFilterIncludedIn(fr, "")

		f.flowSearchQuery.Sort = true
		f.flowSearchQuery.SortBy = "Last"

		var err error
		if flowMetrics, err = f.Storage.SearchMetrics(f.flowSearchQuery, metricFilter); err != nil {
			return traversal.NewMetricsTraversalStep(nil, nil, f.error)
		}
	} else {
		flowMetrics = make(map[string][]*common.TimedMetric, len(f.flowset.Flows))
		for _, flow := range f.flowset.Flows {
			var timedMetric *common.TimedMetric
			if flow.LastUpdateStart != 0 || flow.LastUpdateLast != 0 {
				timedMetric = &common.TimedMetric{
					TimeSlice: *common.NewTimeSlice(flow.LastUpdateStart, flow.LastUpdateLast),
					Metric:    flow.LastUpdateMetric,
				}
			} else {
				// if we get empty LastUpdateMetric it means that we got flow not already updated
				// by the flow table update ticker, so packets between the start of the flow and
				// the first update.
				timedMetric = &common.TimedMetric{
					TimeSlice: *common.NewTimeSlice(flow.Start, flow.Last),
					Metric:    flow.Metric,
				}
			}
			flowMetrics[flow.UUID] = append(flowMetrics[flow.UUID], timedMetric)
		}
	}

	return traversal.NewMetricsTraversalStep(f.GraphTraversal, flowMetrics, nil)
}

func (f *FlowTraversalStep) Values() []interface{} {
	a := make([]interface{}, len(f.flowset.Flows))
	for i, flow := range f.flowset.Flows {
		a[i] = flow
	}
	return a
}

func (f *FlowTraversalStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Values())
}

func (f *FlowTraversalStep) Error() error {
	return f.error
}

func NewFlowTraversalExtension(client *flow.TableClient, storage storage.Storage) *FlowTraversalExtension {
	return &FlowTraversalExtension{
		FlowToken:        FLOW_TOKEN,
		HopsToken:        HOPS_TOKEN,
		NodesToken:       NODES_TOKEN,
		CaptureNodeToken: CAPTURE_NODE_TOKEN,
		AggregatesToken:  AGGREGATES_TOKEN,
		TableClient:      client,
		Storage:          storage,
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
	case "CAPTURENODE":
		return e.CaptureNodeToken, true
	case "AGGREGATES":
		return e.AggregatesToken, true
	}
	return traversal.IDENT, false
}

func (e *FlowTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.FlowToken:
		return &FlowGremlinTraversalStep{TableClient: e.TableClient, Storage: e.Storage, context: p}, nil
	case e.HopsToken:
		return &HopsGremlinTraversalStep{context: p}, nil
	case e.NodesToken:
		return &NodesGremlinTraversalStep{context: p}, nil
	case e.CaptureNodeToken:
		return &CaptureNodeGremlinTraversalStep{context: p}, nil
	case e.AggregatesToken:
		return &AggregatesGremlinTraversalStep{context: p}, nil
	}

	return nil, nil
}

func (s *FlowGremlinTraversalStep) makeSearchQuery() (fsq filters.SearchQuery, err error) {
	var paramsFilter *filters.Filter

	if len(s.hasParams) > 0 {
		if paramsFilter, err = paramsToFilter(s.hasParams...); err != nil {
			return fsq, err
		}
	}

	var interval *filters.Range
	if s.context.StepContext.PaginationRange != nil {
		// not using the From parameter as the pagination will be applied after
		// flow request.
		interval = &filters.Range{From: 0, To: s.context.StepContext.PaginationRange[1]}
	}

	fsq = filters.SearchQuery{
		Filter:          paramsFilter,
		PaginationRange: interval,
		Dedup:           s.dedup,
		DedupBy:         s.dedupBy,
		Sort:            s.sort,
		SortBy:          s.sortBy,
	}

	return
}

func captureAllowedNodes(nodes []*graph.Node) []*graph.Node {
	var allowed []*graph.Node
	for _, n := range nodes {
		if tp, _ := n.GetFieldString("Type"); tp != "" && common.IsCaptureAllowed(tp) {
			allowed = append(allowed, n)
		}
	}
	return allowed
}

func (s *FlowGremlinTraversalStep) addTimeFilter(fsq *filters.SearchQuery, timeContext *common.TimeSlice) {
	var timeFilter *filters.Filter
	tr := filters.Range{
		// When we query the flows on the agents, we get the flows that have not
		// expired yet. To replicate this behaviour for stored storage, we need
		// to add the expire duration:
		// [-----------------------------------]         |
		// Start                            Last       Query
		//                      ^                        ^
		//                     From                      To
		From: timeContext.Start - config.GetConfig().GetInt64("analyzer.flowtable_expire")*1000,
		To:   timeContext.Last,
	}
	// flow need to have at least one metric included in the time range
	timeFilter = filters.NewFilterActiveIn(tr, "")
	fsq.Filter = filters.NewAndFilter(fsq.Filter, timeFilter)
}

func (s *FlowGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	var graphTraversal *traversal.GraphTraversal
	var err error

	flowSearchQuery, err := s.makeSearchQuery()
	if err != nil {
		return nil, err
	}

	flowset := &flow.FlowSet{}

	switch tv := last.(type) {
	case *traversal.GraphTraversal:
		graphTraversal = tv
		context := graphTraversal.Graph.GetContext()

		if context.TimeSlice != nil {
			if s.Storage == nil {
				return nil, storage.NoStorageConfigured
			}

			s.addTimeFilter(&flowSearchQuery, context.TimeSlice)

			// We do nothing as the following step is Metrics
			// and we'll make a request on metrics instead of flows
			if s.metricsNextStep {
				return &FlowTraversalStep{GraphTraversal: graphTraversal, Storage: s.Storage, flowSearchQuery: flowSearchQuery}, nil
			}

			if flowset, err = s.Storage.SearchFlows(flowSearchQuery); err != nil {
				return nil, err
			}
		} else {
			flowset, err = s.TableClient.LookupFlows(flowSearchQuery)
		}
	case *traversal.GraphTraversalV:
		graphTraversal = tv.GraphTraversal
		context := graphTraversal.Graph.GetContext()

		// not need to get flows from node not supporting capture
		nodes := captureAllowedNodes(tv.GetNodes())
		if len(nodes) != 0 {
			if context.TimeSlice != nil {
				if s.Storage == nil {
					return nil, storage.NoStorageConfigured
				}

				s.addTimeFilter(&flowSearchQuery, context.TimeSlice)

				// previously selected nodes then need to filter flow belonging to them
				nodeFilter := flow.NewFilterForNodes(nodes)
				flowSearchQuery.Filter = filters.NewAndFilter(flowSearchQuery.Filter, nodeFilter)

				// We do nothing as the following step is Metrics
				// and we'll make a request on metrics instead of flows
				if s.metricsNextStep {
					return &FlowTraversalStep{GraphTraversal: graphTraversal, Storage: s.Storage, flowSearchQuery: flowSearchQuery}, nil
				}

				if flowset, err = s.Storage.SearchFlows(flowSearchQuery); err != nil {
					return nil, err
				}
			} else {
				hnmap := topology.BuildHostNodeTIDMap(nodes)
				flowset, err = s.TableClient.LookupFlowsByNodes(hnmap, flowSearchQuery)
			}
		}
	default:
		return nil, traversal.ExecutionError
	}

	if err != nil {
		logging.GetLogger().Errorf("Error while looking for flows: %s", err.Error())
		return nil, err
	}

	if r := s.context.StepContext.PaginationRange; r != nil {
		flowset.Slice(int(r[0]), int(r[1]))
	}

	return &FlowTraversalStep{GraphTraversal: graphTraversal, Storage: s.Storage, flowset: flowset, flowSearchQuery: flowSearchQuery}, nil
}

func (s *FlowGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	if hasStep, ok := next.(*traversal.GremlinTraversalStepHas); ok {
		// merge has parameters, useful in case of multiple Has reduce
		s.hasParams = append(s.hasParams, hasStep.Params...)
		return s
	}

	if dedupStep, ok := next.(*traversal.GremlinTraversalStepDedup); ok {
		s.dedup = true
		if len(dedupStep.Params) > 0 {
			s.dedupBy = dedupStep.Params[0].(string)
		}
	}

	if sortStep, ok := next.(*traversal.GremlinTraversalStepSort); ok {
		s.sort = true
		s.sortBy = "Last"
		if len(sortStep.Params) > 0 {
			s.sortBy = sortStep.Params[0].(string)
		}
		return s
	}

	if _, ok := next.(*traversal.GremlinTraversalStepMetrics); ok {
		s.metricsNextStep = true
	}

	if s.context.ReduceRange(next) {
		return s
	}

	return next
}

func (s *FlowGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

func (s *HopsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fts := last.(*FlowTraversalStep)
		return fts.Hops(s.context.Params...), nil
	}

	return nil, traversal.ExecutionError
}

func (s *HopsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	if hasStep, ok := next.(*traversal.GremlinTraversalStepHas); ok {
		s.context.Params = hasStep.Params
		return s
	}
	return next
}

func (s *HopsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

func (s *NodesGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fts := last.(*FlowTraversalStep)
		return fts.Nodes(s.context.Params...), nil
	}
	return nil, traversal.ExecutionError
}

func (s *NodesGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	if hasStep, ok := next.(*traversal.GremlinTraversalStepHas); ok {
		s.context.Params = hasStep.Params
		return s
	}
	return next
}

func (s *NodesGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

func (s *CaptureNodeGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fs := last.(*FlowTraversalStep)
		return fs.CaptureNode(s.context.Params...), nil
	}

	return nil, traversal.ExecutionError
}

func (s *CaptureNodeGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	if hasStep, ok := next.(*traversal.GremlinTraversalStepHas); ok {
		s.context.Params = hasStep.Params
		return s
	}
	return next
}

func (s *CaptureNodeGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

func (a *AggregatesGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *traversal.MetricsTraversalStep:
		mts := last.(*traversal.MetricsTraversalStep)
		return mts.Aggregates(), nil
	}

	return nil, traversal.ExecutionError
}

func (a *AggregatesGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

func (a *AggregatesGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &a.context
}
