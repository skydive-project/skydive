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

// MemoryBackendNode a memory backend node
type MemoryBackendNode struct {
	*Node
	edges map[Identifier]*MemoryBackendEdge
}

// MemoryBackendEdge a memory backend edge
type MemoryBackendEdge struct {
	*Edge
}

// MemoryBackend describes the memory backend
type MemoryBackend struct {
	Backend
	nodes map[Identifier]*MemoryBackendNode
	edges map[Identifier]*MemoryBackendEdge
}

// MetadataUpdated returns true
func (m *MemoryBackend) MetadataUpdated(i interface{}) bool {
	return true
}

// EdgeAdded event add an edge in the memory backend
func (m *MemoryBackend) EdgeAdded(e *Edge) bool {
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

// GetEdge in the graph backend
func (m *MemoryBackend) GetEdge(i Identifier, t Context) []*Edge {
	if e, ok := m.edges[i]; ok {
		return []*Edge{e.Edge}
	}
	return nil
}

// GetEdgeNodes returns a list of nodes of an edge
func (m *MemoryBackend) GetEdgeNodes(e *Edge, t Context, parentMetadata, childMetadata ElementMatcher) ([]*Node, []*Node) {
	var parent *MemoryBackendNode
	if n, ok := m.nodes[e.parent]; ok && n.MatchMetadata(parentMetadata) {
		parent = n
	}

	var child *MemoryBackendNode
	if n, ok := m.nodes[e.child]; ok && n.MatchMetadata(childMetadata) {
		child = n
	}

	if parent == nil || child == nil {
		return nil, nil
	}

	return []*Node{parent.Node}, []*Node{child.Node}
}

// NodeAdded in the graph backend
func (m *MemoryBackend) NodeAdded(n *Node) bool {
	m.nodes[n.ID] = &MemoryBackendNode{
		Node:  n,
		edges: make(map[Identifier]*MemoryBackendEdge),
	}

	return true
}

// GetNode from the graph backend
func (m *MemoryBackend) GetNode(i Identifier, t Context) []*Node {
	if n, ok := m.nodes[i]; ok {
		return []*Node{n.Node}
	}
	return nil
}

// GetNodeEdges returns a list of edges of a node
func (m *MemoryBackend) GetNodeEdges(n *Node, t Context, meta ElementMatcher) []*Edge {
	edges := []*Edge{}

	if n, ok := m.nodes[n.ID]; ok {
		for _, e := range n.edges {
			if e.MatchMetadata(meta) {
				edges = append(edges, e.Edge)
			}
		}
	}

	return edges
}

// EdgeDeleted in the graph backend
func (m *MemoryBackend) EdgeDeleted(e *Edge) bool {
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

// NodeDeleted in the graph backend
func (m *MemoryBackend) NodeDeleted(n *Node) (removed bool) {
	if _, removed = m.nodes[n.ID]; removed {
		delete(m.nodes, n.ID)
	}
	return
}

// GetNodes from the graph backend
func (m MemoryBackend) GetNodes(t Context, metadata ElementMatcher) (nodes []*Node) {
	for _, n := range m.nodes {
		if n.MatchMetadata(metadata) {
			nodes = append(nodes, n.Node)
		}
	}
	return
}

// GetEdges from the graph backend
func (m MemoryBackend) GetEdges(t Context, metadata ElementMatcher) (edges []*Edge) {
	for _, e := range m.edges {
		if e.MatchMetadata(metadata) {
			edges = append(edges, e.Edge)
		}
	}
	return
}

// IsHistorySupported returns that this backend doesn't support history
func (m *MemoryBackend) IsHistorySupported() bool {
	return false
}

// NewMemoryBackend creates a new graph memory backend
func NewMemoryBackend() (*MemoryBackend, error) {
	return &MemoryBackend{
		nodes: make(map[Identifier]*MemoryBackendNode),
		edges: make(map[Identifier]*MemoryBackendEdge),
	}, nil
}
