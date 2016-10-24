/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"time"
)

type MemoryBackendNode struct {
	*Node
	edges map[Identifier]*MemoryBackendEdge
}

type MemoryBackendEdge struct {
	*Edge
}

type MemoryBackend struct {
	nodes map[Identifier]*MemoryBackendNode
	edges map[Identifier]*MemoryBackendEdge
}

func (m MemoryBackend) SetMetadata(i interface{}, meta Metadata) bool {
	return true
}

func (m MemoryBackend) AddMetadata(i interface{}, k string, v interface{}) bool {
	return true
}

func (m MemoryBackend) AddEdge(e *Edge) bool {
	edge := &MemoryBackendEdge{
		Edge: e,
	}

	parent, ok := m.nodes[e.parent]
	if !ok {
		return false
	}

	child, ok := m.nodes[e.child]
	if !ok {
		return false
	}

	m.edges[e.ID] = edge
	parent.edges[e.ID] = edge
	child.edges[e.ID] = edge

	return true
}

func (m MemoryBackend) GetEdge(i Identifier, t *time.Time) *Edge {
	if e, ok := m.edges[i]; ok {
		return e.Edge
	}
	return nil
}

func (m MemoryBackend) GetEdgeNodes(e *Edge, t *time.Time) (*Node, *Node) {
	var parent *MemoryBackendNode
	if e, ok := m.edges[e.ID]; ok {
		if n, ok := m.nodes[e.parent]; ok {
			parent = n
		}
	}

	var child *MemoryBackendNode
	if e, ok := m.edges[e.ID]; ok {
		if n, ok := m.nodes[e.child]; ok {
			child = n
		}
	}

	if parent == nil || child == nil {
		return nil, nil
	}

	return parent.Node, child.Node
}

func (m MemoryBackend) AddNode(n *Node) bool {
	m.nodes[n.ID] = &MemoryBackendNode{
		Node:  n,
		edges: make(map[Identifier]*MemoryBackendEdge),
	}

	return true
}

func (m MemoryBackend) GetNode(i Identifier, t *time.Time) *Node {
	if n, ok := m.nodes[i]; ok {
		return n.Node
	}
	return nil
}

func (m MemoryBackend) GetNodeEdges(n *Node, t *time.Time) []*Edge {
	edges := []*Edge{}

	if n, ok := m.nodes[n.ID]; ok {
		for _, e := range n.edges {
			edges = append(edges, e.Edge)
		}
	}

	return edges
}

func (m MemoryBackend) DelEdge(e *Edge) bool {
	if _, ok := m.edges[e.ID]; !ok {
		return false
	}

	if parent, ok := m.nodes[e.parent]; ok {
		delete(parent.edges, e.ID)
	}

	if child, ok := m.nodes[e.child]; ok {
		delete(child.edges, e.ID)
	}

	delete(m.edges, e.ID)

	return true
}

func (m MemoryBackend) DelNode(n *Node) bool {
	delete(m.nodes, n.ID)

	return true
}

func (m MemoryBackend) GetNodes(t *time.Time, metadata Metadata) []*Node {
	nodes := []*Node{}

	for _, n := range m.nodes {
		if n.MatchMetadata(metadata) {
			nodes = append(nodes, n.Node)
		}
	}

	return nodes
}

func (m MemoryBackend) GetEdges(t *time.Time, metadata Metadata) []*Edge {
	edges := []*Edge{}

	for _, e := range m.edges {
		if e.MatchMetadata(metadata) {
			edges = append(edges, e.Edge)
		}
	}

	return edges
}

func (m MemoryBackend) WithContext(graph *Graph, context GraphContext) (*Graph, error) {
	if context.Time != nil {
		return nil, errors.New("Memory backend does not support history")
	}
	return graph, nil
}

func NewMemoryBackend() (*MemoryBackend, error) {
	return &MemoryBackend{
		nodes: make(map[Identifier]*MemoryBackendNode),
		edges: make(map[Identifier]*MemoryBackendEdge),
	}, nil
}
