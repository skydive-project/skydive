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

package graph

import (
	"errors"
	"fmt"

	"github.com/redhat-cip/skydive/common"
)

type GraphTraversalStep interface {
	Values() []interface{}
	Error() error
}

type GraphTraversal struct {
	Graph *Graph
}

type GraphTraversalV struct {
	GraphTraversal *GraphTraversal
	nodes          []*Node
	error          error
}

type GraphTraversalE struct {
	GraphTraversal *GraphTraversal
	edges          []*Edge
	error          error
}

type GraphTraversalShortestPath struct {
	GraphTraversal *GraphTraversal
	paths          [][]*Node
	error          error
}

type WithinMetadataMatcher struct {
	list []interface{}
}

func (w *WithinMetadataMatcher) Match(v interface{}) bool {
	for _, el := range w.list {
		if common.CrossTypeEqual(v, el) {
			return true
		}
	}

	return false
}

func Within(s ...interface{}) *WithinMetadataMatcher {
	return &WithinMetadataMatcher{list: s}
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

func Ne(s interface{}) *NEMetadataMatcher {
	return &NEMetadataMatcher{value: s}
}

func sliceToMetadata(s ...interface{}) (Metadata, error) {
	m := Metadata{}
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

func NewGrahTraversal(g *Graph) *GraphTraversal {
	return &GraphTraversal{Graph: g}
}

func (t *GraphTraversal) Values() []interface{} {
	return []interface{}{t.Graph}
}

func (t *GraphTraversal) Error() error {
	return nil
}

func (t *GraphTraversal) V(ids ...Identifier) *GraphTraversalV {
	if len(ids) > 0 {
		node := t.Graph.GetNode(ids[0])
		if node != nil {
			return &GraphTraversalV{GraphTraversal: t, nodes: []*Node{node}}
		}
		return &GraphTraversalV{GraphTraversal: t, nodes: []*Node{}}
	}

	return &GraphTraversalV{GraphTraversal: t, nodes: t.Graph.GetNodes()}
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

func (tv *GraphTraversalV) Dedup() *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*Node{}}

	visited := make(map[Identifier]bool)
	for _, n := range tv.nodes {
		if _, ok := visited[n.ID]; !ok {
			ntv.nodes = append(ntv.nodes, n)
			visited[n.ID] = true
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

func (sp *GraphTraversalShortestPath) Error() error {
	return sp.error
}

func (tv *GraphTraversalV) ShortestPathTo(m Metadata, e ...Metadata) *GraphTraversalShortestPath {
	if tv.error != nil {
		return &GraphTraversalShortestPath{GraphTraversal: tv.GraphTraversal, paths: [][]*Node{}, error: tv.error}
	}
	sp := &GraphTraversalShortestPath{GraphTraversal: tv.GraphTraversal, paths: [][]*Node{}}

	visited := make(map[Identifier]bool)
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

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*Node{}}
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
		return &GraphTraversalV{GraphTraversal: tv.GraphTraversal, error: errors.New("At least one parameters must be provided")}
	case 1:
		k, ok := s[0].(string)
		if !ok {
			return &GraphTraversalV{GraphTraversal: tv.GraphTraversal, error: errors.New("Key must be a string")}
		}
		return tv.hasKey(k)
	}

	m, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{GraphTraversal: tv.GraphTraversal, error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*Node{}}
	for _, n := range tv.nodes {
		if n.matchMetadata(m) {
			ntv.nodes = append(ntv.nodes, n)
		}
	}

	return ntv
}

func (tv *GraphTraversalV) Both(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	metadata, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{GraphTraversal: tv.GraphTraversal, error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*Node{}}
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.backend.GetNodeEdges(n) {
			parent, child := tv.GraphTraversal.Graph.backend.GetEdgeNodes(e)

			if parent != nil && parent.ID == n.ID && child.matchMetadata(metadata) {
				ntv.nodes = append(ntv.nodes, child)
			}

			if child != nil && child.ID == n.ID && parent.matchMetadata(metadata) {
				ntv.nodes = append(ntv.nodes, parent)
			}
		}
	}

	return ntv
}

func (tv *GraphTraversalV) Out(s ...interface{}) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	metadata, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{GraphTraversal: tv.GraphTraversal, error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*Node{}}
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.backend.GetNodeEdges(n) {
			parent, child := tv.GraphTraversal.Graph.backend.GetEdgeNodes(e)

			if parent != nil && parent.ID == n.ID && child.matchMetadata(metadata) {
				ntv.nodes = append(ntv.nodes, child)
			}
		}
	}

	return ntv
}

func (tv *GraphTraversalV) OutE(s ...interface{}) *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{GraphTraversal: tv.GraphTraversal, error: tv.error}
	}

	metadata, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalE{GraphTraversal: tv.GraphTraversal, error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: tv.GraphTraversal, edges: []*Edge{}}
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.backend.GetNodeEdges(n) {
			parent, _ := tv.GraphTraversal.Graph.backend.GetEdgeNodes(e)

			if parent != nil && parent.ID == n.ID && e.matchMetadata(metadata) {
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

	metadata, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{GraphTraversal: tv.GraphTraversal, error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: tv.GraphTraversal, nodes: []*Node{}}
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.backend.GetNodeEdges(n) {
			parent, child := tv.GraphTraversal.Graph.backend.GetEdgeNodes(e)

			if child != nil && child.ID == n.ID && parent.matchMetadata(metadata) {
				ntv.nodes = append(ntv.nodes, parent)
			}
		}
	}

	return ntv
}

func (tv *GraphTraversalV) InE(s ...interface{}) *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{GraphTraversal: tv.GraphTraversal, error: tv.error}
	}

	metadata, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalE{GraphTraversal: tv.GraphTraversal, error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: tv.GraphTraversal, edges: []*Edge{}}
	for _, n := range tv.nodes {
		for _, e := range tv.GraphTraversal.Graph.backend.GetNodeEdges(n) {
			_, child := tv.GraphTraversal.Graph.backend.GetEdgeNodes(e)

			if child != nil && child.ID == n.ID && e.matchMetadata(metadata) {
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

func (te *GraphTraversalE) Dedup() *GraphTraversalE {
	ntv := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*Edge{}}

	visited := make(map[Identifier]bool)
	for _, e := range te.edges {
		if _, ok := visited[e.ID]; !ok {
			ntv.edges = append(ntv.edges, e)
			visited[e.ID] = true
		}
	}
	return ntv
}

func (te *GraphTraversalE) hasKey(k string) *GraphTraversalE {
	if te.error != nil {
		return te
	}

	nte := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*Edge{}}
	for _, e := range te.edges {
		if _, ok := e.Metadata()[k]; ok {
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
		return &GraphTraversalE{GraphTraversal: te.GraphTraversal, error: errors.New("At least one parameters must be provided")}
	case 1:
		k, ok := s[0].(string)
		if !ok {
			return &GraphTraversalE{GraphTraversal: te.GraphTraversal, error: errors.New("Key must be a string")}
		}
		return te.hasKey(k)
	}

	m, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalE{GraphTraversal: te.GraphTraversal, error: err}
	}

	nte := &GraphTraversalE{GraphTraversal: te.GraphTraversal, edges: []*Edge{}}
	for _, e := range te.edges {
		if e.matchMetadata(m) {
			nte.edges = append(nte.edges, e)
		}
	}

	return nte
}

func (te *GraphTraversalE) InV(s ...interface{}) *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{GraphTraversal: te.GraphTraversal, error: te.error}
	}

	metadata, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{GraphTraversal: te.GraphTraversal, error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: te.GraphTraversal, nodes: []*Node{}}
	for _, e := range te.edges {
		parent, _ := te.GraphTraversal.Graph.backend.GetEdgeNodes(e)
		if parent != nil && parent.matchMetadata(metadata) {
			ntv.nodes = append(ntv.nodes, parent)
		}
	}

	return ntv
}

func (te *GraphTraversalE) OutV(s ...interface{}) *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{GraphTraversal: te.GraphTraversal, error: te.error}
	}

	metadata, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{GraphTraversal: te.GraphTraversal, error: err}
	}

	ntv := &GraphTraversalV{GraphTraversal: te.GraphTraversal, nodes: []*Node{}}
	for _, e := range te.edges {
		_, child := te.GraphTraversal.Graph.backend.GetEdgeNodes(e)
		if child != nil && child.matchMetadata(metadata) {
			ntv.nodes = append(ntv.nodes, child)
		}
	}

	return ntv
}
