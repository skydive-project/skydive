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
	Graph *Graph
	nodes []*Node
	error error
}

type GraphTraversalE struct {
	Graph *Graph
	edges []*Edge
	error error
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

func (t *GraphTraversal) V(ids ...Identifier) *GraphTraversalV {
	if len(ids) > 0 {
		node := t.Graph.GetNode(ids[0])
		if node != nil {
			return &GraphTraversalV{Graph: t.Graph, nodes: []*Node{node}}
		}
		return &GraphTraversalV{Graph: t.Graph, nodes: []*Node{}}
	}

	return &GraphTraversalV{Graph: t.Graph, nodes: t.Graph.GetNodes()}
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
	ntv := &GraphTraversalV{Graph: tv.Graph, nodes: []*Node{}}

	visited := make(map[Identifier]bool)
	for _, n := range tv.nodes {
		if _, ok := visited[n.ID]; !ok {
			ntv.nodes = append(ntv.nodes, n)
			visited[n.ID] = true
		}
	}
	return ntv
}

func (tv *GraphTraversalV) hasKey(k string) *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	ntv := &GraphTraversalV{Graph: tv.Graph, nodes: []*Node{}}
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
		return &GraphTraversalV{Graph: tv.Graph, error: errors.New("At least one parameters must be provided")}
	case 1:
		k, ok := s[0].(string)
		if !ok {
			return &GraphTraversalV{Graph: tv.Graph, error: errors.New("Key must be a string")}
		}
		return tv.hasKey(k)
	}

	m, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalV{Graph: tv.Graph, error: err}
	}

	ntv := &GraphTraversalV{Graph: tv.Graph, nodes: []*Node{}}
	for _, n := range tv.nodes {
		if n.matchMetadata(m) {
			ntv.nodes = append(ntv.nodes, n)
		}
	}

	return ntv
}

func (tv *GraphTraversalV) Out() *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	ntv := &GraphTraversalV{Graph: tv.Graph, nodes: []*Node{}}
	for _, n := range tv.nodes {
		for _, e := range tv.Graph.backend.GetNodeEdges(n) {
			parent, child := tv.Graph.backend.GetEdgeNodes(e)

			if parent != nil && parent.ID == n.ID {
				ntv.nodes = append(ntv.nodes, child)
			}
		}
	}

	return ntv
}

func (tv *GraphTraversalV) OutE() *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{Graph: tv.Graph, error: tv.error}
	}

	nte := &GraphTraversalE{Graph: tv.Graph, edges: []*Edge{}}
	for _, n := range tv.nodes {
		for _, e := range tv.Graph.backend.GetNodeEdges(n) {
			parent, _ := tv.Graph.backend.GetEdgeNodes(e)

			if parent != nil && parent.ID == n.ID {
				nte.edges = append(nte.edges, e)
			}
		}
	}

	return nte
}

func (tv *GraphTraversalV) In() *GraphTraversalV {
	if tv.error != nil {
		return tv
	}

	ntv := &GraphTraversalV{Graph: tv.Graph, nodes: []*Node{}}
	for _, n := range tv.nodes {
		for _, e := range tv.Graph.backend.GetNodeEdges(n) {
			parent, child := tv.Graph.backend.GetEdgeNodes(e)

			if child != nil && child.ID == n.ID {
				ntv.nodes = append(ntv.nodes, parent)
			}
		}
	}

	return ntv
}

func (tv *GraphTraversalV) InE() *GraphTraversalE {
	if tv.error != nil {
		return &GraphTraversalE{Graph: tv.Graph, error: tv.error}
	}

	nte := &GraphTraversalE{Graph: tv.Graph, edges: []*Edge{}}
	for _, n := range tv.nodes {
		for _, e := range tv.Graph.backend.GetNodeEdges(n) {
			_, child := tv.Graph.backend.GetEdgeNodes(e)

			if child != nil && child.ID == n.ID {
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
	ntv := &GraphTraversalE{Graph: te.Graph, edges: []*Edge{}}

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

	nte := &GraphTraversalE{Graph: te.Graph, edges: []*Edge{}}
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
		return &GraphTraversalE{Graph: te.Graph, error: errors.New("At least one parameters must be provided")}
	case 1:
		k, ok := s[0].(string)
		if !ok {
			return &GraphTraversalE{Graph: te.Graph, error: errors.New("Key must be a string")}
		}
		return te.hasKey(k)
	}

	m, err := sliceToMetadata(s...)
	if err != nil {
		return &GraphTraversalE{Graph: te.Graph, error: err}
	}

	nte := &GraphTraversalE{Graph: te.Graph, edges: []*Edge{}}
	for _, e := range te.edges {
		if e.matchMetadata(m) {
			nte.edges = append(nte.edges, e)
		}
	}

	return nte
}

func (te *GraphTraversalE) InV() *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{Graph: te.Graph, error: te.error}
	}

	ntv := &GraphTraversalV{Graph: te.Graph, nodes: []*Node{}}
	for _, e := range te.edges {
		parent, _ := te.Graph.backend.GetEdgeNodes(e)
		ntv.nodes = append(ntv.nodes, parent)
	}

	return ntv
}

func (te *GraphTraversalE) OutV() *GraphTraversalV {
	if te.error != nil {
		return &GraphTraversalV{Graph: te.Graph, error: te.error}
	}

	ntv := &GraphTraversalV{Graph: te.Graph, nodes: []*Node{}}
	for _, e := range te.edges {
		_, child := te.Graph.backend.GetEdgeNodes(e)
		ntv.nodes = append(ntv.nodes, child)
	}

	return ntv
}
