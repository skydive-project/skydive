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
	"sort"
	"strings"
	"time"

	"github.com/mitchellh/hashstructure"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	defaultSortBy = "CreatedAt"
)

// GraphTraversalStep describes a step in the graph containing Values
type GraphTraversalStep interface {
	Values() []interface{}
	MarshalJSON() ([]byte, error)
	Error() error
}

// GraphStepContext a step within a context
type GraphStepContext struct {
	PaginationRange *GraphTraversalRange
}

// Iterator on the range
func (r *GraphTraversalRange) Iterator() *common.Iterator {
	if r != nil {
		return common.NewIterator(0, r[0], r[1])
	}
	return common.NewIterator()
}

// GraphTraversalRange is within a min and a max
type GraphTraversalRange [2]int64

// GraphTraversal describes multiple step within a graph
type GraphTraversal struct {
	Graph              *graph.Graph
	error              error
	currentStepContext GraphStepContext
	lockGraph          bool
}

type WrapperGraphTraversalStep struct {
	values []interface{}
	err    error
}

func (w *WrapperGraphTraversalStep) Values() []interface{} {
	return w.values
}

func (w *WrapperGraphTraversalStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.values)
}

func (w *WrapperGraphTraversalStep) Error() error {
	return w.err
}

// GraphTraversalV traversal steps on nodes
type GraphTraversalV struct {
	GraphTraversal *GraphTraversal
	nodes          []*graph.Node
	error          error
}

// GraphTraversalE traversal steps on Edges
type GraphTraversalE struct {
	GraphTraversal *GraphTraversal
	edges          []*graph.Edge
	error          error
}

// GraphTraversalShortestPath traversal step shortest path
type GraphTraversalShortestPath struct {
	GraphTraversal *GraphTraversal
	paths          [][]*graph.Node
	error          error
}

// GraphTraversalValue traversal step value
type GraphTraversalValue struct {
	GraphTraversal *GraphTraversal
	value          interface{}
	error          error
}

// MetricsTraversalStep traversal step metric interface counters
type MetricsTraversalStep struct {
	GraphTraversal *GraphTraversal
	metrics        map[string][]*common.TimedMetric
	error          error
}

// ParamToFilter creates a filter based on parameters
// [RegexMetadataMatcher, NE/LT/GT/GTE/LTE/Inside/Outside/Between/Within/Contains/string,int64 MetadataMatcher]
func ParamToFilter(k string, v interface{}) (*filters.Filter, error) {
	switch v := v.(type) {
	case *RegexMetadataMatcher:
		// As we force anchors raise an error if anchor provided by the user
		if strings.HasPrefix(v.regex, "^") || strings.HasSuffix(v.regex, "$") {
			return nil, errors.New("Regex are anchored by default, ^ and $ don't have to be provided")
		}

		// always anchored
		rf, err := filters.NewRegexFilter(k, "^"+v.regex+"$")
		if err != nil {
			return nil, err
		}

		return &filters.Filter{RegexFilter: rf}, nil
	case *NEMetadataMatcher:
		switch t := v.value.(type) {
		case string:
			return filters.NewNotFilter(filters.NewTermStringFilter(k, t)), nil
		default:
			i, err := common.ToInt64(t)
			if err != nil {
				return nil, err
			}
			return filters.NewNotFilter(filters.NewTermInt64Filter(k, i)), nil
		}
	case *LTMetadataMatcher:
		i, err := common.ToInt64(v.value)
		if err != nil {
			return nil, errors.New("LT values should be of int64 type")
		}
		return filters.NewLtInt64Filter(k, i), nil
	case *GTMetadataMatcher:
		i, err := common.ToInt64(v.value)
		if err != nil {
			return nil, errors.New("GT values should be of int64 type")
		}
		return filters.NewGtInt64Filter(k, i), nil
	case *GTEMetadataMatcher:
		i, err := common.ToInt64(v.value)
		if err != nil {
			return nil, errors.New("GTE values should be of int64 type")
		}
		return &filters.Filter{
			GteInt64Filter: &filters.GteInt64Filter{Key: k, Value: i},
		}, nil
	case *LTEMetadataMatcher:
		i, err := common.ToInt64(v.value)
		if err != nil {
			return nil, errors.New("LTE values should be of int64 type")
		}
		return &filters.Filter{
			LteInt64Filter: &filters.LteInt64Filter{Key: k, Value: i},
		}, nil
	case *InsideMetadataMatcher:
		f64, fok := common.ToInt64(v.from)
		t64, tok := common.ToInt64(v.to)

		if fok != nil || tok != nil {
			return nil, errors.New("Inside values should be of int64 type")
		}

		return filters.NewAndFilter(filters.NewGtInt64Filter(k, f64), filters.NewLtInt64Filter(k, t64)), nil
	case *OutsideMetadataMatcher:
		f64, fok := common.ToInt64(v.from)
		t64, tok := common.ToInt64(v.to)

		if fok != nil || tok != nil {
			return nil, errors.New("Outside values should be of int64 type")
		}

		return filters.NewAndFilter(filters.NewLtInt64Filter(k, f64), filters.NewGtInt64Filter(k, t64)), nil
	case *BetweenMetadataMatcher:
		f64, fok := common.ToInt64(v.from)
		t64, tok := common.ToInt64(v.to)

		if fok != nil || tok != nil {
			return nil, errors.New("Between values should be of int64 type")
		}

		return filters.NewAndFilter(filters.NewGteInt64Filter(k, f64), filters.NewLtInt64Filter(k, t64)), nil
	case *WithinMetadataMatcher:
		var orFilters []*filters.Filter
		for _, val := range v.List {
			switch v := val.(type) {
			case string:
				orFilters = append(orFilters, filters.NewTermStringFilter(k, v))
			default:
				i, err := common.ToInt64(v)
				if err != nil {
					return nil, err
				}

				orFilters = append(orFilters, filters.NewTermInt64Filter(k, i))
			}
		}

		return filters.NewOrFilter(orFilters...), nil
	case string:
		return filters.NewTermStringFilter(k, v), nil
	case int64:
		return filters.NewTermInt64Filter(k, v), nil
	case *IPV4RangeMetadataMatcher:
		cidr, ok := v.value.(string)
		if !ok {
			return nil, errors.New("Ipv4Range value has to be a string")
		}

		rf, err := filters.NewIPV4RangeFilter(k, cidr)
		if err != nil {
			return nil, err
		}

		return &filters.Filter{IPV4RangeFilter: rf}, nil
	default:
		i, err := common.ToInt64(v)
		if err != nil {
			return nil, err
		}
		return filters.NewTermInt64Filter(k, i), nil
	}
}

// ParamsToFilter convert multiple pairs to a unique filter, logically AND linked
func ParamsToFilter(params ...interface{}) (*filters.Filter, error) {
	if len(params)%2 != 0 {
		return nil, fmt.Errorf("Slice must be defined by pair k,v: %v", params)
	}

	var andFilters []*filters.Filter
	for i := 0; i < len(params); i += 2 {
		k, ok := params[i].(string)
		if !ok {
			return nil, errors.New("Keys should be of string type")
		}

		filter, err := ParamToFilter(k, params[i+1])
		if err != nil {
			return nil, err
		}
		andFilters = append(andFilters, filter)
	}

	return filters.NewAndFilter(andFilters...), nil
}

// SliceToMetadata slice multiple filters pair to metadata
func SliceToMetadata(s ...interface{}) (graph.Metadata, error) {
	m := graph.Metadata{}
	if len(s)%2 != 0 {
		return m, fmt.Errorf("slice must be defined by pair k,v: %v", s)
	}

	for i := 0; i < len(s); i += 2 {
		k, ok := s[i].(string)
		if !ok {
			return m, errors.New("keys should be of string type")
		}

		filter, err := ParamToFilter(k, s[i+1])
		if err != nil {
			return m, err
		}

		m[k] = filter
	}

	return m, nil
}

// WithinMetadataMatcher describes a list of metadata that should match (within)
type WithinMetadataMatcher struct {
	List []interface{}
}

// Within step
func Within(s ...interface{}) *WithinMetadataMatcher {
	return &WithinMetadataMatcher{List: s}
}

// WithoutMetadataMatcher describes a list of metadata that shouldn't match (without)
type WithoutMetadataMatcher struct {
	list []interface{}
}

// Without step
func Without(s ...interface{}) *WithoutMetadataMatcher {
	return &WithoutMetadataMatcher{list: s}
}

// NEMetadataMatcher describes a list of metadata that match NotEqual
type NEMetadataMatcher struct {
	value interface{}
}

// Ne step
func Ne(s interface{}) *NEMetadataMatcher {
	return &NEMetadataMatcher{value: s}
}

// LTMetadataMatcher describes a list of metadata that match LessThan
type LTMetadataMatcher struct {
	value interface{}
}

// Lt step
func Lt(s interface{}) *LTMetadataMatcher {
	return &LTMetadataMatcher{value: s}
}

// GTMetadataMatcher describes a list of metadata that match GreaterThan
type GTMetadataMatcher struct {
	value interface{}
}

// Gt step
func Gt(s interface{}) *GTMetadataMatcher {
	return &GTMetadataMatcher{value: s}
}

// LTEMetadataMatcher describes a list of metadata that match Less Than Equal
type LTEMetadataMatcher struct {
	value interface{}
}

// Lte step
func Lte(s interface{}) *LTEMetadataMatcher {
	return &LTEMetadataMatcher{value: s}
}

// GTEMetadataMatcher describes a list of metadata that match Greater Than Equal
type GTEMetadataMatcher struct {
	value interface{}
}

// Gte step
func Gte(s interface{}) *GTEMetadataMatcher {
	return &GTEMetadataMatcher{value: s}
}

// InsideMetadataMatcher describes a list of metadata that match inside the range from, to
type InsideMetadataMatcher struct {
	from interface{}
	to   interface{}
}

// Inside step
func Inside(from, to interface{}) *InsideMetadataMatcher {
	return &InsideMetadataMatcher{from: from, to: to}
}

// OutsideMetadataMatcher describes a list of metadata that match outside the range from, to
type OutsideMetadataMatcher struct {
	from interface{}
	to   interface{}
}

// Outside step
func Outside(from, to interface{}) *OutsideMetadataMatcher {
	return &OutsideMetadataMatcher{from: from, to: to}
}

// BetweenMetadataMatcher describes a list of metadata that match between the range from, to
type BetweenMetadataMatcher struct {
	from interface{}
	to   interface{}
}

// Between step
func Between(from interface{}, to interface{}) *BetweenMetadataMatcher {
	return &BetweenMetadataMatcher{from: from, to: to}
}

// RegexMetadataMatcher describes a list of metadata that match a regex
type RegexMetadataMatcher struct {
	regex string
}

// Regex step
func Regex(regex string) *RegexMetadataMatcher {
	return &RegexMetadataMatcher{regex: regex}
}

// IPV4RangeMetadataMatcher matches ipv4 contained in an ipv4 range
type IPV4RangeMetadataMatcher struct {
	value interface{}
}

// IPV4RANGE step
func IPV4Range(s interface{}) *IPV4RangeMetadataMatcher {
	return &IPV4RangeMetadataMatcher{value: s}
}

// Since describes a list of metadata that match since seconds
type Since struct {
	Seconds int64
}

// NewGraphTraversal creates a new graph traversal
func NewGraphTraversal(g *graph.Graph, lockGraph bool) *GraphTraversal {
	return &GraphTraversal{Graph: g, lockGraph: lockGraph}
}

// RLock reads lock the graph
func (t *GraphTraversal) RLock() {
	if t.lockGraph {
		t.Graph.RLock()
	}
}

// RUnlock reads unlock the graph
func (t *GraphTraversal) RUnlock() {
	if t.lockGraph {
		t.Graph.RUnlock()
	}
}

// Values returns the graph values
func (t *GraphTraversal) Values() []interface{} {
	t.RLock()
	defer t.RUnlock()

	return []interface{}{t.Graph}
}

// MarshalJSON serialize in JSON
func (t *GraphTraversal) MarshalJSON() ([]byte, error) {
	values := t.Values()
	t.RLock()
	defer t.RUnlock()
	return json.Marshal(values)
}

func (t *GraphTraversal) Error() error {
	return t.error
}

func parseTimeContext(param string) (time.Time, error) {
	if at, err := time.Parse(time.RFC1123, param); err == nil {
		return at.UTC(), nil
	}

	if d, err := time.ParseDuration(param); err == nil {
		return time.Now().UTC().Add(d), nil
	}

	return time.Time{}, errors.New("Time must be in RFC1123 or in Go Duration format")
}

func (t *GraphTraversal) getPaginationRange() (filter *filters.Range) {
	if t.currentStepContext.PaginationRange != nil {
		filter = &filters.Range{
			From: t.currentStepContext.PaginationRange[0],
			To:   t.currentStepContext.PaginationRange[1],
		}
	}
	return
}

// Context step : at, [duration]
func (t *GraphTraversal) Context(s ...interface{}) *GraphTraversal {
	if t.error != nil {
		return t
	}

	var (
		at       time.Time
		duration time.Duration
		err      error
	)

	at = s[0].(time.Time)
	if len(s) > 1 {
		duration = s[1].(time.Duration)
	}

	if at.After(time.Now().UTC()) {
		return &GraphTraversal{error: errors.New("Sorry, I can't predict the future")}
	}

	t.RLock()
	defer t.RUnlock()

	g, err := t.Graph.WithContext(graph.GraphContext{TimeSlice: common.NewTimeSlice(common.UnixMillis(at.Add(-duration)), common.UnixMillis(at))})
	if err != nil {
		return &GraphTraversal{error: err}
	}

	return &GraphTraversal{Graph: g}
}

// V step : [node ID]
func (t *GraphTraversal) V(s ...interface{}) *GraphTraversalV {
	var nodes []*graph.Node
	var metadata graph.Metadata
	var err error

	if t.error != nil {
		return &GraphTraversalV{error: t.error}
	}

	t.RLock()
	defer t.RUnlock()

	switch len(s) {
	case 1:
		id, ok := s[0].(string)
		if !ok {
			return &GraphTraversalV{error: fmt.Errorf("V accepts only a string when there is only one argument")}
		}
		node := t.Graph.GetNode(graph.Identifier(id))
		if node == nil {
			return &GraphTraversalV{error: fmt.Errorf("Node '%s' does not exist", id)}
		}
		nodes = []*graph.Node{node}
	default:
		if metadata, err = SliceToMetadata(s...); err != nil {
			return &GraphTraversalV{error: err}
		}
		fallthrough
	case 0:
		nodes = t.Graph.GetNodes(metadata)
	}

	if t.currentStepContext.PaginationRange != nil {
		var nodeRange []*graph.Node
		it := t.currentStepContext.PaginationRange.Iterator()
		for _, node := range nodes {
			if it.Done() {
				break
			} else if it.Next() {
				nodeRange = append(nodeRange, node)
			}
		}
		nodes = nodeRange
	}

	return &GraphTraversalV{GraphTraversal: t, nodes: nodes}
}

// NewGraphTraversalV returns a new traversal step
func NewGraphTraversalV(gt *GraphTraversal, nodes []*graph.Node, err ...error) *GraphTraversalV {
	tv := &GraphTraversalV{
		GraphTraversal: gt,
		nodes:          nodes,
	}

	if len(err) > 0 {
		tv.error = err[0]
	}

	return tv
}

func (tv *GraphTraversalV) Error() error {
	return tv.error
}

// Values returns the graph values
func (tv *GraphTraversalV) Values() []interface{} {
	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	s := make([]interface{}, len(tv.nodes))
	for i, n := range tv.nodes {
		s[i] = n
	}
	return s
}

// MarshalJSON serialize in JSON
func (tv *GraphTraversalV) MarshalJSON() ([]byte, error) {
	values := tv.Values()
	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

// GetNodes returns the step nodes
func (tv *GraphTraversalV) GetNodes() (nodes []*graph.Node) {
	return tv.nodes
}

// PropertyValues returns at this step, the values of each metadata selected by the first key
func (tv *GraphTraversalV) PropertyValues(k ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return &GraphTraversalValue{error: tv.error}
	}

	key := k[0].(string)

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	var s []interface{}
	for _, n := range tv.nodes {
		if value, ok := n.Metadata()[key]; ok {
			s = append(s, value)
		}
	}
	return &GraphTraversalValue{GraphTraversal: tv.GraphTraversal, value: s}
}

// PropertyKeys returns at this step, all the metadata keys of each metadata
func (tv *GraphTraversalV) PropertyKeys(keys ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return &GraphTraversalValue{error: tv.error}
	}

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	var s []interface{}
	for _, n := range tv.nodes {
		for key := range n.Metadata() {
			s = append(s, key)
		}
	}

	return &GraphTraversalValue{GraphTraversal: tv.GraphTraversal, value: s}
}

// Sum step : key
// returns the sum of the metadata values of the first argument key
func (tv *GraphTraversalV) Sum(keys ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return &GraphTraversalValue{error: tv.error}
	}

	if len(keys) != 1 {
		return &GraphTraversalValue{error: fmt.Errorf("Sum requires 1 parameter")}
	}
	key, ok := keys[0].(string)
	if !ok {
		return &GraphTraversalValue{error: fmt.Errorf("Sum parameter has to be a string key")}
	}

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	var s float64
	for _, n := range tv.nodes {
		if value, err := n.GetFieldInt64(key); err == nil {
			if v, err := common.ToFloat64(value); err == nil {
				s += v
			} else {
				return &GraphTraversalValue{error: err}
			}
		} else {
			if err != common.ErrFieldNotFound {
				return &GraphTraversalValue{error: err}
			}
		}
	}
	return &GraphTraversalValue{GraphTraversal: tv.GraphTraversal, value: s}
}

// E step : [edge ID]
func (t *GraphTraversal) E(s ...interface{}) *GraphTraversalE {
	var edges []*graph.Edge
	var metadata graph.Metadata
	var err error

	if t.error != nil {
		return &GraphTraversalE{error: t.error}
	}

	t.RLock()
	defer t.RUnlock()

	switch len(s) {
	case 1:
		id, ok := s[0].(string)
		if !ok {
			return &GraphTraversalE{error: fmt.Errorf("E accepts only a string when there is only one argument")}
		}
		edge := t.Graph.GetEdge(graph.Identifier(id))
		if edge == nil {
			return &GraphTraversalE{error: fmt.Errorf("Edge '%s' does not exist", id)}
		}
		edges = []*graph.Edge{edge}
	default:
		if metadata, err = SliceToMetadata(s...); err != nil {
			return &GraphTraversalE{error: err}
		}
		fallthrough
	case 0:
		edges = t.Graph.GetEdges(metadata)
	}

	if t.currentStepContext.PaginationRange != nil {
		var edgeRange []*graph.Edge
		it := t.currentStepContext.PaginationRange.Iterator()
		for _, edge := range edges {
			if it.Done() {
				break
			} else if it.Next() {
				edgeRange = append(edgeRange, edge)
			}
		}
		edges = edgeRange
	}

	return &GraphTraversalE{GraphTraversal: t, edges: edges}
}

// NewGraphTraversalE creates a new graph traversal Edges
func NewGraphTraversalE(gt *GraphTraversal, edges []*graph.Edge, err ...error) *GraphTraversalE {
	te := &GraphTraversalE{
		GraphTraversal: gt,
		edges:          edges,
	}

	if len(err) > 0 {
		te.error = err[0]
	}

	return te
}

func (te *GraphTraversalE) Error() error {
	return te.error
}

// Values returns the graph values
func (te *GraphTraversalE) Values() []interface{} {
	te.GraphTraversal.RLock()
	defer te.GraphTraversal.RUnlock()

	s := make([]interface{}, len(te.edges))
	for i, e := range te.edges {
		s[i] = e
	}
	return s
}

// MarshalJSON serialize in JSON
func (te *GraphTraversalE) MarshalJSON() ([]byte, error) {
	values := te.Values()
	te.GraphTraversal.RLock()
	defer te.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

// ParseSortParameter helper
func ParseSortParameter(keys ...interface{}) (order common.SortOrder, sortBy string, err error) {
	order = common.SortAscending

	switch len(keys) {
	case 0:
	case 2:
		var ok1, ok2 bool

		order, ok1 = keys[0].(common.SortOrder)
		sortBy, ok2 = keys[1].(string)
		if !ok1 || !ok2 {
			return order, sortBy, fmt.Errorf("Sort parameters has to be SortOrder(ASC/DESC) and a sort string Key")
		}
	default:
		return order, sortBy, fmt.Errorf("Sort accepts up to 2 parameters only")
	}

	return order, sortBy, err
}

const (
	sortByInt64 int = iota + 1
	sortByString
)

type sortableNodeSlice struct {
	sortBy     string
	sortOrder  common.SortOrder
	nodes      []*graph.Node
	sortByType int
}

func (s sortableNodeSlice) Len() int {
	return len(s.nodes)
}

func (s sortableNodeSlice) lessInt64(i, j int) bool {
	i1, _ := s.nodes[i].GetFieldInt64(s.sortBy)
	i2, _ := s.nodes[j].GetFieldInt64(s.sortBy)

	if s.sortOrder == common.SortAscending {
		return i1 < i2
	}
	return i1 > i2
}

func (s sortableNodeSlice) lessString(i, j int) bool {
	s1, _ := s.nodes[i].GetFieldString(s.sortBy)
	s2, _ := s.nodes[j].GetFieldString(s.sortBy)

	if s.sortOrder == common.SortAscending {
		return s1 < s2
	}
	return s1 > s2
}

func (s sortableNodeSlice) Less(i, j int) bool {
	switch s.sortByType {
	case sortByInt64:
		return s.lessInt64(i, j)
	case sortByString:
		return s.lessString(i, j)
	}

	// detection of type
	if _, err := s.nodes[i].GetFieldInt64(s.sortBy); err == nil {
		s.sortByType = sortByInt64
		return s.lessInt64(i, j)
	}

	s.sortByType = sortByString
	return s.lessString(i, j)
}

func (s sortableNodeSlice) Swap(i, j int) {
	s.nodes[i], s.nodes[j] = s.nodes[j], s.nodes[i]
}

// Sort step
func (tv *GraphTraversalV) Sort(keys ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	order, sortBy, err := ParseSortParameter(keys...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	if sortBy == "" {
		sortBy = defaultSortBy
	}

	sortable := sortableNodeSlice{
		sortBy:    sortBy,
		sortOrder: order,
		nodes:     tv.nodes,
	}
	sort.Sort(sortable)

	return tv
}

// Dedup step : deduplicate output
func (tv *GraphTraversalV) Dedup(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	var keys []string
	if len(s) > 0 {
		for _, key := range s {
			k, ok := key.(string)
			if !ok {
				return &GraphTraversalV{error: fmt.Errorf("Dedup parameters have to be string keys")}
			}
			keys = append(keys, k)
		}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	visited := make(map[interface{}]bool)
	var kvisited interface{}
	var err error

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeLoop:
	for _, n := range tv.nodes {
		if it.Done() {
			break
		}

		skip := false
		if len(keys) != 0 {
			values := make([]interface{}, len(keys))
			for i, key := range keys {
				v, err := n.GetField(key)
				if err != nil {
					continue nodeLoop
				}
				values[i] = v
			}

			kvisited, err = hashstructure.Hash(values, nil)
			if err != nil {
				skip = true
			}
		} else {
			kvisited = n.ID
		}

		_, ok := visited[kvisited]
		if ok || !it.Next() {
			continue
		}

		ntv.nodes = append(ntv.nodes, n)
		if !skip {
			visited[kvisited] = true
		}
	}

	return ntv
}

// Values returns the graph values
func (sp *GraphTraversalShortestPath) Values() []interface{} {
	sp.GraphTraversal.RLock()
	defer sp.GraphTraversal.RUnlock()

	s := make([]interface{}, len(sp.paths))
	for i, p := range sp.paths {
		s[i] = p
	}
	return s
}

// MarshalJSON serialize in JSON
func (sp *GraphTraversalShortestPath) MarshalJSON() ([]byte, error) {
	values := sp.Values()
	sp.GraphTraversal.RLock()
	defer sp.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

func (sp *GraphTraversalShortestPath) Error() error {
	return sp.error
}

// GetNodes : returns all the nodes in single array, so it will used to find flows.
func (sp *GraphTraversalShortestPath) GetNodes() []*graph.Node {
	var nodes []*graph.Node
	for _, v := range sp.paths {
		nodes = append(nodes, v...)
	}
	return nodes
}

// ShortestPathTo step
func (tv *GraphTraversalV) ShortestPathTo(m graph.Metadata, e graph.Metadata) *GraphTraversalShortestPath {
	if tv.error != nil {
		return &GraphTraversalShortestPath{error: tv.error}
	}

	sp := &GraphTraversalShortestPath{GraphTraversal: tv.GraphTraversal, paths: [][]*graph.Node{}}

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	visited := make(map[graph.Identifier]bool)
	for _, n := range tv.nodes {
		if _, ok := visited[n.ID]; !ok {
			path := tv.GraphTraversal.Graph.LookupShortestPath(n, m, e)
			if len(path) > 0 {
				sp.paths = append(sp.paths, path)
			}
		}
	}
	return sp
}

// Has step
func (tv *GraphTraversalV) Has(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	var err error
	var filter *filters.Filter
	switch len(s) {
	case 0:
		return &GraphTraversalV{error: errors.New("At least one parameter must be provided")}
	case 1:
		k, ok := s[0].(string)
		if !ok {
			return &GraphTraversalV{error: errors.New("Key must be a string")}
		}
		filter = filters.NewNotFilter(filters.NewNullFilter(k))
	default:
		filter, err = ParamsToFilter(s...)
		if err != nil {
			return &GraphTraversalV{error: err}
		}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	for _, n := range tv.nodes {
		if it.Done() {
			break
		}
		if (filter == nil || filter.Eval(n)) && it.Next() {
			ntv.nodes = append(ntv.nodes, n)
		}
	}

	return ntv
}

// HasKey step
func (tv *GraphTraversalV) HasKey(s string) *GraphTraversalV {
	return tv.Has(s)
}

// HasNot step
func (tv *GraphTraversalV) HasNot(s string) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	filter := filters.NewNullFilter(s)
	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	for _, n := range tv.nodes {
		if it.Done() {
			break
		}
		if (filter == nil || filter.Eval(n)) && it.Next() {
			ntv.nodes = append(ntv.nodes, n)
		}
	}

	return ntv
}

// Both step
func (tv *GraphTraversalV) Both(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n, nil) {
			var nodes []*graph.Node
			if e.GetChild() == n.ID {
				nodes, _ = tv.GraphTraversal.Graph.GetEdgeNodes(e, metadata, nil)
			} else {
				_, nodes = tv.GraphTraversal.Graph.GetEdgeNodes(e, nil, metadata)
			}

			for _, node := range nodes {
				if it.Done() {
					break nodeloop
				} else if it.Next() {
					ntv.nodes = append(ntv.nodes, node)
				}
			}
		}
	}

	return ntv
}

// Count step
func (tv *GraphTraversalV) Count(s ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return &GraphTraversalValue{error: tv.error}
	}

	return &GraphTraversalValue{GraphTraversal: tv.GraphTraversal, value: len(tv.nodes)}
}

// Range step
func (tv *GraphTraversalV) Range(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	if len(s) == 2 {
		from, ok := s[0].(int64)
		if !ok {
			return &GraphTraversalV{error: fmt.Errorf("%s is not an integer", s[0])}
		}
		to, ok := s[1].(int64)
		if !ok {
			return &GraphTraversalV{error: fmt.Errorf("%s is not an integer", s[1])}
		}
		var nodes []*graph.Node
		for ; from < int64(len(tv.nodes)) && from < to; from++ {
			nodes = append(nodes, tv.nodes[from])
		}
		return &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: nodes}
	}

	return &GraphTraversalV{error: errors.New("2 parameters must be provided to 'range'")}
}

// Limit step
func (tv *GraphTraversalV) Limit(s ...interface{}) *GraphTraversalV {
	return tv.Range(int64(0), s[0])
}

// Out step : out of a node step
func (tv *GraphTraversalV) Out(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.nodes {
		for _, child := range tv.GraphTraversal.Graph.LookupChildren(n, metadata, nil) {
			if it.Done() {
				break nodeloop
			} else if it.Next() {
				ntv.nodes = append(ntv.nodes, child)
			}
		}
	}

	return ntv
}

// OutE step : out of an edge
func (tv *GraphTraversalV) OutE(s ...interface{}) *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{error: tv.error}
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalE{error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: tv.GraphTraversal, edges: []*graph.Edge{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n, metadata) {
			if e.GetParent() == n.ID {
				if it.Done() {
					break nodeloop
				} else {
					nte.edges = append(nte.edges, e)
				}
			}
		}
	}

	return nte
}

// BothE : both are edges
func (tv *GraphTraversalV) BothE(s ...interface{}) *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{error: tv.error}
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalE{GraphTraversal: tv.GraphTraversal, error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: tv.GraphTraversal, edges: []*graph.Edge{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n, metadata) {
			if it.Done() {
				break nodeloop
			} else if it.Next() {
				nte.edges = append(nte.edges, e)
			}
		}
	}

	return nte
}

// In node step
func (tv *GraphTraversalV) In(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.nodes {
		for _, parent := range tv.GraphTraversal.Graph.LookupParents(n, metadata, nil) {
			if it.Done() {
				break nodeloop
			} else {
				ntv.nodes = append(ntv.nodes, parent)
			}
		}
	}

	return ntv
}

// InE step of an node
func (tv *GraphTraversalV) InE(s ...interface{}) *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{error: tv.error}
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalE{GraphTraversal: tv.GraphTraversal, error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: tv.GraphTraversal, edges: []*graph.Edge{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n, metadata) {
			if e.GetChild() == n.ID {
				if it.Done() {
					break nodeloop
				} else if it.Next() {
					nte.edges = append(nte.edges, e)
				}
			}
		}
	}

	return nte
}

// SubGraph step, node/edge out
func (tv *GraphTraversalV) SubGraph(s ...interface{}) *GraphTraversal {
	if tv.error != nil {
		return &GraphTraversal{error: tv.error}
	}

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	memory, err := graph.NewMemoryBackend()
	if err != nil {
		return &GraphTraversal{error: err}
	}

	// first insert all the nodes
	for _, n := range tv.nodes {
		if !memory.NodeAdded(n) {
			return &GraphTraversal{error: errors.New("Error while adding node to SubGraph")}
		}
	}

	// then insert edges, ignore edge insert error since one of the linked node couldn't be part
	// of the SubGraph
	for _, n := range tv.nodes {
		edges := tv.GraphTraversal.Graph.GetNodeEdges(n, nil)
		for _, e := range edges {
			memory.EdgeAdded(e)
		}
	}

	ng := graph.NewGraph(tv.GraphTraversal.Graph.GetHost(), memory)

	return NewGraphTraversal(ng, tv.GraphTraversal.lockGraph)
}

// Metrics step : packets counters
func (tv *GraphTraversalV) Metrics() *MetricsTraversalStep {
	if tv.error != nil {
		return &MetricsTraversalStep{error: tv.error}
	}

	tv = tv.Dedup("ID", "LastMetric.Start").Sort(common.SortAscending, "LastMetric.Start")
	if tv.error != nil {
		return &MetricsTraversalStep{error: tv.error}
	}

	metrics := make(map[string][]*common.TimedMetric)
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()
	gslice := tv.GraphTraversal.Graph.GetContext().TimeSlice

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.nodes {
		if it.Done() {
			break nodeloop
		}

		m := n.Metadata()
		lastMetric, hasLastMetric := m["LastMetric"].(map[string]interface{})
		if hasLastMetric {
			start := lastMetric["Start"].(int64)
			last := lastMetric["Last"].(int64)
			if gslice == nil || (start > gslice.Start && last < gslice.Last) {
				im := &graph.InterfaceMetric{
					RxPackets:         lastMetric["RxPackets"].(int64),
					TxPackets:         lastMetric["TxPackets"].(int64),
					RxBytes:           lastMetric["RxBytes"].(int64),
					TxBytes:           lastMetric["TxBytes"].(int64),
					RxErrors:          lastMetric["RxErrors"].(int64),
					TxErrors:          lastMetric["TxErrors"].(int64),
					RxDropped:         lastMetric["RxDropped"].(int64),
					TxDropped:         lastMetric["TxDropped"].(int64),
					Multicast:         lastMetric["Multicast"].(int64),
					Collisions:        lastMetric["Collisions"].(int64),
					RxLengthErrors:    lastMetric["RxLengthErrors"].(int64),
					RxOverErrors:      lastMetric["RxOverErrors"].(int64),
					RxCrcErrors:       lastMetric["RxCrcErrors"].(int64),
					RxFrameErrors:     lastMetric["RxFrameErrors"].(int64),
					RxFifoErrors:      lastMetric["RxFifoErrors"].(int64),
					RxMissedErrors:    lastMetric["RxMissedErrors"].(int64),
					TxAbortedErrors:   lastMetric["TxAbortedErrors"].(int64),
					TxCarrierErrors:   lastMetric["TxCarrierErrors"].(int64),
					TxFifoErrors:      lastMetric["TxFifoErrors"].(int64),
					TxHeartbeatErrors: lastMetric["TxHeartbeatErrors"].(int64),
					TxWindowErrors:    lastMetric["TxWindowErrors"].(int64),
					RxCompressed:      lastMetric["RxCompressed"].(int64),
					TxCompressed:      lastMetric["TxCompressed"].(int64),
				}
				metric := &common.TimedMetric{
					TimeSlice: *common.NewTimeSlice(start, last),
					Metric:    im,
				}
				metrics[string(n.ID)] = append(metrics[string(n.ID)], metric)
			}
		}
	}

	return NewMetricsTraversalStep(tv.GraphTraversal, metrics, nil)
}

// Count step
func (te *GraphTraversalE) Count(s ...interface{}) *GraphTraversalValue {
	if te.error != nil {
		return &GraphTraversalValue{error: te.error}
	}

	return &GraphTraversalValue{GraphTraversal: te.GraphTraversal, value: len(te.edges)}
}

// Range step
func (te *GraphTraversalE) Range(s ...interface{}) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	switch len(s) {
	case 2:
		from, ok := s[0].(int64)
		if !ok {
			return &GraphTraversalE{error: fmt.Errorf("%s is not an integer", s[0])}
		}
		to, ok := s[1].(int64)
		if !ok {
			return &GraphTraversalE{error: fmt.Errorf("%s is not an integer", s[1])}
		}
		var edges []*graph.Edge
		for ; from < int64(len(te.edges)) && from < to; from++ {
			edges = append(edges, te.edges[from])
		}
		return &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: edges}

	default:
		return &GraphTraversalE{GraphTraversal: te.GraphTraversal, error: errors.New("2 parameters must be provided to 'range'")}
	}
}

// Limit step
func (te *GraphTraversalE) Limit(s ...interface{}) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	return te.Range(int64(0), s[0])
}

// Dedup step : deduplicate
func (te *GraphTraversalE) Dedup(keys ...interface{}) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	var key string
	if len(keys) > 0 {
		k, ok := keys[0].(string)
		if !ok {
			return &GraphTraversalE{error: fmt.Errorf("Dedup parameter has to be a string key")}
		}
		key = k
	}

	ntv := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*graph.Edge{}}
	visited := make(map[interface{}]bool)
	var kvisited interface{}

	te.GraphTraversal.RLock()
	defer te.GraphTraversal.RUnlock()

	for _, e := range te.edges {
		kvisited = e.ID
		if key != "" {
			if v, ok := e.Metadata()[key]; ok {
				kvisited = v
			}
		}

		if _, ok := visited[kvisited]; !ok {
			ntv.edges = append(ntv.edges, e)
			visited[kvisited] = true
		}
	}
	return ntv
}

// Has step
func (te *GraphTraversalE) Has(s ...interface{}) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	var err error
	var filter *filters.Filter
	switch len(s) {
	case 0:
		return &GraphTraversalE{error: errors.New("At least one parameter must be provided")}
	case 1:
		k, ok := s[0].(string)
		if !ok {
			return &GraphTraversalE{error: errors.New("Key must be a string")}
		}
		filter = filters.NewNotFilter(filters.NewNullFilter(k))
	default:
		filter, err = ParamsToFilter(s...)
		if err != nil {
			return &GraphTraversalE{error: err}
		}
	}

	nte := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*graph.Edge{}}
	it := te.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	te.GraphTraversal.RLock()
	defer te.GraphTraversal.RUnlock()

	for _, e := range te.edges {
		if it.Done() {
			break
		}
		if (filter == nil || filter.Eval(e)) && it.Next() {
			nte.edges = append(nte.edges, e)
		}
	}

	return nte
}

// HasKey step
func (te *GraphTraversalE) HasKey(s string) *GraphTraversalE {
	return te.Has(s)
}

// HasNot step
func (te *GraphTraversalE) HasNot(s string) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	filter := filters.NewNullFilter(s)
	nte := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*graph.Edge{}}
	it := te.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	te.GraphTraversal.RLock()
	defer te.GraphTraversal.RUnlock()

	for _, e := range te.edges {
		if it.Done() {
			break
		}
		if (filter == nil || filter.Eval(e)) && it.Next() {
			nte.edges = append(nte.edges, e)
		}
	}

	return nte
}

// InV step, node in
func (te *GraphTraversalE) InV(s ...interface{}) *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{error: te.error}
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: te.GraphTraversal, nodes: []*graph.Node{}}
	it := te.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	te.GraphTraversal.RLock()
	defer te.GraphTraversal.RUnlock()

	for _, e := range te.edges {
		parents, _ := te.GraphTraversal.Graph.GetEdgeNodes(e, metadata, nil)
		for _, parent := range parents {
			if it.Done() {
				break
			} else if it.Next() {
				ntv.nodes = append(ntv.nodes, parent)
			}
		}
	}

	return ntv
}

// OutV step, node out
func (te *GraphTraversalE) OutV(s ...interface{}) *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{error: te.error}
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: te.GraphTraversal, nodes: []*graph.Node{}}
	it := te.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	te.GraphTraversal.RLock()
	defer te.GraphTraversal.RUnlock()

	for _, e := range te.edges {
		_, children := te.GraphTraversal.Graph.GetEdgeNodes(e, nil, metadata)
		for _, child := range children {
			if it.Done() {
				break
			} else if it.Next() {
				ntv.nodes = append(ntv.nodes, child)
			}
		}
	}

	return ntv
}

// BothV step, nodes in/out
func (te *GraphTraversalE) BothV(s ...interface{}) *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{error: te.error}
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := NewGraphTraversalV(te.GraphTraversal, []*graph.Node{})
	it := te.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	te.GraphTraversal.RLock()
	defer te.GraphTraversal.RUnlock()

	for _, e := range te.edges {
		parents, children := te.GraphTraversal.Graph.GetEdgeNodes(e, metadata, metadata)
		for _, parent := range parents {
			if it.Done() {
				break
			} else if it.Next() {
				ntv.nodes = append(ntv.nodes, parent)
			}
		}
		for _, child := range children {
			if it.Done() {
				break
			} else if it.Next() {
				ntv.nodes = append(ntv.nodes, child)
			}
		}
	}

	return ntv
}

// SubGraph step, node/edge out
func (te *GraphTraversalE) SubGraph(s ...interface{}) *GraphTraversal {
	if te.error != nil {
		return &GraphTraversal{error: te.error}
	}

	te.GraphTraversal.RLock()
	defer te.GraphTraversal.RUnlock()

	memory, err := graph.NewMemoryBackend()
	if err != nil {
		return &GraphTraversal{error: err}
	}

	for _, e := range te.edges {
		parents, children := te.GraphTraversal.Graph.GetEdgeNodes(e, nil, nil)
		for _, child := range children {
			if !memory.NodeAdded(child) {
				return &GraphTraversal{error: errors.New("Error while adding node to SubGraph")}
			}
		}

		for _, parent := range parents {
			if !memory.NodeAdded(parent) {
				return &GraphTraversal{error: errors.New("Error while adding node to SubGraph")}
			}
		}

		if !memory.EdgeAdded(e) {
			return &GraphTraversal{error: errors.New("Error while adding edge to SubGraph")}
		}
	}

	ng := graph.NewGraph(te.GraphTraversal.Graph.GetHost(), memory)

	return NewGraphTraversal(ng, te.GraphTraversal.lockGraph)
}

// NewGraphTraversalValue creates a new traversal value step
func NewGraphTraversalValue(gt *GraphTraversal, value interface{}, err ...error) *GraphTraversalValue {
	tv := &GraphTraversalValue{
		GraphTraversal: gt,
		value:          value,
	}

	if len(err) > 0 {
		tv.error = err[0]
	}

	return tv
}

// Values return the graph values
func (t *GraphTraversalValue) Values() []interface{} {
	// Values like all step has to return an array of interface
	// if v is already an array return it otherwise instanciate an new array
	// with the value as first element.
	if v, ok := t.value.([]interface{}); ok {
		return v
	}
	return []interface{}{t.value}
}

// MarshalJSON serialize in JSON
func (t *GraphTraversalValue) MarshalJSON() ([]byte, error) {
	t.GraphTraversal.RLock()
	defer t.GraphTraversal.RUnlock()
	return json.Marshal(t.value)
}

func (t *GraphTraversalValue) Error() error {
	return t.error
}

// Dedup step : deduplicate
func (t *GraphTraversalValue) Dedup(keys ...interface{}) *GraphTraversalValue {
	if t.error != nil {
		return t
	}

	var nv []interface{}
	ntv := &GraphTraversalValue{GraphTraversal: t.GraphTraversal, value: nv}
	visited := make(map[interface{}]bool)
	for _, v := range t.Values() {
		if _, ok := visited[v]; !ok {
			visited[v] = true
			ntv.value = append(ntv.value.([]interface{}), v)
		}
	}
	return ntv
}

// Sum aggregates integer values mapped by 'key' cross flows
func (m *MetricsTraversalStep) Sum(keys ...interface{}) *GraphTraversalValue {
	if m.error != nil {
		return NewGraphTraversalValue(m.GraphTraversal, nil, m.error)
	}

	if len(keys) > 0 {
		if len(keys) != 1 {
			return NewGraphTraversalValue(m.GraphTraversal, nil, fmt.Errorf("Sum requires 1 parameter"))
		}

		key, ok := keys[0].(string)
		if !ok {
			return NewGraphTraversalValue(m.GraphTraversal, nil, errors.New("Argument of Sum must be a string"))
		}

		var total int64
		for _, metrics := range m.metrics {
			for _, metric := range metrics {
				value, err := metric.GetFieldInt64(key)
				if err != nil {
					NewGraphTraversalValue(m.GraphTraversal, nil, err)
				}
				total += value
			}
		}
		return NewGraphTraversalValue(m.GraphTraversal, total)
	}

	total := common.TimedMetric{}
	for _, metrics := range m.metrics {
		for _, metric := range metrics {
			if total.Metric == nil {
				total.Metric = metric.Metric
			} else {
				total.Metric.Add(metric.Metric)
			}

			if total.Start == 0 || total.Start > metric.Start {
				total.Start = metric.Start
			}

			if total.Last == 0 || total.Last < metric.Last {
				total.Last = metric.Last
			}
		}
	}

	return NewGraphTraversalValue(m.GraphTraversal, &total)
}

func aggregateMetrics(a, b []*common.TimedMetric) []*common.TimedMetric {
	var result []*common.TimedMetric
	boundA, boundB := len(a)-1, len(b)-1

	var i, j int
	for i <= boundA || j <= boundB {
		if i > boundA && j <= boundB {
			return append(result, b[j:]...)
		} else if j > boundB && i <= boundA {
			return append(result, a[i:]...)
		} else if a[i].Last < b[j].Start {
			// metric a is strictly before metric b
			result = append(result, a[i])
			i++
		} else if b[j].Last < a[i].Start {
			// metric b is strictly before metric a
			result = append(result, b[j])
			j++
		} else {
			start := a[i].Start
			last := a[i].Last
			if a[i].Start > b[j].Start {
				start = b[j].Start
				last = b[j].Last
			}

			// in case of an overlap then summing using the smallest start/last slice
			var metric = a[i].Metric
			metric.Add(b[j].Metric)

			result = append(result, &common.TimedMetric{
				Metric:    metric,
				TimeSlice: *common.NewTimeSlice(start, last),
			})
			i++
			j++
		}
	}
	return result
}

// Aggregates merges multiple metrics array into one by summing overlapping
// metrics. It returns a unique array will all the aggregated metrics.
func (m *MetricsTraversalStep) Aggregates() *MetricsTraversalStep {
	if m.error != nil {
		return m
	}

	var aggregated []*common.TimedMetric
	for _, metrics := range m.metrics {
		aggregated = aggregateMetrics(aggregated, metrics)
	}

	return &MetricsTraversalStep{GraphTraversal: m.GraphTraversal, metrics: map[string][]*common.TimedMetric{"Aggregated": aggregated}}
}

// Values returns the graph metric values
func (m *MetricsTraversalStep) Values() []interface{} {
	if len(m.metrics) == 0 {
		return []interface{}{}
	}
	return []interface{}{m.metrics}
}

// MarshalJSON serialize in JSON
func (m *MetricsTraversalStep) MarshalJSON() ([]byte, error) {
	values := m.Values()
	m.GraphTraversal.RLock()
	defer m.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

// Error returns error present at this step
func (m *MetricsTraversalStep) Error() error {
	return m.error
}

// Count step
func (m *MetricsTraversalStep) Count(s ...interface{}) *GraphTraversalValue {
	return NewGraphTraversalValue(m.GraphTraversal, len(m.metrics))
}

// NewMetricsTraversalStep creates a new tranversal metric step
func NewMetricsTraversalStep(gt *GraphTraversal, metrics map[string][]*common.TimedMetric, err error) *MetricsTraversalStep {
	return &MetricsTraversalStep{GraphTraversal: gt, metrics: metrics, error: err}
}
