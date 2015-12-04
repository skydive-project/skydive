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

package topology

import (
	"encoding/json"
	"sync"

	"github.com/nu7hatch/gouuid"
)

type Identifier string

type EventListener interface {
	OnNodeUpdated(n *Node)
	OnNodeAdded(n *Node)
	OnNodeDeleted(n *Node)
	OnEdgeUpdated(e *Edge)
	OnEdgeAdded(e *Edge)
	OnEdgeDeleted(e *Edge)
}

type Metadatas map[string]interface{}

type GraphElement struct {
	ID        Identifier
	Graph     *Graph    `json:"-"`
	Metadatas Metadatas `json:",omitempty"`
}

type JGraph Graph

type Graph struct {
	sync.RWMutex
	ID             Identifier
	Nodes          map[Identifier]*Node `json:"Nodes"`
	Edges          map[Identifier]*Edge `json:"Edges"`
	EventListeners []EventListener      `json:"-"`
}

type Node struct {
	GraphElement
	Edges map[Identifier]*Edge
}

type Edge struct {
	Parent *Node
	Child  *Node
	GraphElement
}

func GenID() Identifier {
	u, _ := uuid.NewV4()

	return Identifier(u.String())
}

func (e *GraphElement) String() string {
	j, _ := json.Marshal(e)
	return string(j)
}

func (e *Edge) SetMetadata(k string, v interface{}) {
	e.Metadatas[k] = v

	e.Graph.NotifyEdgeUpdated(e)
}

func (e *Edge) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID        Identifier
		Parent    Identifier
		Child     Identifier
		Metadatas map[string]interface{} `json:",omitempty"`
	}{
		ID:        e.ID,
		Parent:    e.Parent.ID,
		Child:     e.Child.ID,
		Metadatas: e.Metadatas,
	})
}

func (n *Node) SetMetadata(k string, v interface{}) {
	if o, ok := n.Metadatas[k]; ok && o == v {
		return
	}

	n.Metadatas[k] = v

	n.Graph.NotifyNodeUpdated(n)
}

func (n *Node) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID        Identifier
		Metadatas map[string]interface{} `json:",omitempty"`
	}{
		ID:        n.ID,
		Metadatas: n.Metadatas,
	})
}

func (n *Node) LookupParentNode(f Metadatas) *Node {
	for _, e := range n.Edges {
		if e.Child == n && e.Parent.matchFilters(f) {
			return e.Parent
		}
	}

	return nil
}

func (n *Node) LookupChildren(f Metadatas) []*Node {
	children := []*Node{}

	for _, e := range n.Edges {
		if e.Parent == n && e.Child.matchFilters(f) {
			children = append(children, e.Child)
		}
	}

	return children
}

func (n *Node) IsLinkedTo(c *Node) bool {
	for _, e := range n.Edges {
		if e.Child == c || e.Parent == c {
			return true
		}
	}

	return false
}

func (n *Node) LinkTo(c *Node) {
	n.Graph.NewEdge(n, c, nil)
}

func (n *Node) UnlinkFrom(c *Node) {
	for _, e := range n.Edges {
		if e.Child == c {
			n.Graph.DelEdge(e)
		} else if e.Parent == c {
			n.Graph.DelEdge(e)
		}
	}
}

func (n *Node) Replace(o *Node, m Metadatas) *Node {
	for _, e := range n.Edges {
		n.Graph.DelEdge(e)

		if e.Parent == n {
			n.Graph.NewEdge(o, e.Child, e.Metadatas)
		} else {
			n.Graph.NewEdge(e.Parent, o, e.Metadatas)
		}
	}

	// copy metadatas
	for k, v := range m {
		if _, ok := n.Metadatas[k]; ok {
			o.Metadatas[k] = v
		}
	}
	n.Graph.NotifyNodeUpdated(o)

	n.Graph.DelNode(n)

	return o
}

func (n *Node) matchFilters(f Metadatas) bool {
	for k, v := range f {
		nv, ok := n.Metadatas[k]
		if !ok || v != nv {
			return false
		}
	}

	return true
}

func (g *Graph) LookupNode(f Metadatas) *Node {
	for _, n := range g.Nodes {
		if n.matchFilters(f) {
			return n
		}
	}

	return nil
}

func (g *Graph) LookupNodes(f Metadatas) []*Node {
	nodes := []*Node{}

	for _, n := range g.Nodes {
		if n.matchFilters(f) {
			nodes = append(nodes, n)
		}
	}

	return nodes
}

func (g *Graph) AddEdge(e *Edge) {
	g.Edges[e.ID] = e

	g.NotifyEdgeAdded(e)
}

func (g *Graph) AddNode(n *Node) {
	g.Nodes[n.ID] = n

	g.NotifyNodeAdded(n)
}

func (g *Graph) GetNode(i Identifier) *Node {
	if node, ok := g.Nodes[i]; ok {
		return node
	}
	return nil
}

func (g *Graph) NewNode(m Metadatas) *Node {
	n := &Node{
		GraphElement: GraphElement{
			ID:    GenID(),
			Graph: g,
		},
		Edges: make(map[Identifier]*Edge),
	}

	if m != nil {
		n.Metadatas = m
	} else {
		n.Metadatas = make(Metadatas)
	}

	g.AddNode(n)

	return n
}

func (g *Graph) NewEdge(p *Node, c *Node, m Metadatas) *Edge {
	e := &Edge{
		Parent: p,
		Child:  c,
		GraphElement: GraphElement{
			ID:    GenID(),
			Graph: g,
		},
	}

	p.Edges[e.ID] = e
	c.Edges[e.ID] = e

	if m != nil {
		e.Metadatas = m
	} else {
		e.Metadatas = make(Metadatas)
	}

	g.AddEdge(e)

	return e
}

func (g *Graph) DelEdge(e *Edge) {
	if _, ok := g.Edges[e.ID]; !ok {
		return
	}

	delete(e.Parent.Edges, e.ID)
	if len(e.Parent.Edges) == 0 {
		node := g.Nodes[e.Parent.ID]
		delete(g.Nodes, e.Parent.ID)

		g.NotifyNodeDeleted(node)
	}

	delete(e.Child.Edges, e.ID)
	if len(e.Child.Edges) == 0 {
		node := g.Nodes[e.Child.ID]
		delete(g.Nodes, e.Child.ID)

		g.NotifyNodeDeleted(node)
	}

	delete(g.Edges, e.ID)
	g.NotifyEdgeDeleted(e)
}

func (g *Graph) DelNode(n *Node) {
	if _, ok := g.Nodes[n.ID]; !ok {
		return
	}

	for _, e := range n.Edges {
		g.DelEdge(e)
	}

	delete(g.Nodes, n.ID)
	g.NotifyNodeDeleted(n)
}

func (g *Graph) String() string {
	j, _ := json.Marshal(g)
	return string(j)
}

func (g *Graph) MarshalJSON() ([]byte, error) {
	g.RLock()
	defer g.RUnlock()

	return json.Marshal(JGraph(*g))
}

func (g *Graph) NotifyNodeUpdated(n *Node) {
	for _, l := range g.EventListeners {
		l.OnNodeUpdated(n)
	}
}

func (g *Graph) NotifyNodeDeleted(n *Node) {
	for _, l := range g.EventListeners {
		l.OnNodeDeleted(n)
	}
}

func (g *Graph) NotifyNodeAdded(n *Node) {
	for _, l := range g.EventListeners {
		l.OnNodeAdded(n)
	}
}

func (g *Graph) NotifyEdgeUpdated(e *Edge) {
	for _, l := range g.EventListeners {
		l.OnEdgeUpdated(e)
	}
}

func (g *Graph) NotifyEdgeDeleted(e *Edge) {
	for _, l := range g.EventListeners {
		l.OnEdgeDeleted(e)
	}
}

func (g *Graph) NotifyEdgeAdded(e *Edge) {
	for _, l := range g.EventListeners {
		l.OnEdgeAdded(e)
	}
}

func (g *Graph) AddEventListener(l EventListener) {
	g.Lock()
	defer g.Unlock()

	g.EventListeners = append(g.EventListeners, l)
}

func NewGraph(i Identifier) *Graph {
	return &Graph{
		ID:    i,
		Nodes: make(map[Identifier]*Node),
		Edges: make(map[Identifier]*Edge),
	}
}
