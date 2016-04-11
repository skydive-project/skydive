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
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/nu7hatch/gouuid"

	"github.com/redhat-cip/skydive/config"
)

type Identifier string

type GraphEventListener interface {
	OnNodeUpdated(n *Node)
	OnNodeAdded(n *Node)
	OnNodeDeleted(n *Node)
	OnEdgeUpdated(e *Edge)
	OnEdgeAdded(e *Edge)
	OnEdgeDeleted(e *Edge)
}

type Metadata map[string]interface{}

type graphElement struct {
	ID       Identifier
	metadata Metadata
	host     string
}

type Node struct {
	graphElement
}

type Edge struct {
	graphElement
	parent Identifier
	child  Identifier
}

type GraphBackend interface {
	AddNode(n *Node) bool
	DelNode(n *Node) bool
	GetNode(i Identifier) *Node
	GetNodeEdges(n *Node) []*Edge

	AddEdge(e *Edge) bool
	DelEdge(e *Edge) bool
	GetEdge(i Identifier) *Edge
	GetEdgeNodes(e *Edge) (*Node, *Node)

	AddMetadata(e interface{}, k string, v interface{}) bool
	SetMetadata(e interface{}, m Metadata) bool

	GetNodes() []*Node
	GetEdges() []*Edge
}

type Graph struct {
	sync.RWMutex
	backend        GraphBackend
	host           string
	eventListeners []GraphEventListener
}

type EdgeValidator func(e *Edge) bool

// default implementation of a graph listener, can be used when not implementing
// the whole set of callbacks
type DefaultGraphListener struct {
}

func (d *DefaultGraphListener) OnNodeUpdated(n *Node) {
}

func (c *DefaultGraphListener) OnNodeAdded(n *Node) {
}

func (c *DefaultGraphListener) OnNodeDeleted(n *Node) {
}

func (c *DefaultGraphListener) OnEdgeUpdated(e *Edge) {
}

func (c *DefaultGraphListener) OnEdgeAdded(e *Edge) {
}

func (c *DefaultGraphListener) OnEdgeDeleted(e *Edge) {
}

func GenID() Identifier {
	u, _ := uuid.NewV4()

	return Identifier(u.String())
}

func (m *Metadata) String() string {
	j, _ := json.Marshal(m)
	return string(j)
}

func (e *graphElement) Metadata() Metadata {
	return e.metadata
}

func (e *graphElement) matchFilters(f Metadata) bool {
	for k, v := range f {
		nv, ok := e.metadata[k]
		if !ok || v != nv {
			return false
		}
	}

	return true
}

func (e *graphElement) String() string {
	j, _ := json.Marshal(&struct {
		ID       Identifier
		Metadata Metadata `json:",omitempty"`
		Host     string
	}{
		ID:       e.ID,
		Metadata: e.metadata,
		Host:     e.host,
	})
	return string(j)
}

func (n *Node) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID       Identifier
		Metadata Metadata `json:",omitempty"`
		Host     string
	}{
		ID:       n.ID,
		Metadata: n.metadata,
		Host:     n.host,
	})
}

func (e *Edge) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID       Identifier
		Metadata Metadata `json:",omitempty"`
		Parent   Identifier
		Child    Identifier
		Host     string
	}{
		ID:       e.ID,
		Metadata: e.metadata,
		Parent:   e.parent,
		Child:    e.child,
		Host:     e.host,
	})
}

func (g *Graph) notifyMetadataUpdated(e interface{}) {
	switch e.(type) {
	case *Node:
		g.NotifyNodeUpdated(e.(*Node))
	case *Edge:
		g.NotifyEdgeUpdated(e.(*Edge))
	}
}

func (g *Graph) SetMetadata(e interface{}, m Metadata) {
	if !g.backend.SetMetadata(e, m) {
		return
	}
	g.notifyMetadataUpdated(e)
}

func (g *Graph) AddMetadata(e interface{}, k string, v interface{}) {
	if !g.backend.AddMetadata(e, k, v) {
		return
	}
	g.notifyMetadataUpdated(e)
}

func (g *Graph) lookupShortestPath(n *Node, m Metadata, path []*Node, v map[Identifier]bool, ev ...EdgeValidator) []*Node {
	v[n.ID] = true

	path = append(path, n)

	if n.matchFilters(m) {
		return path
	}

	shortest := []*Node{}
	for _, e := range g.backend.GetNodeEdges(n) {
		if len(ev) > 0 && !ev[0](e) {
			continue
		}

		parent, child := g.backend.GetEdgeNodes(e)
		if parent == nil || child == nil {
			continue
		}

		var neighbor *Node
		if parent.ID != n.ID && !v[parent.ID] {
			neighbor = parent
		}

		if child.ID != n.ID && !v[child.ID] {
			neighbor = child
		}

		if neighbor != nil {
			sub := g.lookupShortestPath(neighbor, m, path, v)
			if len(sub) > 0 && (len(shortest) == 0 || len(sub) < len(shortest)) {
				shortest = sub
			}
		}
	}

	return shortest
}

func (g *Graph) LookupShortestPath(n *Node, m Metadata, ev ...EdgeValidator) []*Node {
	return g.lookupShortestPath(n, m, []*Node{}, make(map[Identifier]bool), ev...)
}

func (g *Graph) LookupParentNodes(n *Node, f Metadata) []*Node {
	parents := []*Node{}

	for _, e := range g.backend.GetNodeEdges(n) {
		parent, child := g.backend.GetEdgeNodes(e)

		if child != nil && child.ID == n.ID && parent.matchFilters(f) {
			parents = append(parents, parent)
		}
	}

	return parents
}

func (g *Graph) LookupFirstChild(n *Node, f Metadata) *Node {
	nodes := g.LookupChildren(n, f)
	if len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}

func (g *Graph) LookupChildren(n *Node, f Metadata) []*Node {
	children := []*Node{}

	for _, e := range g.backend.GetNodeEdges(n) {
		parent, child := g.backend.GetEdgeNodes(e)

		if parent != nil && parent.ID == n.ID && child.matchFilters(f) {
			children = append(children, child)
		}
	}

	return children
}

func (g *Graph) AreLinked(n1 *Node, n2 *Node) bool {
	for _, e := range g.backend.GetNodeEdges(n1) {
		parent, child := g.backend.GetEdgeNodes(e)
		if parent == nil || child == nil {
			continue
		}

		if child.ID == n2.ID || parent.ID == n2.ID {
			return true
		}
	}

	return false
}

func (g *Graph) Link(n1 *Node, n2 *Node, m ...Metadata) {
	if len(m) > 0 {
		g.NewEdge(GenID(), n1, n2, m[0])
	} else {
		g.NewEdge(GenID(), n1, n2, nil)
	}
}

func (g *Graph) Unlink(n1 *Node, n2 *Node) {
	for _, e := range g.backend.GetNodeEdges(n1) {
		parent, child := g.backend.GetEdgeNodes(e)
		if parent == nil || child == nil {
			continue
		}

		if child.ID == n2.ID || parent.ID == n2.ID {
			g.DelEdge(e)
		}
	}
}

func (g *Graph) Replace(o *Node, n *Node) *Node {
	for _, e := range g.backend.GetNodeEdges(o) {
		parent, child := g.backend.GetEdgeNodes(e)
		if parent == nil || child == nil {
			continue
		}

		g.DelEdge(e)

		if parent.ID == n.ID {
			g.Link(n, child, e.metadata)
		} else {
			g.Link(parent, n, e.metadata)
		}
	}
	n.metadata = o.metadata
	g.NotifyNodeUpdated(n)

	g.DelNode(o)

	return n
}

func (g *Graph) LookupFirstNode(m Metadata) *Node {
	nodes := g.LookupNodes(m)
	if len(nodes) > 0 {
		return nodes[0]
	}

	return nil
}

func (g *Graph) LookupNodes(m Metadata) []*Node {
	nodes := []*Node{}

	for _, n := range g.backend.GetNodes() {
		if n.matchFilters(m) {
			nodes = append(nodes, n)
		}
	}

	return nodes
}

func (g *Graph) LookupNodesFromKey(key string) []*Node {
	nodes := []*Node{}

	for _, n := range g.backend.GetNodes() {
		_, ok := n.metadata[key]
		if ok {
			nodes = append(nodes, n)
		}
	}

	return nodes
}

func (g *Graph) AddEdge(e *Edge) bool {
	if !g.backend.AddEdge(e) {
		return false
	}
	g.NotifyEdgeAdded(e)

	return true
}

func (g *Graph) GetEdge(i Identifier) *Edge {
	return g.backend.GetEdge(i)
}

func (g *Graph) AddNode(n *Node) bool {
	if !g.backend.AddNode(n) {
		return false
	}
	g.NotifyNodeAdded(n)

	return true
}

func (g *Graph) GetNode(i Identifier) *Node {
	return g.backend.GetNode(i)
}

func (g *Graph) NewNode(i Identifier, m Metadata) *Node {
	n := &Node{
		graphElement: graphElement{
			ID:   i,
			host: g.host,
		},
	}

	if m != nil {
		n.metadata = m
	} else {
		n.metadata = make(Metadata)
	}

	if !g.AddNode(n) {
		return nil
	}

	return n
}

func (g *Graph) NewEdge(i Identifier, p *Node, c *Node, m Metadata) *Edge {
	e := &Edge{
		parent: p.ID,
		child:  c.ID,
		graphElement: graphElement{
			ID:   i,
			host: g.host,
		},
	}

	if m != nil {
		e.metadata = m
	} else {
		e.metadata = make(Metadata)
	}

	if !g.AddEdge(e) {
		return nil
	}

	return e
}

func (g *Graph) DelEdge(e *Edge) {
	if g.backend.DelEdge(e) {
		g.NotifyEdgeDeleted(e)
	}
}

func (g *Graph) DelNode(n *Node) {
	for _, e := range g.backend.GetNodeEdges(n) {
		g.DelEdge(e)
	}

	if g.backend.DelNode(n) {
		g.NotifyNodeDeleted(n)
	}
}

func (g *Graph) delSubGraph(n *Node, v map[Identifier]bool) {
	v[n.ID] = true

	for _, e := range g.backend.GetNodeEdges(n) {
		parent, child := g.backend.GetEdgeNodes(e)

		if parent != nil && parent.ID != n.ID && !v[parent.ID] {
			g.delSubGraph(parent, v)
			g.DelNode(parent)
		}

		if child != nil && child.ID != n.ID && !v[child.ID] {
			g.delSubGraph(child, v)
			g.DelNode(child)
		}
	}
}

func (g *Graph) DelSubGraph(n *Node) {
	g.delSubGraph(n, make(map[Identifier]bool))
}

func (g *Graph) GetNodes() []*Node {
	return g.backend.GetNodes()
}

func (g *Graph) GetEdges() []*Edge {
	return g.backend.GetEdges()
}

func (g *Graph) GetEdgeNodes(e *Edge) (*Node, *Node) {
	return g.backend.GetEdgeNodes(e)
}

func (g *Graph) String() string {
	j, _ := json.Marshal(g)
	return string(j)
}

func (g *Graph) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Nodes []*Node
		Edges []*Edge
	}{
		Nodes: g.GetNodes(),
		Edges: g.GetEdges(),
	})
}

func (g *Graph) NotifyNodeUpdated(n *Node) {
	for _, l := range g.eventListeners {
		l.OnNodeUpdated(n)
	}
}

func (g *Graph) NotifyNodeDeleted(n *Node) {
	for _, l := range g.eventListeners {
		l.OnNodeDeleted(n)
	}
}

func (g *Graph) NotifyNodeAdded(n *Node) {
	for _, l := range g.eventListeners {
		l.OnNodeAdded(n)
	}
}

func (g *Graph) NotifyEdgeUpdated(e *Edge) {
	for _, l := range g.eventListeners {
		l.OnEdgeUpdated(e)
	}
}

func (g *Graph) NotifyEdgeDeleted(e *Edge) {
	for _, l := range g.eventListeners {
		l.OnEdgeDeleted(e)
	}
}

func (g *Graph) NotifyEdgeAdded(e *Edge) {
	for _, l := range g.eventListeners {
		l.OnEdgeAdded(e)
	}
}

func (g *Graph) AddEventListener(l GraphEventListener) {
	g.Lock()
	defer g.Unlock()

	g.eventListeners = append(g.eventListeners, l)
}

func NewGraph(b GraphBackend) (*Graph, error) {
	h, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &Graph{
		backend: b,
		host:    h,
	}, nil
}

func BackendFromConfig() (GraphBackend, error) {
	backend := config.GetConfig().GetString("graph.backend")
	if len(backend) == 0 {
		backend = "memory"
	}

	switch backend {
	case "memory":
		return NewMemoryBackend()
	case "gremlin":
		endpoint := config.GetConfig().GetString("graph.gremlin")
		return NewGremlinBackend(endpoint)
	case "titangraph":
		endpoint := config.GetConfig().GetString("graph.gremlin")
		return NewTitangraphBackend(endpoint)
	default:
		return nil, errors.New("Config file is misconfigured, graph backend unknown: " + backend)
	}
}
