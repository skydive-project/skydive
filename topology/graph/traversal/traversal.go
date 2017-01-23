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
	"regexp"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/topology/graph"
)

type GraphTraversalStep interface {
	Values() []interface{}
	MarshalJSON() ([]byte, error)
	Error() error
}

type GraphStepContext struct {
	PaginationRange *GraphTraversalRange
}

func (r *GraphTraversalRange) Iterator() *common.Iterator {
	if r != nil {
		return common.NewIterator(0, r[0], r[1])
	}
	return common.NewIterator()
}

type GraphTraversalRange [2]int64

type GraphTraversal struct {
	Graph              *graph.Graph
	error              error
	currentStepContext GraphStepContext
}

type GraphTraversalV struct {
	GraphTraversal *GraphTraversal
	nodes          []*graph.Node
	error          error
}

type GraphTraversalE struct {
	GraphTraversal *GraphTraversal
	edges          []*graph.Edge
	error          error
}

type GraphTraversalShortestPath struct {
	GraphTraversal *GraphTraversal
	paths          [][]*graph.Node
	error          error
}

type GraphTraversalValue struct {
	GraphTraversal *GraphTraversal
	value          interface{}
	error          error
}

type WithinMetadataMatcher struct {
	List []interface{}
}

func (w *WithinMetadataMatcher) Match(v interface{}) bool {
	for _, el := range w.List {
		if common.CrossTypeEqual(v, el) {
			return true
		}
	}

	return false
}

func Within(s ...interface{}) *WithinMetadataMatcher {
	return &WithinMetadataMatcher{List: s}
}

type WithoutMetadataMatcher struct {
	list []interface{}
}

func (w *WithoutMetadataMatcher) Match(v interface{}) bool {
	for _, el := range w.list {
		if common.CrossTypeEqual(v, el) {
			return false
		}
	}

	return true
}

func Without(s ...interface{}) *WithoutMetadataMatcher {
	return &WithoutMetadataMatcher{list: s}
}

type NEMetadataMatcher struct {
	value interface{}
}

func (n *NEMetadataMatcher) Match(v interface{}) bool {
	if !common.CrossTypeEqual(v, n.value) {
		return true
	}

	return false
}

func (n *NEMetadataMatcher) Value() interface{} {
	return n.value
}

func Ne(s interface{}) *NEMetadataMatcher {
	return &NEMetadataMatcher{value: s}
}

type LTMetadataMatcher struct {
	value interface{}
}

func (lt *LTMetadataMatcher) Match(v interface{}) bool {
	if result, err := common.CrossTypeCompare(v, lt.value); err == nil && result == -1 {
		return true
	}

	return false
}

func (lt *LTMetadataMatcher) Value() interface{} {
	return lt.value
}

func Lt(s interface{}) *LTMetadataMatcher {
	return &LTMetadataMatcher{value: s}
}

type GTMetadataMatcher struct {
	value interface{}
}

func (gt *GTMetadataMatcher) Match(v interface{}) bool {
	if result, err := common.CrossTypeCompare(v, gt.value); err == nil && result == 1 {
		return true
	}

	return false
}

func (gt *GTMetadataMatcher) Value() interface{} {
	return gt.value
}

func Gt(s interface{}) *GTMetadataMatcher {
	return &GTMetadataMatcher{value: s}
}

type LTEMetadataMatcher struct {
	value interface{}
}

func (lte *LTEMetadataMatcher) Match(v interface{}) bool {
	if result, err := common.CrossTypeCompare(v, lte.value); err == nil && result <= 0 {
		return true
	}

	return false
}

func (lte *LTEMetadataMatcher) Value() interface{} {
	return lte.value
}

func Lte(s interface{}) *LTEMetadataMatcher {
	return &LTEMetadataMatcher{value: s}
}

type GTEMetadataMatcher struct {
	value interface{}
}

func (gte *GTEMetadataMatcher) Match(v interface{}) bool {
	if result, err := common.CrossTypeCompare(v, gte.value); err == nil && result >= 0 {
		return true
	}

	return false
}

func (gte *GTEMetadataMatcher) Value() interface{} {
	return gte.value
}

func Gte(s interface{}) *GTEMetadataMatcher {
	return &GTEMetadataMatcher{value: s}
}

type InsideMetadataMatcher struct {
	from interface{}
	to   interface{}
}

func (i *InsideMetadataMatcher) Match(v interface{}) bool {
	result, err := common.CrossTypeCompare(v, i.from)
	result2, err2 := common.CrossTypeCompare(v, i.to)

	if err == nil && err2 == nil && result == 1 && result2 == -1 {
		return true
	}

	return false
}

func (i *InsideMetadataMatcher) Value() (interface{}, interface{}) {
	return i.from, i.to
}

func Inside(from, to interface{}) *InsideMetadataMatcher {
	return &InsideMetadataMatcher{from: from, to: to}
}

type OutsideMetadataMatcher struct {
	from interface{}
	to   interface{}
}

func (i *OutsideMetadataMatcher) Match(v interface{}) bool {
	result, err := common.CrossTypeCompare(v, i.from)
	result2, err2 := common.CrossTypeCompare(v, i.to)

	if err == nil && err2 == nil && result == -1 && result2 == 1 {
		return true
	}

	return false
}

func (i *OutsideMetadataMatcher) Value() (interface{}, interface{}) {
	return i.from, i.to
}

func Outside(from, to interface{}) *OutsideMetadataMatcher {
	return &OutsideMetadataMatcher{from: from, to: to}
}

type BetweenMetadataMatcher struct {
	from interface{}
	to   interface{}
}

func (b *BetweenMetadataMatcher) Match(v interface{}) bool {
	result, err := common.CrossTypeCompare(v, b.from)
	result2, err2 := common.CrossTypeCompare(v, b.to)

	if err == nil && err2 == nil && result >= 0 && result2 == -1 {
		return true
	}

	return false
}

func (b *BetweenMetadataMatcher) Value() (interface{}, interface{}) {
	return b.from, b.to
}

func Between(from interface{}, to interface{}) *BetweenMetadataMatcher {
	return &BetweenMetadataMatcher{from: from, to: to}
}

type RegexMetadataMatcher struct {
	regexp  *regexp.Regexp
	pattern string
}

func (r *RegexMetadataMatcher) Value() interface{} {
	return r.pattern
}

func (r *RegexMetadataMatcher) Match(v interface{}) bool {
	if r.regexp == nil {
		return false
	}

	switch v.(type) {
	case string:
		return r.regexp.MatchString(v.(string))
	}

	return false
}

func Regex(expr string) *RegexMetadataMatcher {
	r, _ := regexp.Compile(expr)
	return &RegexMetadataMatcher{regexp: r, pattern: expr}
}

type Since struct {
	Seconds int64
}

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

		m[k] = s[i+1]
	}

	return m, nil
}

func NewGraphTraversal(g *graph.Graph) *GraphTraversal {
	return &GraphTraversal{Graph: g}
}

func (t *GraphTraversal) Values() []interface{} {
	return []interface{}{t.Graph}
}

func (t *GraphTraversal) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Values())
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

func (t *GraphTraversal) Context(s ...interface{}) *GraphTraversal {
	if t.error != nil {
		return t
	}

	if len(s) != 1 {
		return &GraphTraversal{error: errors.New("At least one parameter must be provided")}
	}

	var (
		at  time.Time
		err error
	)
	switch param := s[0].(type) {
	case string:
		if at, err = parseTimeContext(param); err != nil {
			return &GraphTraversal{error: err}
		}
	case int64:
		at = time.Unix(param, 0)
	default:
		return &GraphTraversal{error: errors.New("Key must be either an integer or a string")}
	}

	t.Graph.RLock()
	defer t.Graph.RUnlock()

	if at.After(time.Now().UTC()) {
		return &GraphTraversal{error: errors.New("Sorry, I can't predict the future")}
	}

	g, err := t.Graph.WithContext(graph.GraphContext{Time: &at})
	if err != nil {
		return &GraphTraversal{error: err}
	}

	return &GraphTraversal{Graph: g}
}

func (t *GraphTraversal) V(s ...interface{}) *GraphTraversalV {
	var nodes []*graph.Node
	var metadata graph.Metadata
	var err error

	if t.error != nil {
		return &GraphTraversalV{error: t.error}
	}

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

func (tv *GraphTraversalV) Values() []interface{} {
	s := make([]interface{}, len(tv.nodes))
	for i, n := range tv.nodes {
		s[i] = n
	}
	return s
}

func (tv *GraphTraversalV) MarshalJSON() ([]byte, error) {
	return json.Marshal(tv.Values())
}

func (tv *GraphTraversalV) GetNodes() (nodes []*graph.Node) {
	return tv.nodes
}

func (tv *GraphTraversalV) PropertyValues(keys ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return &GraphTraversalValue{error: tv.error}
	}

	key := keys[0].(string)

	var s []interface{}
	for _, n := range tv.nodes {
		if value, ok := n.Metadata()[key]; ok {
			s = append(s, value)
		}
	}
	return &GraphTraversalValue{GraphTraversal: tv.GraphTraversal, value: s}
}

func (tv *GraphTraversalV) PropertyKeys(keys ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return &GraphTraversalValue{error: tv.error}
	}

	var s []interface{}
	for _, n := range tv.nodes {
		for key := range n.Metadata() {
			s = append(s, key)
		}
	}

	return &GraphTraversalValue{GraphTraversal: tv.GraphTraversal, value: s}
}

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

	var s float64
	for _, n := range tv.nodes {
		if value, ok := n.Metadata()[key]; ok {
			if v, err := common.ToFloat64(value); err == nil {
				s += v
			} else {
				return &GraphTraversalValue{error: err}
			}
		}
	}
	return &GraphTraversalValue{GraphTraversal: tv.GraphTraversal, value: s}
}

func (tv *GraphTraversalV) Dedup(keys ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	var key string
	if len(keys) > 0 {
		k, ok := keys[0].(string)
		if !ok {
			return &GraphTraversalV{error: fmt.Errorf("Dedup parameter has to be a string key")}
		}
		key = k
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}

	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()
	visited := make(map[interface{}]bool)

	var kvisited interface{}
	for _, n := range tv.nodes {

		kvisited = n.ID
		if key != "" {
			if v, ok := n.Metadata()[key]; ok {
				kvisited = v
			}
		}

		if it.Done() {
			break
		} else if _, ok := visited[kvisited]; !ok && it.Next() {
			ntv.nodes = append(ntv.nodes, n)
			visited[kvisited] = true
		}
	}

	return ntv
}

func (sp *GraphTraversalShortestPath) Values() []interface{} {
	s := make([]interface{}, len(sp.paths))
	for i, p := range sp.paths {
		s[i] = p
	}
	return s
}

func (sp *GraphTraversalShortestPath) MarshalJSON() ([]byte, error) {
	return json.Marshal(sp.Values())
}

func (sp *GraphTraversalShortestPath) Error() error {
	return sp.error
}

func (tv *GraphTraversalV) ShortestPathTo(m graph.Metadata, e ...graph.Metadata) *GraphTraversalShortestPath {
	if tv.error != nil {
		return &GraphTraversalShortestPath{error: tv.error}
	}
	sp := &GraphTraversalShortestPath{GraphTraversal: tv.GraphTraversal, paths: [][]*graph.Node{}}

	visited := make(map[graph.Identifier]bool)
	for _, n := range tv.nodes {
		if _, ok := visited[n.ID]; !ok {
			path := tv.GraphTraversal.Graph.LookupShortestPath(n, m, e...)
			if len(path) > 0 {
				sp.paths = append(sp.paths, path)
			}
		}
	}
	return sp
}

func (tv *GraphTraversalV) hasKey(k string) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	for _, n := range tv.nodes {
		if _, ok := n.Metadata()[k]; ok {
			ntv.nodes = append(ntv.nodes, n)
		}
	}

	return ntv
}

func (tv *GraphTraversalV) Has(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	switch len(s) {
	case 0:
		return &GraphTraversalV{error: errors.New("At least one parameter must be provided")}
	case 1:
		k, ok := s[0].(string)
		if !ok {
			return &GraphTraversalV{error: errors.New("Key must be a string")}
		}
		return tv.hasKey(k)
	}

	m, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()
	for _, n := range tv.nodes {
		if it.Done() {
			break
		} else if n.MatchMetadata(m) && it.Next() {
			ntv.nodes = append(ntv.nodes, n)
		}
	}

	return ntv
}

func (tv *GraphTraversalV) Both(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	metadata, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{GraphTraversal: tv.GraphTraversal, error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*graph.Node{}}
	it := tv.GraphTraversal.currentStepContext.PaginationRange.Iterator()

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n) {
			if it.Done() {
				break nodeloop
			}

			parent, child := tv.GraphTraversal.Graph.GetEdgeNodes(e)

			if parent != nil && parent.ID == n.ID && child.MatchMetadata(metadata) && it.Next() {
				ntv.nodes = append(ntv.nodes, child)
			}

			if !it.Done() && child != nil && child.ID == n.ID && parent.MatchMetadata(metadata) && it.Next() {
				ntv.nodes = append(ntv.nodes, parent)
			}
		}
	}

	return ntv
}

func (tv *GraphTraversalV) Count(s ...interface{}) *GraphTraversalValue {
	if tv.error != nil {
		return &GraphTraversalValue{error: tv.error}
	}

	return &GraphTraversalValue{GraphTraversal: tv.GraphTraversal, value: len(tv.nodes)}
}

func (tv *GraphTraversalV) Range(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return &GraphTraversalV{error: tv.error}
	}

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

func (tv *GraphTraversalV) Limit(s ...interface{}) *GraphTraversalV {
	return tv.Range(int64(0), s[0])
}

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

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n) {
			if it.Done() {
				break nodeloop
			}

			parent, child := tv.GraphTraversal.Graph.GetEdgeNodes(e)

			if parent != nil && parent.ID == n.ID && child.MatchMetadata(metadata) && it.Next() {
				ntv.nodes = append(ntv.nodes, child)
			}
		}
	}

	return ntv
}

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

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n) {
			if it.Done() {
				break nodeloop
			}

			parent, _ := tv.GraphTraversal.Graph.GetEdgeNodes(e)

			if parent != nil && parent.ID == n.ID && e.MatchMetadata(metadata) && it.Next() {
				nte.edges = append(nte.edges, e)
			}
		}
	}

	return nte
}

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

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n) {
			if it.Done() {
				break nodeloop
			}

			parent, child := tv.GraphTraversal.Graph.GetEdgeNodes(e)

			if child != nil && child.ID == n.ID && parent.MatchMetadata(metadata) && it.Next() {
				ntv.nodes = append(ntv.nodes, parent)
			}
		}
	}

	return ntv
}

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

nodeloop:
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.GetNodeEdges(n) {
			if it.Done() {
				break nodeloop
			}

			_, child := tv.GraphTraversal.Graph.GetEdgeNodes(e)

			if child != nil && child.ID == n.ID && e.MatchMetadata(metadata) && it.Next() {
				nte.edges = append(nte.edges, e)
			}
		}
	}

	return nte
}

func (te *GraphTraversalE) Error() error {
	return te.error
}

func (te *GraphTraversalE) Values() []interface{} {
	s := make([]interface{}, len(te.edges))
	for i, v := range te.edges {
		s[i] = v
	}
	return s
}

func (te *GraphTraversalE) MarshalJSON() ([]byte, error) {
	return json.Marshal(te.Values())
}

func (te *GraphTraversalE) Count(s ...interface{}) *GraphTraversalValue {
	if te.error != nil {
		return &GraphTraversalValue{error: te.error}
	}

	return &GraphTraversalValue{GraphTraversal: te.GraphTraversal, value: len(te.edges)}
}

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

func (te *GraphTraversalE) Limit(s ...interface{}) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	return te.Range(int64(0), s[0])
}

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

func (te *GraphTraversalE) hasKey(k string) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	nte := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*graph.Edge{}}
	it := te.GraphTraversal.currentStepContext.PaginationRange.Iterator()

	for _, e := range te.edges {
		if it.Done() {
			break
		} else if _, ok := e.Metadata()[k]; ok && it.Next() {
			nte.edges = append(nte.edges, e)
		}
	}

	return nte
}

func (te *GraphTraversalE) Has(s ...interface{}) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	switch len(s) {
	case 0:
		return &GraphTraversalE{error: errors.New("At least one parameters must be provided")}
	case 1:
		k, ok := s[0].(string)
		if !ok {
			return &GraphTraversalE{error: errors.New("Key must be a string")}
		}
		return te.hasKey(k)
	}

	m, err := SliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalE{error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*graph.Edge{}}
	it := te.GraphTraversal.currentStepContext.PaginationRange.Iterator()
	for _, e := range te.edges {
		if it.Done() {
			break
		} else if e.MatchMetadata(m) && it.Next() {
			nte.edges = append(nte.edges, e)
		}
	}

	return nte
}

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
	for _, e := range te.edges {
		if it.Done() {
			break
		}
		parent, _ := te.GraphTraversal.Graph.GetEdgeNodes(e)
		if parent != nil && parent.MatchMetadata(metadata) && it.Next() {
			ntv.nodes = append(ntv.nodes, parent)
		}
	}

	return ntv
}

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
	for _, e := range te.edges {
		if it.Done() {
			break
		}
		_, child := te.GraphTraversal.Graph.GetEdgeNodes(e)
		if child != nil && child.MatchMetadata(metadata) && it.Next() {
			ntv.nodes = append(ntv.nodes, child)
		}
	}

	return ntv
}

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

func (t *GraphTraversalValue) Values() []interface{} {
	// Values like all step has to return an array of interface
	// if v is already an array return it otherwise instanciate an new array
	// with the value as first element.
	if v, ok := t.value.([]interface{}); ok {
		return v
	}
	return []interface{}{t.value}
}

func (t *GraphTraversalValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.value)
}

func (t *GraphTraversalValue) Error() error {
	return t.error
}

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
