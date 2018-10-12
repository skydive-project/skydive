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
	"reflect"
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

// StepContext a step within a context
type StepContext struct {
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
	Graph     *graph.Graph
	error     error
	lockGraph bool
	as        map[string]*GraphTraversalAs
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

// GraphTraversalAs store a state of the nodes selected
type GraphTraversalAs struct {
	GraphTraversal *GraphTraversal
	nodes          []*graph.Node
}

// KeyValueToFilter creates a filter for a key with a fixed value or a predicate
func KeyValueToFilter(k string, v interface{}) (*filters.Filter, error) {
	switch v := v.(type) {
	case *RegexElementMatcher:
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
	case *NEElementMatcher:
		switch t := v.value.(type) {
		case string:
			return filters.NewNotFilter(filters.NewTermStringFilter(k, t)), nil
		case bool:
			return filters.NewTermBoolFilter(k, !t), nil
		default:
			i, err := common.ToInt64(t)
			if err != nil {
				return nil, err
			}
			return filters.NewNotFilter(filters.NewTermInt64Filter(k, i)), nil
		}
	case *LTElementMatcher:
		i, err := common.ToInt64(v.value)
		if err != nil {
			return nil, errors.New("LT values should be of int64 type")
		}
		return filters.NewLtInt64Filter(k, i), nil
	case *GTElementMatcher:
		i, err := common.ToInt64(v.value)
		if err != nil {
			return nil, errors.New("GT values should be of int64 type")
		}
		return filters.NewGtInt64Filter(k, i), nil
	case *GTEElementMatcher:
		i, err := common.ToInt64(v.value)
		if err != nil {
			return nil, errors.New("GTE values should be of int64 type")
		}
		return &filters.Filter{
			GteInt64Filter: &filters.GteInt64Filter{Key: k, Value: i},
		}, nil
	case *LTEElementMatcher:
		i, err := common.ToInt64(v.value)
		if err != nil {
			return nil, errors.New("LTE values should be of int64 type")
		}
		return &filters.Filter{
			LteInt64Filter: &filters.LteInt64Filter{Key: k, Value: i},
		}, nil
	case *InsideElementMatcher:
		f64, fok := common.ToInt64(v.from)
		t64, tok := common.ToInt64(v.to)

		if fok != nil || tok != nil {
			return nil, errors.New("Inside values should be of int64 type")
		}

		return filters.NewAndFilter(filters.NewGtInt64Filter(k, f64), filters.NewLtInt64Filter(k, t64)), nil
	case *OutsideElementMatcher:
		f64, fok := common.ToInt64(v.from)
		t64, tok := common.ToInt64(v.to)

		if fok != nil || tok != nil {
			return nil, errors.New("Outside values should be of int64 type")
		}

		return filters.NewAndFilter(filters.NewLtInt64Filter(k, f64), filters.NewGtInt64Filter(k, t64)), nil
	case *BetweenElementMatcher:
		f64, fok := common.ToInt64(v.from)
		t64, tok := common.ToInt64(v.to)

		if fok != nil || tok != nil {
			return nil, errors.New("Between values should be of int64 type")
		}

		return filters.NewAndFilter(filters.NewGteInt64Filter(k, f64), filters.NewLtInt64Filter(k, t64)), nil
	case *WithinElementMatcher:
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
	case bool:
		return filters.NewTermBoolFilter(k, v), nil
	case *IPV4RangeElementMatcher:
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

// ParamsToMetadataFilter converts a slice to a ElementMatcher
func ParamsToMetadataFilter(s ...interface{}) (graph.ElementMatcher, error) {
	m, err := ParamsToMap(s...)
	if err != nil {
		return nil, err
	}

	return MapToMetadataFilter(m)
}

// ParamsToMetadata converts a slice to a ElementMatcher
func ParamsToMetadata(s ...interface{}) (graph.Metadata, error) {
	m, err := ParamsToMap(s...)
	if err != nil {
		return nil, err
	}

	return graph.Metadata(m), nil
}

// ParamsToFilter converts a slice to a filter
func ParamsToFilter(s ...interface{}) (*filters.Filter, error) {
	m, err := ParamsToMap(s...)
	if err != nil {
		return nil, err
	}

	return MapToFilter(m)
}

// MapToFilter return a AND filter of the given map
func MapToFilter(m map[string]interface{}) (*filters.Filter, error) {
	var lf []*filters.Filter
	for k, v := range m {
		filter, err := KeyValueToFilter(k, v)
		if err != nil {
			return nil, err
		}

		lf = append(lf, filter)
	}
	return filters.NewBoolFilter(filters.BoolFilterOp_AND, lf...), nil
}

// MapToMetadataFilter converts a map to a ElementMatcher
func MapToMetadataFilter(m map[string]interface{}) (graph.ElementMatcher, error) {
	filter, err := MapToFilter(m)
	if err != nil {
		return nil, err
	}
	return graph.NewElementFilter(filter), nil
}

// ParamsToMap converts a slice to a map
func ParamsToMap(s ...interface{}) (map[string]interface{}, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("slice must be defined by pair k,v: %v", s)
	}

	m := make(map[string]interface{})
	for i := 0; i < len(s); i += 2 {
		k, ok := s[i].(string)
		if !ok {
			return nil, errors.New("keys should be of string type")
		}

		m[k] = s[i+1]
	}

	return m, nil
}

// WithinElementMatcher describes a list of metadata that should match (within)
type WithinElementMatcher struct {
	List []interface{}
}

// Within predicate
func Within(s ...interface{}) *WithinElementMatcher {
	return &WithinElementMatcher{List: s}
}

// WithoutElementMatcher describes a list of metadata that shouldn't match (without)
type WithoutElementMatcher struct {
	list []interface{}
}

// Without predicate
func Without(s ...interface{}) *WithoutElementMatcher {
	return &WithoutElementMatcher{list: s}
}

// NEElementMatcher describes a list of metadata that match NotEqual
type NEElementMatcher struct {
	value interface{}
}

// Ne predicate
func Ne(s interface{}) *NEElementMatcher {
	return &NEElementMatcher{value: s}
}

// LTElementMatcher describes a list of metadata that match LessThan
type LTElementMatcher struct {
	value interface{}
}

// Lt predicate
func Lt(s interface{}) *LTElementMatcher {
	return &LTElementMatcher{value: s}
}

// GTElementMatcher describes a list of metadata that match GreaterThan
type GTElementMatcher struct {
	value interface{}
}

// Gt predicate
func Gt(s interface{}) *GTElementMatcher {
	return &GTElementMatcher{value: s}
}

// LTEElementMatcher describes a list of metadata that match Less Than Equal
type LTEElementMatcher struct {
	value interface{}
}

// Lte predicate
func Lte(s interface{}) *LTEElementMatcher {
	return &LTEElementMatcher{value: s}
}

// ForeverPredicate describes a entire time limit in the history
type ForeverPredicate struct{}

// NowPredicate describes a current time in the history
type NowPredicate struct{}

// GTEElementMatcher describes a list of metadata that match Greater Than Equal
type GTEElementMatcher struct {
	value interface{}
}

// Gte predicate
func Gte(s interface{}) *GTEElementMatcher {
	return &GTEElementMatcher{value: s}
}

// InsideElementMatcher describes a list of metadata that match inside the range from, to
type InsideElementMatcher struct {
	from interface{}
	to   interface{}
}

// Inside predicate
func Inside(from, to interface{}) *InsideElementMatcher {
	return &InsideElementMatcher{from: from, to: to}
}

// OutsideElementMatcher describes a list of metadata that match outside the range from, to
type OutsideElementMatcher struct {
	from interface{}
	to   interface{}
}

// Outside predicate
func Outside(from, to interface{}) *OutsideElementMatcher {
	return &OutsideElementMatcher{from: from, to: to}
}

// BetweenElementMatcher describes a list of metadata that match between the range from, to
type BetweenElementMatcher struct {
	from interface{}
	to   interface{}
}

// Between predicate
func Between(from interface{}, to interface{}) *BetweenElementMatcher {
	return &BetweenElementMatcher{from: from, to: to}
}

// RegexElementMatcher describes a list of metadata that match a regex
type RegexElementMatcher struct {
	regex string
}

// Regex predicate
func Regex(regex string) *RegexElementMatcher {
	return &RegexElementMatcher{regex: regex}
}

// IPV4RangeElementMatcher matches ipv4 contained in an ipv4 range
type IPV4RangeElementMatcher struct {
	value interface{}
}

// IPV4Range predicate
func IPV4Range(s interface{}) *IPV4RangeElementMatcher {
	return &IPV4RangeElementMatcher{value: s}
}

// Since describes a list of metadata that match since seconds
type Since struct {
	Seconds int64
}

// NewGraphTraversal creates a new graph traversal
func NewGraphTraversal(g *graph.Graph, lockGraph bool) *GraphTraversal {
	return &GraphTraversal{
		Graph:     g,
		lockGraph: lockGraph,
		as:        make(map[string]*GraphTraversalAs),
	}
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

func (t *GraphTraversal) getPaginationRange(ctx *StepContext) (filter *filters.Range) {
	if ctx.PaginationRange != nil {
		filter = &filters.Range{
			From: ctx.PaginationRange[0],
			To:   ctx.PaginationRange[1],
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

	t.RLock()
	defer t.RUnlock()

	g, err := t.Graph.CloneWithContext(graph.Context{
		TimePoint: len(s) == 1,
		TimeSlice: common.NewTimeSlice(common.UnixMillis(at.Add(-duration)), common.UnixMillis(at)),
	})
	if err != nil {
		return &GraphTraversal{error: err}
	}

	return &GraphTraversal{Graph: g}
}

// V step : [node ID]
func (t *GraphTraversal) V(ctx StepContext, s ...interface{}) *GraphTraversalV {
	var nodes []*graph.Node
	var matcher graph.ElementMatcher
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
			return &GraphTraversalV{error: errors.New("V accepts only a string when there is only one argument")}
		}
		node := t.Graph.GetNode(graph.Identifier(id))
		if node == nil {
			return &GraphTraversalV{error: fmt.Errorf("Node '%s' does not exist", id)}
		}
		nodes = []*graph.Node{node}
	default:
		if matcher, err = ParamsToMetadataFilter(s...); err != nil {
			return &GraphTraversalV{error: err}
		}
		fallthrough
	case 0:
		nodes = t.Graph.GetNodes(matcher)
	}

	if ctx.PaginationRange != nil {
		var nodeRange []*graph.Node
		it := ctx.PaginationRange.Iterator()
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
func (tv *GraphTraversalV) PropertyValues(ctx StepContext, k ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return NewGraphTraversalValueFromError(tv.error)
	}

	key := k[0].(string)

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	var s []interface{}
	for _, n := range tv.nodes {
		if value, err := n.GetField(key); err == nil {
			v := reflect.ValueOf(value)
			switch v.Kind() {
			case reflect.Map, reflect.Array, reflect.Slice:
				if v.Len() > 0 {
					s = append(s, value)
				}
			default:
				s = append(s, value)
			}
		}
	}
	return NewGraphTraversalValue(tv.GraphTraversal, s)
}

// PropertyKeys returns at this step, all the metadata keys of each metadata
func (tv *GraphTraversalV) PropertyKeys(ctx StepContext, keys ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return NewGraphTraversalValueFromError(tv.error)
	}

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	var s []interface{}

	seen := make(map[string]bool)
	for _, n := range tv.nodes {
		fields, err := n.GetFields()
		if err != nil {
			return NewGraphTraversalValueFromError(err)
		}
		for _, k := range fields {
			if _, ok := seen[k]; !ok {
				s = append(s, k)
				seen[k] = true
			}
		}
	}

	return NewGraphTraversalValue(tv.GraphTraversal, s)
}

// Sum step : key
// returns the sum of the metadata values of the first argument key
func (tv *GraphTraversalV) Sum(ctx StepContext, keys ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return NewGraphTraversalValueFromError(tv.error)
	}

	if len(keys) != 1 {
		return NewGraphTraversalValueFromError(errors.New("Sum requires 1 parameter"))
	}
	key, ok := keys[0].(string)
	if !ok {
		return NewGraphTraversalValueFromError(errors.New("Sum parameter has to be a string key"))
	}

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	var s float64
	for _, n := range tv.nodes {
		if value, err := n.GetFieldInt64(key); err == nil {
			if v, err := common.ToFloat64(value); err == nil {
				s += v
			} else {
				return NewGraphTraversalValueFromError(err)
			}
		} else {
			if err != common.ErrFieldNotFound {
				return NewGraphTraversalValueFromError(err)
			}
		}
	}
	return NewGraphTraversalValue(tv.GraphTraversal, s)
}

// As stores the result of the previous step using the given key
func (tv *GraphTraversalV) As(ctx StepContext, keys ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	if len(keys) != 1 {
		return &GraphTraversalV{error: errors.New("As parameter have to be a string key")}
	}
	key, ok := keys[0].(string)
	if !ok {
		return &GraphTraversalV{error: errors.New("As parameter have to be a string key")}
	}

	tv.GraphTraversal.as[key] = &GraphTraversalAs{nodes: tv.nodes}

	return tv
}

// G returns the GraphTraversal
func (tv *GraphTraversalV) G() *GraphTraversal {
	return tv.GraphTraversal
}

// Select implements the SELECT gremlin step
func (tv *GraphTraversalV) Select(ctx StepContext, keys ...interface{}) *GraphTraversalV {
	if len(keys) == 0 {
		return &GraphTraversalV{error: errors.New("Select requires at least one key")}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	for _, k := range keys {
		key, ok := k.(string)
		if !ok {
			return &GraphTraversalV{error: errors.New("Select accepts only string parameters")}
		}

		as, ok := tv.GraphTraversal.as[key]
		if !ok {
			return &GraphTraversalV{error: fmt.Errorf("Key %s not registered. Need to be registered using 'As' step", key)}
		}

		ntv.nodes = append(ntv.nodes, as.nodes...)
	}

	return ntv
}

// E step : [edge ID]
func (t *GraphTraversal) E(ctx StepContext, s ...interface{}) *GraphTraversalE {
	var edges []*graph.Edge
	var matcher graph.ElementMatcher
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
			return &GraphTraversalE{error: errors.New("E accepts only a string when there is only one argument")}
		}
		edge := t.Graph.GetEdge(graph.Identifier(id))
		if edge == nil {
			return &GraphTraversalE{error: fmt.Errorf("Edge '%s' does not exist", id)}
		}
		edges = []*graph.Edge{edge}
	default:
		if matcher, err = ParamsToMetadataFilter(s...); err != nil {
			return &GraphTraversalE{error: err}
		}
		fallthrough
	case 0:
		edges = t.Graph.GetEdges(matcher)
	}

	if ctx.PaginationRange != nil {
		var edgeRange []*graph.Edge
		it := ctx.PaginationRange.Iterator()
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

// G returns the GraphTraversal
func (te *GraphTraversalE) G() *GraphTraversal {
	return te.GraphTraversal
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
			return order, sortBy, errors.New("Sort parameters have to be SortOrder(ASC/DESC) and a sort string Key")
		}
	default:
		return order, sortBy, errors.New("Sort accepts up to 2 parameters only")
	}

	return order, sortBy, err
}

// Sort step
func (tv *GraphTraversalV) Sort(ctx StepContext, keys ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	sortOrder, sortBy, err := ParseSortParameter(keys...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	if sortBy == "" {
		sortBy = defaultSortBy
	}

	graph.SortNodes(tv.nodes, sortBy, sortOrder)

	return tv
}

// Dedup step : deduplicate output
func (tv *GraphTraversalV) Dedup(ctx StepContext, s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	var keys []string
	if len(s) > 0 {
		for _, key := range s {
			k, ok := key.(string)
			if !ok {
				return &GraphTraversalV{error: errors.New("Dedup parameters have to be string keys")}
			}
			keys = append(keys, k)
		}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := ctx.PaginationRange.Iterator()

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
func (tv *GraphTraversalV) ShortestPathTo(ctx StepContext, m graph.Metadata, e graph.Metadata) *GraphTraversalShortestPath {
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
func (tv *GraphTraversalV) Has(ctx StepContext, s ...interface{}) *GraphTraversalV {
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
		filter = filters.NewNotNullFilter(k)
	default:
		filter, err = ParamsToFilter(s...)
		if err != nil {
			return &GraphTraversalV{error: err}
		}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := ctx.PaginationRange.Iterator()

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
func (tv *GraphTraversalV) HasKey(ctx StepContext, s string) *GraphTraversalV {
	return tv.Has(ctx, s)
}

// HasNot step
func (tv *GraphTraversalV) HasNot(ctx StepContext, s string) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	filter := filters.NewNullFilter(s)
	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := ctx.PaginationRange.Iterator()

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
func (tv *GraphTraversalV) Both(ctx StepContext, s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	metadata, err := ParamsToMetadataFilter(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := ctx.PaginationRange.Iterator()

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
func (tv *GraphTraversalV) Count(ctx StepContext, s ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return NewGraphTraversalValueFromError(tv.error)
	}

	return NewGraphTraversalValue(tv.GraphTraversal, len(tv.nodes))
}

// Range step
func (tv *GraphTraversalV) Range(ctx StepContext, s ...interface{}) *GraphTraversalV {
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
func (tv *GraphTraversalV) Limit(ctx StepContext, s ...interface{}) *GraphTraversalV {
	return tv.Range(ctx, int64(0), s[0])
}

// Out step : out of a node step
func (tv *GraphTraversalV) Out(ctx StepContext, s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	metadata, err := ParamsToMetadataFilter(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := ctx.PaginationRange.Iterator()

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
func (tv *GraphTraversalV) OutE(ctx StepContext, s ...interface{}) *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{error: tv.error}
	}

	metadata, err := ParamsToMetadataFilter(s...)
	if err != nil {
		return &GraphTraversalE{error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: tv.GraphTraversal, edges: []*graph.Edge{}}
	it := ctx.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n, metadata) {
			if e.GetParent() == n.ID {
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

// BothE : both are edges
func (tv *GraphTraversalV) BothE(ctx StepContext, s ...interface{}) *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{error: tv.error}
	}

	metadata, err := ParamsToMetadataFilter(s...)
	if err != nil {
		return &GraphTraversalE{GraphTraversal: tv.GraphTraversal, error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: tv.GraphTraversal, edges: []*graph.Edge{}}
	it := ctx.PaginationRange.Iterator()

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
func (tv *GraphTraversalV) In(ctx StepContext, s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	metadata, err := ParamsToMetadataFilter(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := ctx.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.nodes {
		for _, parent := range tv.GraphTraversal.Graph.LookupParents(n, metadata, nil) {
			if it.Done() {
				break nodeloop
			} else if it.Next() {
				ntv.nodes = append(ntv.nodes, parent)
			}
		}
	}

	return ntv
}

// InE step of an node
func (tv *GraphTraversalV) InE(ctx StepContext, s ...interface{}) *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{error: tv.error}
	}

	metadata, err := ParamsToMetadataFilter(s...)
	if err != nil {
		return &GraphTraversalE{GraphTraversal: tv.GraphTraversal, error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: tv.GraphTraversal, edges: []*graph.Edge{}}
	it := ctx.PaginationRange.Iterator()

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
func (tv *GraphTraversalV) SubGraph(ctx StepContext, s ...interface{}) *GraphTraversal {
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

	ng := graph.NewGraph(tv.GraphTraversal.Graph.GetHost(), memory, common.UnknownService)

	return NewGraphTraversal(ng, tv.GraphTraversal.lockGraph)
}

// SubGraph step, node/edge out
func (sp *GraphTraversalShortestPath) SubGraph(ctx StepContext, s ...interface{}) *GraphTraversal {
	if sp.error != nil {
		return &GraphTraversal{error: sp.error}
	}

	sp.GraphTraversal.RLock()
	defer sp.GraphTraversal.RUnlock()

	memory, err := graph.NewMemoryBackend()
	if err != nil {
		return &GraphTraversal{error: err}
	}

	// first insert all the nodes
	for _, p := range sp.paths {
		for _, n := range p {
			if !memory.NodeAdded(n) {
				return &GraphTraversal{error: errors.New("Error while adding node to SubGraph")}
			}
		}
	}
	for _, p := range sp.paths {
		for _, n := range p {
			edges := sp.GraphTraversal.Graph.GetNodeEdges(n, nil)
			for _, e := range edges {
				memory.EdgeAdded(e)
			}
		}
	}

	ng := graph.NewGraph(sp.GraphTraversal.Graph.GetHost(), memory, common.UnknownService)

	return NewGraphTraversal(ng, sp.GraphTraversal.lockGraph)
}

// Count step
func (te *GraphTraversalE) Count(ctx StepContext, s ...interface{}) *GraphTraversalValue {
	if te.error != nil {
		return NewGraphTraversalValueFromError(te.error)
	}

	return NewGraphTraversalValue(te.GraphTraversal, len(te.edges))
}

// Range step
func (te *GraphTraversalE) Range(ctx StepContext, s ...interface{}) *GraphTraversalE {
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
func (te *GraphTraversalE) Limit(ctx StepContext, s ...interface{}) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	return te.Range(ctx, int64(0), s[0])
}

// Dedup step : deduplicate
func (te *GraphTraversalE) Dedup(ctx StepContext, keys ...interface{}) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	var key string
	if len(keys) > 0 {
		k, ok := keys[0].(string)
		if !ok {
			return &GraphTraversalE{error: errors.New("Dedup parameter has to be a string key")}
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
func (te *GraphTraversalE) Has(ctx StepContext, s ...interface{}) *GraphTraversalE {
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
		filter = filters.NewNotNullFilter(k)
	default:
		filter, err = ParamsToFilter(s...)
		if err != nil {
			return &GraphTraversalE{error: err}
		}
	}

	nte := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*graph.Edge{}}
	it := ctx.PaginationRange.Iterator()

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
func (te *GraphTraversalE) HasKey(ctx StepContext, s string) *GraphTraversalE {
	return te.Has(ctx, s)
}

// HasNot step
func (te *GraphTraversalE) HasNot(ctx StepContext, s string) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	filter := filters.NewNullFilter(s)
	nte := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*graph.Edge{}}
	it := ctx.PaginationRange.Iterator()

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
func (te *GraphTraversalE) InV(ctx StepContext, s ...interface{}) *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{error: te.error}
	}

	metadata, err := ParamsToMetadataFilter(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: te.GraphTraversal, nodes: []*graph.Node{}}
	it := ctx.PaginationRange.Iterator()

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
func (te *GraphTraversalE) OutV(ctx StepContext, s ...interface{}) *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{error: te.error}
	}

	metadata, err := ParamsToMetadataFilter(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: te.GraphTraversal, nodes: []*graph.Node{}}
	it := ctx.PaginationRange.Iterator()

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
func (te *GraphTraversalE) BothV(ctx StepContext, s ...interface{}) *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{error: te.error}
	}

	metadata, err := ParamsToMetadataFilter(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := NewGraphTraversalV(te.GraphTraversal, []*graph.Node{})
	it := ctx.PaginationRange.Iterator()

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

// SubGraph step, node/edge out
func (te *GraphTraversalE) SubGraph(ctx StepContext, s ...interface{}) *GraphTraversal {
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

	ng := graph.NewGraph(te.GraphTraversal.Graph.GetHost(), memory, common.UnknownService)

	return NewGraphTraversal(ng, te.GraphTraversal.lockGraph)
}

// NewGraphTraversalValue creates a new traversal value step
func NewGraphTraversalValue(gt *GraphTraversal, value interface{}) *GraphTraversalValue {
	tv := &GraphTraversalValue{
		GraphTraversal: gt,
		value:          value,
	}

	return tv
}

// NewGraphTraversalValueFromError creates a new traversal value step
func NewGraphTraversalValueFromError(err ...error) *GraphTraversalValue {
	tv := &GraphTraversalValue{}

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
func (t *GraphTraversalValue) Dedup(ctx StepContext, keys ...interface{}) *GraphTraversalValue {
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
