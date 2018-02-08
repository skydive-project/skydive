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
	"strings"

	"github.com/google/gopacket/layers"
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
	defaultSortBy = "Last"
)

// FlowTraversalExtension describes flows in a graph Gremlin language extension
type FlowTraversalExtension struct {
	FlowToken        traversal.Token
	HopsToken        traversal.Token
	NodesToken       traversal.Token
	CaptureNodeToken traversal.Token
	AggregatesToken  traversal.Token
	RawPacketsToken  traversal.Token
	BpfToken         traversal.Token
	TableClient      *flow.TableClient
	Storage          storage.Storage
}

// FlowGremlinTraversalStep a flow Gremlin language step
type FlowGremlinTraversalStep struct {
	TableClient        *flow.TableClient
	Storage            storage.Storage
	context            traversal.GremlinTraversalContext
	hasParams          []interface{}
	metricsNextStep    bool
	rawpacketsNextStep bool
	dedup              bool
	dedupBy            string
	sort               bool
	sortBy             string
	sortOrder          common.SortOrder
}

// FlowTraversalStep a flow step linked to a storage
type FlowTraversalStep struct {
	GraphTraversal  *traversal.GraphTraversal
	Storage         storage.Storage
	flowset         *flow.FlowSet
	flowSearchQuery filters.SearchQuery
	error           error
}

// RawPacketsTraversalStep rawpackets step
type RawPacketsTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	rawPackets     map[string]*flow.RawPackets
	error          error
}

// HopsGremlinTraversalStep hops step
type HopsGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

// NodesGremlinTraversalStep nodes step
type NodesGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

// CaptureNodeGremlinTraversalStep capture step
type CaptureNodeGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

// AggregatesGremlinTraversalStep aggregates step
type AggregatesGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

// RawPacketsGremlinTraversalStep rawpackets step
type RawPacketsGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

// BpfGremlinTraversalStep step
type BpfGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

// Out returns the B node
func (f *FlowTraversalStep) Out(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.ParamsToMap(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	f.GraphTraversal.RLock()
	defer f.GraphTraversal.RUnlock()

	for _, flow := range f.flowset.Flows {
		if flow.BNodeTID != "" && flow.BNodeTID != "*" {
			m["TID"] = flow.BNodeTID

			matcher, err := traversal.MapToMetadataFilter(m)
			if err != nil {
				return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
			}

			if node := f.GraphTraversal.Graph.LookupFirstNode(matcher); node != nil {
				nodes = append(nodes, node)
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

// In returns the A node
func (f *FlowTraversalStep) In(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.ParamsToMap(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	f.GraphTraversal.RLock()
	defer f.GraphTraversal.RUnlock()

	for _, flow := range f.flowset.Flows {
		if flow.ANodeTID != "" && flow.ANodeTID != "*" {
			m["TID"] = flow.ANodeTID

			matcher, err := traversal.MapToMetadataFilter(m)
			if err != nil {
				return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
			}

			if node := f.GraphTraversal.Graph.LookupFirstNode(matcher); node != nil {
				nodes = append(nodes, node)
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

// Both returns A and B nodes
func (f *FlowTraversalStep) Both(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.ParamsToMap(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	f.GraphTraversal.RLock()
	defer f.GraphTraversal.RUnlock()

	for _, flow := range f.flowset.Flows {
		if flow.ANodeTID != "" && flow.ANodeTID != "*" {
			m["TID"] = flow.ANodeTID

			matcher, err := traversal.MapToMetadataFilter(m)
			if err != nil {
				return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
			}

			if node := f.GraphTraversal.Graph.LookupFirstNode(matcher); node != nil {
				nodes = append(nodes, node)
			}
		}
		if flow.BNodeTID != "" && flow.BNodeTID != "*" {
			m["TID"] = flow.BNodeTID

			matcher, err := traversal.MapToMetadataFilter(m)
			if err != nil {
				return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
			}

			if node := f.GraphTraversal.Graph.LookupFirstNode(matcher); node != nil {
				nodes = append(nodes, node)
			}
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

// Nodes returns A, B and the capture nodes
func (f *FlowTraversalStep) Nodes(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.ParamsToMap()
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	f.GraphTraversal.RLock()
	defer f.GraphTraversal.RUnlock()

	for _, flow := range f.flowset.Flows {
		if flow.NodeTID != "" && flow.NodeTID != "*" {
			m["TID"] = flow.NodeTID

			matcher, err := traversal.MapToMetadataFilter(m)
			if err != nil {
				return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
			}

			if node := f.GraphTraversal.Graph.LookupFirstNode(matcher); node != nil {
				nodes = append(nodes, node)
			}
		}
		if flow.ANodeTID != "" && flow.ANodeTID != "*" {
			m["TID"] = flow.ANodeTID

			matcher, err := traversal.MapToMetadataFilter(m)
			if err != nil {
				return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
			}

			if node := f.GraphTraversal.Graph.LookupFirstNode(matcher); node != nil {
				nodes = append(nodes, node)
			}
		}
		if flow.BNodeTID != "" && flow.BNodeTID != "*" {
			m["TID"] = flow.BNodeTID

			matcher, err := traversal.MapToMetadataFilter(m)
			if err != nil {
				return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
			}

			if node := f.GraphTraversal.Graph.LookupFirstNode(matcher); node != nil {
				nodes = append(nodes, node)
			}
		}
	}
	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

// Hops returns all the capture nodes where the flow was seen
func (f *FlowTraversalStep) Hops(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.ParamsToMap(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	f.GraphTraversal.RLock()
	defer f.GraphTraversal.RUnlock()

	for _, fl := range f.flowset.Flows {
		m["TID"] = fl.NodeTID

		matcher, err := traversal.MapToMetadataFilter(m)
		if err != nil {
			return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
		}

		if node := f.GraphTraversal.Graph.LookupFirstNode(matcher); node != nil {
			nodes = append(nodes, node)
		}
	}

	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

// Count step
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

		if k == "Network" || k == "Link" || k == "Transport" {
			fa, err := traversal.KeyValueToFilter(k+".A", params[i+1])
			if err != nil {
				return nil, err
			}

			fb, err := traversal.KeyValueToFilter(k+".B", params[i+1])
			if err != nil {
				return nil, err
			}

			filter = filters.NewOrFilter(fa, fb)
		} else {
			f, err := traversal.KeyValueToFilter(k, params[i+1])
			if err != nil {
				return nil, err
			}
			filter = f
		}

		andFilters = append(andFilters, filter)
	}

	return filters.NewAndFilter(andFilters...), nil
}

// Has step
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

// Dedup deduplicate step
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

// CaptureNode step
func (f *FlowTraversalStep) CaptureNode(s ...interface{}) *traversal.GraphTraversalV {
	var nodes []*graph.Node

	if f.error != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, f.error)
	}

	m, err := traversal.ParamsToMap(s...)
	if err != nil {
		return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
	}

	f.GraphTraversal.RLock()
	defer f.GraphTraversal.RUnlock()

	for _, fl := range f.flowset.Flows {
		m["TID"] = fl.NodeTID

		matcher, err := traversal.MapToMetadataFilter(m)
		if err != nil {
			return traversal.NewGraphTraversalV(f.GraphTraversal, nodes, err)
		}

		if node := f.GraphTraversal.Graph.LookupFirstNode(matcher); node != nil {
			nodes = append(nodes, node)
		}
	}
	return traversal.NewGraphTraversalV(f.GraphTraversal, nodes)
}

// Sort step
func (f *FlowTraversalStep) Sort(keys ...interface{}) *FlowTraversalStep {
	if f.error != nil {
		return f
	}

	order, sortBy, err := traversal.ParseSortParameter(keys...)
	if err != nil {
		return &FlowTraversalStep{error: err}
	}

	if sortBy == "" {
		sortBy = defaultSortBy
	}

	f.flowset.Sort(order, sortBy)
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

	k := strings.Split(key, ".")
	if k[0] != "Metric" && k[0] != "LastUpdateMetric" {
		return traversal.NewGraphTraversalValue(f.GraphTraversal, nil, fmt.Errorf("Sum accepts only sub fields of Metric and LastUpadteMetric"))
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

// PropertyValues returns a flow field value
func (f *FlowTraversalStep) PropertyValues(keys ...interface{}) *traversal.GraphTraversalValue {
	if f.error != nil {
		return traversal.NewGraphTraversalValue(f.GraphTraversal, nil, f.error)
	}

	key := keys[0].(string)
	var s []interface{}
	for _, fl := range f.flowset.Flows {
		v, err := fl.GetField(key)
		if err != nil {
			return traversal.NewGraphTraversalValue(f.GraphTraversal, nil, common.ErrFieldNotFound)
		}
		s = append(s, v)
	}
	return traversal.NewGraphTraversalValue(f.GraphTraversal, s, nil)

}

// PropertyKeys returns a flow field
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

// FlowMetrics returns flow metric counters
func (f *FlowTraversalStep) FlowMetrics() *MetricsTraversalStep {
	if f.error != nil {
		return NewMetricsTraversalStep(nil, nil, f.error)
	}

	var flowMetrics map[string][]common.Metric

	context := f.GraphTraversal.Graph.GetContext()
	if context.TimeSlice != nil {
		// two cases, either we have a flowset and we need to use it in order to filter
		// flows or we don't have flowset but we have the pre-built flowSearchQuery filter
		// if none of these cases it's an error.
		if f.flowset != nil {
			flowFilter := flow.NewFilterForFlowSet(f.flowset)
			f.flowSearchQuery.Filter = filters.NewAndFilter(f.flowSearchQuery.Filter, flowFilter)
		} else if f.flowSearchQuery.Filter == nil {
			return NewMetricsTraversalStep(nil, nil, errors.New("Unable to filter flows"))
		}

		fr := filters.Range{To: context.TimeSlice.Last}
		if context.TimeSlice.Start != context.TimeSlice.Last {
			fr.From = context.TimeSlice.Start
		}
		metricFilter := filters.NewFilterIncludedIn(fr, "")

		f.flowSearchQuery.Sort = true
		f.flowSearchQuery.SortBy = defaultSortBy
		f.flowSearchQuery.SortOrder = string(common.SortAscending)

		var err error
		if flowMetrics, err = f.Storage.SearchMetrics(f.flowSearchQuery, metricFilter); err != nil {
			return NewMetricsTraversalStep(nil, nil, f.error)
		}
	} else {
		flowMetrics = make(map[string][]common.Metric, len(f.flowset.Flows))
		for _, f := range f.flowset.Flows {
			var metric *flow.FlowMetric
			if f.LastUpdateMetric != nil {
				metric = f.LastUpdateMetric
			} else {
				// if we get empty LastUpdateMetric it means that we got flow not already updated
				// by the flow table update ticker, so packets between the start of the flow and
				// the first update.
				metric = f.Metric
			}
			flowMetrics[f.UUID] = append(flowMetrics[f.UUID], metric)
		}
	}

	return NewMetricsTraversalStep(f.GraphTraversal, flowMetrics, nil)
}

// Values returns list of raw packets
func (r *RawPacketsTraversalStep) Values() []interface{} {
	if len(r.rawPackets) == 0 {
		return []interface{}{}
	}
	return []interface{}{r.rawPackets}
}

// MarshalJSON serialize in JSON
func (r *RawPacketsTraversalStep) MarshalJSON() ([]byte, error) {
	values := r.Values()
	r.GraphTraversal.RLock()
	defer r.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

// Error returns tranversal error
func (r *RawPacketsTraversalStep) Error() error {
	return r.error
}

// RawPackets searches for RawPacket based on previous flow filter from
// either agents or datastore.
func (f *FlowTraversalStep) RawPackets() *RawPacketsTraversalStep {
	if f.error != nil {
		return &RawPacketsTraversalStep{error: f.error}
	}

	rawPackets := make(map[string]*flow.RawPackets)

	context := f.GraphTraversal.Graph.GetContext()
	if context.TimeSlice != nil {
		// two cases, either we have a flowset and we need to use it in order to filter
		// flows or we don't have flowset but we have the pre-built flowSearchQuery filter
		// if none of these cases it's an error.
		if f.flowset != nil {
			flowFilter := flow.NewFilterForFlowSet(f.flowset)
			f.flowSearchQuery.Filter = filters.NewAndFilter(f.flowSearchQuery.Filter, flowFilter)
		} else if f.flowSearchQuery.Filter == nil {
			return &RawPacketsTraversalStep{error: errors.New("Unable to filter flows")}
		}

		fr := filters.Range{To: context.TimeSlice.Last}
		if context.TimeSlice.Start != context.TimeSlice.Last {
			fr.From = context.TimeSlice.Start
		}

		rawPacketsFilter := filters.NewAndFilter(
			filters.NewGteInt64Filter("Timestamp", fr.From),
			filters.NewLteInt64Filter("Timestamp", fr.To),
		)

		f.flowSearchQuery.Sort = true
		f.flowSearchQuery.SortBy = "Index"
		f.flowSearchQuery.SortOrder = string(common.SortAscending)

		var err error
		if rawPackets, err = f.Storage.SearchRawPackets(f.flowSearchQuery, rawPacketsFilter); err != nil {
			return &RawPacketsTraversalStep{error: err}
		}
	} else {
		for _, fl := range f.flowset.Flows {
			linkType, err := fl.LinkType()
			if err != nil {
				return &RawPacketsTraversalStep{error: err}
			}

			if len(fl.LastRawPackets) > 0 {
				rawPackets[fl.UUID] = &flow.RawPackets{
					LinkType:   linkType,
					RawPackets: fl.LastRawPackets,
				}
			}
		}
	}

	return &RawPacketsTraversalStep{GraphTraversal: f.GraphTraversal, rawPackets: rawPackets}
}

// BPF returns only the raw packets that matches the specified BPF filter
func (r *RawPacketsTraversalStep) BPF(s ...interface{}) *RawPacketsTraversalStep {
	if r.error != nil {
		return &RawPacketsTraversalStep{error: r.error}
	}

	if len(s) != 1 {
		return &RawPacketsTraversalStep{error: fmt.Errorf("BPF requires 1 parameter")}
	}

	filter, ok := s[0].(string)
	if !ok {
		return &RawPacketsTraversalStep{error: fmt.Errorf("BPF parameter has to be a string")}
	}

	// While very improbable, we may have different link types so we keep
	// a map of BPF filters for the link types
	bpfFilters := make(map[layers.LinkType]*flow.BPF)
	rawPackets := make(map[string]*flow.RawPackets)
	for key, value := range r.rawPackets {
		var err error
		bpf, ok := bpfFilters[value.LinkType]
		if !ok {
			bpf, err = flow.NewBPF(value.LinkType, flow.DefaultCaptureLength, filter)
			if err != nil {
				return &RawPacketsTraversalStep{error: err}
			}
			bpfFilters[value.LinkType] = bpf
		}

		var filteredPackets []*flow.RawPacket
		for _, packet := range value.RawPackets {
			if bpf.Matches(packet.Data) {
				filteredPackets = append(filteredPackets, packet)
			}
		}
		rawPackets[key] = &flow.RawPackets{
			LinkType:   value.LinkType,
			RawPackets: filteredPackets,
		}
	}

	return &RawPacketsTraversalStep{GraphTraversal: r.GraphTraversal, rawPackets: rawPackets}
}

// Values returns list of flows
func (f *FlowTraversalStep) Values() []interface{} {
	a := make([]interface{}, len(f.flowset.Flows))
	for i, flow := range f.flowset.Flows {
		a[i] = flow
	}
	return a
}

// MarshalJSON serialize in JSON
func (f *FlowTraversalStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Values())
}

// Error returns tranversal error
func (f *FlowTraversalStep) Error() error {
	return f.error
}

// NewFlowTraversalExtension creates a new flow tranversal extension for Gremlin parser
func NewFlowTraversalExtension(client *flow.TableClient, storage storage.Storage) *FlowTraversalExtension {
	return &FlowTraversalExtension{
		FlowToken:        traversalFlowToken,
		HopsToken:        traversalHopsToken,
		NodesToken:       traversalNodesToken,
		CaptureNodeToken: traversalCaptureNodeToken,
		AggregatesToken:  traversalAggregatesToken,
		RawPacketsToken:  traversalRawPacketsToken,
		BpfToken:         traversalBpfToken,
		TableClient:      client,
		Storage:          storage,
	}
}

// ScanIdent tokenize the step
func (e *FlowTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "FLOWS":
		return e.FlowToken, true
	case "HOPS":
		return e.HopsToken, true
	case "NODES":
		return e.NodesToken, true
	case "NODE", "CAPTURENODE":
		return e.CaptureNodeToken, true
	case "AGGREGATES":
		return e.AggregatesToken, true
	case "RAWPACKETS":
		return e.RawPacketsToken, true
	case "BPF":
		return e.BpfToken, true
	}
	return traversal.IDENT, false
}

// ParseStep creates steps from token
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
	case e.RawPacketsToken:
		return &RawPacketsGremlinTraversalStep{context: p}, nil
	case e.BpfToken:
		return &BpfGremlinTraversalStep{context: p}, nil
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
		SortOrder:       string(s.sortOrder),
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
		From: timeContext.Start - int64(config.GetInt("flow.expire"))*1000,
		To:   timeContext.Last,
	}
	// flow need to have at least one metric included in the time range
	timeFilter = filters.NewFilterActiveIn(tr, "")
	fsq.Filter = filters.NewAndFilter(fsq.Filter, timeFilter)
}

// Exec flow step
func (s *FlowGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	var graphTraversal *traversal.GraphTraversal
	var err error
	var context graph.GraphContext
	var nodes []*graph.Node

	flowSearchQuery, err := s.makeSearchQuery()
	if err != nil {
		return nil, err
	}

	flowset := &flow.FlowSet{}

	switch tv := last.(type) {
	case *traversal.GraphTraversal:
		graphTraversal = tv
		graphTraversal.RLock()
		context = graphTraversal.Graph.GetContext()
		graphTraversal.RUnlock()
	case *traversal.GraphTraversalV:
		graphTraversal = tv.GraphTraversal

		graphTraversal.RLock()
		context = graphTraversal.Graph.GetContext()
		// not need to get flows from node not supporting capture
		if nodes = captureAllowedNodes(tv.GetNodes()); len(nodes) == 0 {
			graphTraversal.RUnlock()
			return &FlowTraversalStep{GraphTraversal: graphTraversal, Storage: s.Storage, flowset: flowset, flowSearchQuery: flowSearchQuery}, nil
		}
		graphTraversal.RUnlock()
	case *traversal.GraphTraversalShortestPath:
		graphTraversal = tv.GraphTraversal

		graphTraversal.RLock()
		context = graphTraversal.Graph.GetContext()
		// not need to get flows from node not supporting capture
		if nodes = captureAllowedNodes(tv.GetNodes()); len(nodes) == 0 {
			graphTraversal.RUnlock()
			return &FlowTraversalStep{GraphTraversal: graphTraversal, Storage: s.Storage, flowset: flowset, flowSearchQuery: flowSearchQuery}, nil
		}
		graphTraversal.RUnlock()
	default:
		return nil, traversal.ErrExecutionError
	}

	if context.TimeSlice != nil {
		if s.Storage == nil {
			return nil, storage.ErrNoStorageConfigured
		}

		s.addTimeFilter(&flowSearchQuery, context.TimeSlice)

		if len(nodes) != 0 {
			graphTraversal.RLock()
			nodeFilter := flow.NewFilterForNodes(nodes)
			flowSearchQuery.Filter = filters.NewAndFilter(flowSearchQuery.Filter, nodeFilter)
			graphTraversal.RUnlock()
		}

		// We do nothing as the following step is Metrics
		// and we'll make a request on metrics instead of flows
		if s.metricsNextStep {
			return &FlowTraversalStep{GraphTraversal: graphTraversal, Storage: s.Storage, flowSearchQuery: flowSearchQuery}, nil
		}

		// We do nothing as the following step is Metrics
		// and we'll make a request on rawpackets instead of flows
		if s.rawpacketsNextStep {
			return &FlowTraversalStep{GraphTraversal: graphTraversal, Storage: s.Storage, flowSearchQuery: flowSearchQuery}, nil
		}

		if flowset, err = s.Storage.SearchFlows(flowSearchQuery); err != nil {
			return nil, err
		}
	} else {
		if len(nodes) != 0 {
			graphTraversal.RLock()
			hnmap := topology.BuildHostNodeTIDMap(nodes)
			graphTraversal.RUnlock()
			flowset, err = s.TableClient.LookupFlowsByNodes(hnmap, flowSearchQuery)
		} else {
			flowset, err = s.TableClient.LookupFlows(flowSearchQuery)
		}
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

// Reduce flow step
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
		return s
	}

	if sortStep, ok := next.(*traversal.GremlinTraversalStepSort); ok {
		s.sort = true
		s.sortBy = defaultSortBy
		s.sortOrder = common.SortAscending
		if len(sortStep.Params) > 0 {
			var err error
			if s.sortOrder, s.sortBy, err = traversal.ParseSortParameter(sortStep.Params...); err != nil {
				// in case of error no reduce, the error will be triggered by the non reduce version
				return next
			}
		}

		return s
	}

	switch next.(type) {
	case *MetricsGremlinTraversalStep:
		s.metricsNextStep = true
	case *RawPacketsGremlinTraversalStep:
		s.rawpacketsNextStep = true
	}

	if s.context.ReduceRange(next) {
		return s
	}

	return next
}

// Context flow step
func (s *FlowGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

// Exec hops step
func (s *HopsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fts := last.(*FlowTraversalStep)
		return fts.Hops(s.context.Params...), nil
	}

	return nil, traversal.ErrExecutionError
}

// Reduce hops step
func (s *HopsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	if hasStep, ok := next.(*traversal.GremlinTraversalStepHas); ok {
		s.context.Params = hasStep.Params
		return s
	}
	return next
}

// Context hops step
func (s *HopsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

// Exec Nodes step
func (s *NodesGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fts := last.(*FlowTraversalStep)
		return fts.Nodes(s.context.Params...), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce Nodes step
func (s *NodesGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	if hasStep, ok := next.(*traversal.GremlinTraversalStepHas); ok {
		s.context.Params = hasStep.Params
		return s
	}
	return next
}

// Context Nodes step
func (s *NodesGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

// Exec Capture step
func (s *CaptureNodeGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fs := last.(*FlowTraversalStep)
		return fs.CaptureNode(s.context.Params...), nil
	}

	return nil, traversal.ErrExecutionError
}

// Reduce Capture step
func (s *CaptureNodeGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	if hasStep, ok := next.(*traversal.GremlinTraversalStepHas); ok {
		s.context.Params = hasStep.Params
		return s
	}
	return next
}

// Context step
func (s *CaptureNodeGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

// Exec Aggregates step
func (a *AggregatesGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *MetricsTraversalStep:
		mts := last.(*MetricsTraversalStep)
		return mts.Aggregates(), nil
	}

	return nil, traversal.ErrExecutionError
}

// Reduce Aggregates step
func (a *AggregatesGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

// Context Aggregates step
func (a *AggregatesGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &a.context
}

// Exec RawPackets step
func (r *RawPacketsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *FlowTraversalStep:
		fs := last.(*FlowTraversalStep)
		return fs.RawPackets(), nil
	}

	return nil, traversal.ErrExecutionError
}

// Reduce RawPackets step
func (r *RawPacketsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

// Context RawPackets step
func (r *RawPacketsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &r.context
}

// Exec BPF step
func (s *BpfGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *RawPacketsTraversalStep:
		rs := last.(*RawPacketsTraversalStep)
		return rs.BPF(s.context.Params...), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce BPF step
func (s *BpfGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

// Context of BPF step
func (s *BpfGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}
