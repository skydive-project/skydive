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
)

type Identifier string

type GraphMessage struct {
	Type string
	Obj  interface{}
}

type GraphEventListener interface {
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
	Host      string
}

type JGraph Graph

type Graph struct {
	sync.RWMutex
	ID             Identifier
	Nodes          map[Identifier]*Node `json:"Nodes"`
	Edges          map[Identifier]*Edge `json:"Edges"`
	Host           string
	eventListeners []GraphEventListener `json:"-"`
}

type Node struct {
	GraphElement
	edges map[Identifier]*Edge `json:"-"`
}

type Edge struct {
	Parent *Node
	Child  *Node
	GraphElement
}

func (g GraphMessage) String() string {
	j, _ := json.Marshal(g)
	return string(j)
}

func GenID() Identifier {
	u, _ := uuid.NewV4()

	return Identifier(u.String())
}

func (e *GraphElement) String() string {
	j, _ := json.Marshal(e)
	return string(j)
}

func (e *Edge) SetMetadatas(m Metadatas) {
	e.Metadatas = m

	e.Graph.NotifyEdgeUpdated(e)
}

func (e *Edge) SetMetadata(k string, v interface{}) {
	if o, ok := e.Metadatas[k]; ok && o == v {
		return
	}

	e.Metadatas[k] = v

	e.Graph.NotifyEdgeUpdated(e)
}

func (e *Edge) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID        Identifier
		Parent    Identifier
		Child     Identifier
		Metadatas map[string]interface{} `json:",omitempty"`
		Host      string
	}{
		ID:        e.ID,
		Parent:    e.Parent.ID,
		Child:     e.Child.ID,
		Metadatas: e.Metadatas,
		Host:      e.Host,
	})
}

func (n *Node) SetMetadatas(m Metadatas) {
	n.Metadatas = m

	n.Graph.NotifyNodeUpdated(n)
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
		Host      string
	}{
		ID:        n.ID,
		Metadatas: n.Metadatas,
		Host:      n.Host,
	})
}

func (n *Node) getAncestorsTo(f Metadatas, ancestors []*Node) ([]*Node, bool) {
	ancestors = append(ancestors, n)

	for _, e := range n.edges {
		if e.Child == n && e.Parent.matchFilters(f) {
			ancestors = append(ancestors, e.Parent)

			return ancestors, true
		}
	}
	for _, e := range n.edges {
		if e.Child == n {
			a, ok := e.Parent.getAncestorsTo(f, ancestors)
			if ok {
				return a, ok
			}
		}
	}

	return ancestors, false
}

func (n *Node) GetAncestorsTo(f Metadatas) ([]*Node, bool) {
	ancestors, ok := n.getAncestorsTo(f, []*Node{})

	return ancestors, ok
}

func (n *Node) LookupParentNode(f Metadatas) *Node {
	for _, e := range n.edges {
		if e.Child == n && e.Parent.matchFilters(f) {
			return e.Parent
		}
	}

	return nil
}

func (n *Node) LookupChildren(f Metadatas) []*Node {
	children := []*Node{}

	for _, e := range n.edges {
		if e.Parent == n && e.Child.matchFilters(f) {
			children = append(children, e.Child)
		}
	}

	return children
}

func (n *Node) IsLinkedTo(c *Node) bool {
	for _, e := range n.edges {
		if e.Child == c || e.Parent == c {
			return true
		}
	}

	return false
}

func (n *Node) LinkTo(c *Node) {
	n.Graph.NewEdge(GenID(), n, c, nil)
}

func (n *Node) UnlinkFrom(c *Node) {
	for _, e := range n.edges {
		if e.Child == c {
			n.Graph.DelEdge(e)
		} else if e.Parent == c {
			n.Graph.DelEdge(e)
		}
	}
}

func (n *Node) Replace(o *Node, m Metadatas) *Node {
	for _, e := range n.edges {
		n.Graph.DelEdge(e)

		if e.Parent == n {
			n.Graph.NewEdge(GenID(), o, e.Child, e.Metadatas)
		} else {
			n.Graph.NewEdge(GenID(), e.Parent, o, e.Metadatas)
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
	e.Parent.edges[e.ID] = e
	e.Child.edges[e.ID] = e

	g.Edges[e.ID] = e

	g.NotifyEdgeAdded(e)
}

func (g *Graph) GetEdge(i Identifier) *Edge {
	if edge, ok := g.Edges[i]; ok {
		return edge
	}
	return nil
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

func (g *Graph) NewNode(i Identifier, m Metadatas) *Node {
	n := &Node{
		GraphElement: GraphElement{
			ID:    i,
			Graph: g,
			Host:  g.Host,
		},
		edges: make(map[Identifier]*Edge),
	}

	if m != nil {
		n.Metadatas = m
	} else {
		n.Metadatas = make(Metadatas)
	}

	g.AddNode(n)

	return n
}

func (g *Graph) NewEdge(i Identifier, p *Node, c *Node, m Metadatas) *Edge {
	e := &Edge{
		Parent: p,
		Child:  c,
		GraphElement: GraphElement{
			ID:    i,
			Graph: g,
			Host:  g.Host,
		},
	}

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

	delete(e.Parent.edges, e.ID)
	delete(e.Child.edges, e.ID)

	delete(g.Edges, e.ID)
	g.NotifyEdgeDeleted(e)
}

func (g *Graph) DelNode(n *Node) {
	if _, ok := g.Nodes[n.ID]; !ok {
		return
	}

	for _, e := range n.edges {
		g.DelEdge(e)
	}

	delete(g.Nodes, n.ID)
	g.NotifyNodeDeleted(n)
}

func (g *Graph) subtreeDel(n *Node, m map[Identifier]bool) {
	if _, ok := m[n.ID]; ok {
		return
	}
	m[n.ID] = true

	for _, e := range n.edges {
		if e.Child != n {
			g.subtreeDel(e.Child, m)
			g.DelNode(e.Child)
		}
	}
}

func (g *Graph) SubtreeDel(n *Node) {
	g.subtreeDel(n, make(map[Identifier]bool))
}

func (g *Graph) GetNodes() []*Node {
	nodes := []*Node{}

	for _, n := range g.Nodes {
		nodes = append(nodes, n)
	}

	return nodes
}

func (g *Graph) GetEdges() []*Edge {
	edges := []*Edge{}

	for _, e := range g.Edges {
		edges = append(edges, e)
	}

	return edges
}

func (g *Graph) String() string {
	j, _ := json.Marshal(g)
	return string(j)
}

func (g *Graph) MarshalJSON() ([]byte, error) {
	return json.Marshal(JGraph(*g))
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

func (g *Graph) UnmarshalGraphMessage(b []byte) (GraphMessage, error) {
	msg := GraphMessage{}

	err := json.Unmarshal(b, &msg)
	if err != nil {
		return msg, err
	}

	if msg.Type == "SyncRequest" {
		return msg, nil
	}

	objMap, ok := msg.Obj.(map[string]interface{})
	if !ok {
		return msg, errors.New("Unable to parse event: " + string(b))
	}

	ID := Identifier(objMap["ID"].(string))
	metadatas := make(Metadatas)
	if m, ok := objMap["Metadatas"]; ok {
		metadatas = Metadatas(m.(map[string]interface{}))
	}

	host := objMap["Host"].(string)

	switch msg.Type {
	case "SubtreeDeleted":
		fallthrough
	case "NodeUpdated":
		fallthrough
	case "NodeDeleted":
		fallthrough
	case "NodeAdded":
		if m, ok := objMap["Metadatas"]; ok {
			metadatas = Metadatas(m.(map[string]interface{}))
		}

		msg.Obj = &Node{
			GraphElement: GraphElement{
				ID:        ID,
				Metadatas: metadatas,
				Graph:     g,
				Host:      host,
			},
			edges: make(map[Identifier]*Edge),
		}
	case "EdgeUpdated":
		fallthrough
	case "EdgeDeleted":
		fallthrough
	case "EdgeAdded":
		parentID := Identifier(objMap["Parent"].(string))
		parent := g.GetNode(parentID)
		if parent == nil {
			return msg, errors.New("Edge node not found: " + string(parentID))
		}

		childID := Identifier(objMap["Child"].(string))
		child := g.GetNode(childID)
		if child == nil {
			return msg, errors.New("Edge node not found: " + string(childID))
		}

		msg.Obj = &Edge{
			GraphElement: GraphElement{
				ID:        ID,
				Metadatas: metadatas,
				Graph:     g,
				Host:      host,
			},
			Parent: parent,
			Child:  child,
		}
	}

	return msg, err
}

func NewGraph(i Identifier) (*Graph, error) {
	host, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &Graph{
		ID:    i,
		Nodes: make(map[Identifier]*Node),
		Edges: make(map[Identifier]*Edge),
		Host:  host,
	}, nil
}
