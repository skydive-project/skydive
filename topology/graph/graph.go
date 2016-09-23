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
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
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

type MetadataTransaction struct {
	graph        *Graph
	graphElement interface{}
	metadata     Metadata
}

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
	GetNode(i Identifier, at *time.Time) *Node
	GetNodeEdges(n *Node, at *time.Time) []*Edge

	AddEdge(e *Edge) bool
	DelEdge(e *Edge) bool
	GetEdge(i Identifier, at *time.Time) *Edge
	GetEdgeNodes(e *Edge, at *time.Time) (*Node, *Node)

	AddMetadata(e interface{}, k string, v interface{}) bool
	SetMetadata(e interface{}, m Metadata) bool

	GetNodes(at *time.Time) []*Node
	GetEdges(at *time.Time) []*Edge
}

type GraphContext struct {
	Time *time.Time
}

type graphEventListenerStack []GraphEventListener

type Graph struct {
	sync.RWMutex
	backend            GraphBackend
	context            GraphContext
	host               string
	eventListeners     []GraphEventListener
	eventListenerStack graphEventListenerStack
}

func (s *graphEventListenerStack) push(l GraphEventListener) {
	*s = append(*s, l)
}

func (s *graphEventListenerStack) pop() GraphEventListener {
	if len(*s) == 0 {
		return nil
	}

	l := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]

	return l
}

func (s *graphEventListenerStack) last() GraphEventListener {
	if len(*s) == 0 {
		return nil
	}
	return (*s)[len(*s)-1]
}

func (s *graphEventListenerStack) contains(l GraphEventListener) bool {
	if len(*s) == 0 {
		return false
	}

	for _, el := range *s {
		if reflect.ValueOf(el).Pointer() == reflect.ValueOf(l).Pointer() {
			return true
		}
	}
	return false
}

type MetadataMatcher interface {
	Match(v interface{}) bool
}

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

func (e *graphElement) Host() string {
	return e.host
}

func (e *graphElement) Metadata() Metadata {
	return e.metadata
}

func (e *graphElement) MatchMetadata(f Metadata) bool {
	for k, v := range f {
		switch v.(type) {
		case MetadataMatcher:
			nv, ok := e.metadata[k]
			matcher := v.(MetadataMatcher)
			if !ok || !matcher.Match(nv) {
				return false
			}
		default:
			nv, ok := e.metadata[k]
			if !ok || !common.CrossTypeEqual(nv, v) {
				return false
			}
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

func (n *Node) JsonRawMessage() *json.RawMessage {
	r, _ := n.MarshalJSON()
	raw := json.RawMessage(r)
	return &raw
}

func (n *Node) Decode(i interface{}) error {
	objMap, ok := i.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Unable to decode node: %v", i)
	}

	n.graphElement.ID = Identifier(objMap["ID"].(string))
	n.graphElement.host = objMap["Host"].(string)
	n.metadata = make(Metadata)
	if m, ok := objMap["Metadata"]; ok {
		n.metadata = Metadata(m.(map[string]interface{}))
	}

	return nil
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

func (e *Edge) JsonRawMessage() *json.RawMessage {
	r, _ := e.MarshalJSON()
	raw := json.RawMessage(r)
	return &raw
}

func (e *Edge) Decode(i interface{}) error {
	objMap, ok := i.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Unable to decode node: %v", i)
	}

	e.graphElement.ID = Identifier(objMap["ID"].(string))
	e.graphElement.host = objMap["Host"].(string)
	e.parent = Identifier(objMap["Parent"].(string))
	e.child = Identifier(objMap["Child"].(string))
	e.metadata = make(Metadata)
	if m, ok := objMap["Metadata"]; ok {
		e.metadata = Metadata(m.(map[string]interface{}))
	}

	return nil
}

func (e *Edge) GetParent() Identifier {
	return e.parent
}

func (e *Edge) GetChild() Identifier {
	return e.child
}

func (c *GraphContext) GetTime() *time.Time {
	return c.Time
}

func (g *Graph) notifyMetadataUpdated(e interface{}) {
	switch e.(type) {
	case *Node:
		g.NotifyNodeUpdated(e.(*Node))
	case *Edge:
		g.NotifyEdgeUpdated(e.(*Edge))
	}
}

func (g *Graph) SetMetadata(i interface{}, m Metadata) bool {
	switch i.(type) {
	case *Node:
		i.(*Node).metadata = m
	case *Edge:
		i.(*Edge).metadata = m
	}

	if !g.backend.SetMetadata(i, m) {
		return false
	}
	g.notifyMetadataUpdated(i)
	return true
}

func (g *Graph) AddMetadata(i interface{}, k string, v interface{}) bool {
	var e graphElement

	switch i.(type) {
	case *Node:
		e = i.(*Node).graphElement
	case *Edge:
		e = i.(*Edge).graphElement
	}

	if o, ok := e.metadata[k]; ok && o == v {
		return false
	}
	e.metadata[k] = v

	if !g.backend.AddMetadata(i, k, v) {
		return false
	}

	g.notifyMetadataUpdated(i)
	return true
}

func (t *MetadataTransaction) AddMetadata(k string, v interface{}) {
	t.metadata[k] = v
}

func (t *MetadataTransaction) Commit() {
	var e graphElement

	switch t.graphElement.(type) {
	case *Node:
		e = t.graphElement.(*Node).graphElement
	case *Edge:
		e = t.graphElement.(*Edge).graphElement
	}

	updated := false
	for k, v := range t.metadata {
		if e.metadata[k] != v {
			e.metadata[k] = v
			if !t.graph.backend.AddMetadata(t.graphElement, k, v) {
				return
			}
			updated = true
		}
	}
	if updated {
		t.graph.notifyMetadataUpdated(t.graphElement)
	}
}

func (g *Graph) StartMetadataTransaction(i interface{}) *MetadataTransaction {
	var e graphElement

	switch i.(type) {
	case *Node:
		e = i.(*Node).graphElement
	case *Edge:
		e = i.(*Edge).graphElement
	}

	t := MetadataTransaction{
		graph:        g,
		graphElement: i,
		metadata:     make(Metadata),
	}
	for k, v := range e.metadata {
		t.metadata[k] = v
	}

	return &t
}

func (g *Graph) lookupShortestPath(n *Node, m Metadata, path []*Node, v map[Identifier]bool, em ...Metadata) []*Node {
	v[n.ID] = true

	newPath := make([]*Node, len(path)+1)
	copy(newPath, path)
	newPath[len(path)] = n

	if n.MatchMetadata(m) {
		return newPath
	}

	t := g.context.GetTime()
	shortest := []*Node{}
	for _, e := range g.backend.GetNodeEdges(n, t) {
		if len(em) > 0 && !e.MatchMetadata(em[0]) {
			continue
		}

		parent, child := g.backend.GetEdgeNodes(e, t)
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
			nv := make(map[Identifier]bool)
			for k, v := range v {
				nv[k] = v
			}

			sub := g.lookupShortestPath(neighbor, m, newPath, nv, em...)
			if len(sub) > 0 && (len(shortest) == 0 || len(sub) < len(shortest)) {
				shortest = sub
			}
		}
	}

	// check that the last element if the one we looked for
	if len(shortest) > 0 && !shortest[len(shortest)-1].MatchMetadata(m) {
		return []*Node{}
	}

	return shortest
}

func (g *Graph) LookupShortestPath(n *Node, m Metadata, em ...Metadata) []*Node {
	return g.lookupShortestPath(n, m, []*Node{}, make(map[Identifier]bool), em...)
}

func (g *Graph) LookupParents(n *Node, f Metadata, em ...Metadata) []*Node {
	parents := []*Node{}
	t := g.context.GetTime()
	for _, e := range g.backend.GetNodeEdges(n, t) {
		if len(em) > 0 && !e.MatchMetadata(em[0]) {
			continue
		}

		parent, child := g.backend.GetEdgeNodes(e, t)

		if child != nil && child.ID == n.ID && parent.MatchMetadata(f) {
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

func (g *Graph) LookupChildren(n *Node, f Metadata, em ...Metadata) []*Node {
	children := []*Node{}
	t := g.context.GetTime()
	for _, e := range g.backend.GetNodeEdges(n, t) {
		if len(em) > 0 && !e.MatchMetadata(em[0]) {
			continue
		}

		parent, child := g.backend.GetEdgeNodes(e, t)

		if parent != nil && parent.ID == n.ID && child.MatchMetadata(f) {
			children = append(children, child)
		}
	}

	return children
}

func (g *Graph) AreLinked(n1 *Node, n2 *Node) bool {
	t := g.context.GetTime()
	for _, e := range g.backend.GetNodeEdges(n1, t) {
		parent, child := g.backend.GetEdgeNodes(e, t)
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
	for _, e := range g.backend.GetNodeEdges(n1, nil) {
		parent, child := g.backend.GetEdgeNodes(e, nil)
		if parent == nil || child == nil {
			continue
		}

		if child.ID == n2.ID || parent.ID == n2.ID {
			g.DelEdge(e)
		}
	}
}

func (g *Graph) Replace(o *Node, n *Node) *Node {
	for _, e := range g.backend.GetNodeEdges(o, nil) {
		parent, child := g.backend.GetEdgeNodes(e, nil)
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

	for _, n := range g.backend.GetNodes(g.context.GetTime()) {
		if n.MatchMetadata(m) {
			nodes = append(nodes, n)
		}
	}

	return nodes
}

func (g *Graph) LookupNodesFromKey(key string) []*Node {
	nodes := []*Node{}

	for _, n := range g.backend.GetNodes(g.context.GetTime()) {
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
	return g.backend.GetEdge(i, g.context.GetTime())
}

func (g *Graph) AddNode(n *Node) bool {
	if !g.backend.AddNode(n) {
		return false
	}
	g.NotifyNodeAdded(n)

	return true
}

func (g *Graph) GetNode(i Identifier) *Node {
	return g.backend.GetNode(i, g.context.GetTime())
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
	for _, e := range g.backend.GetNodeEdges(n, nil) {
		g.DelEdge(e)
	}

	if g.backend.DelNode(n) {
		g.NotifyNodeDeleted(n)
	}
}

func (g *Graph) delSubGraph(n *Node, v map[Identifier]bool) {
	v[n.ID] = true

	for _, e := range g.backend.GetNodeEdges(n, nil) {
		parent, child := g.backend.GetEdgeNodes(e, nil)

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
	return g.backend.GetNodes(g.context.GetTime())
}

func (g *Graph) GetEdges() []*Edge {
	return g.backend.GetEdges(g.context.GetTime())
}

func (g *Graph) GetEdgeNodes(e *Edge) (*Node, *Node) {
	return g.backend.GetEdgeNodes(e, g.context.GetTime())
}

func (g *Graph) GetNodeEdges(n *Node) []*Edge {
	return g.backend.GetNodeEdges(n, g.context.GetTime())
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
		if g.eventListenerStack.contains(l) {
			continue
		}

		g.eventListenerStack.push(l)
		l.OnNodeUpdated(n)
		g.eventListenerStack.pop()
	}
}

func (g *Graph) NotifyNodeDeleted(n *Node) {
	for _, l := range g.eventListeners {
		if g.eventListenerStack.contains(l) {
			continue
		}

		g.eventListenerStack.push(l)
		l.OnNodeDeleted(n)
		g.eventListenerStack.pop()
	}
}

func (g *Graph) NotifyNodeAdded(n *Node) {
	for _, l := range g.eventListeners {
		if g.eventListenerStack.contains(l) {
			continue
		}

		g.eventListenerStack.push(l)
		l.OnNodeAdded(n)
		g.eventListenerStack.pop()
	}
}

func (g *Graph) NotifyEdgeUpdated(e *Edge) {
	for _, l := range g.eventListeners {
		if g.eventListenerStack.contains(l) {
			continue
		}

		g.eventListenerStack.push(l)
		l.OnEdgeUpdated(e)
		g.eventListenerStack.pop()
	}
}

func (g *Graph) NotifyEdgeDeleted(e *Edge) {
	for _, l := range g.eventListeners {
		if g.eventListenerStack.contains(l) {
			continue
		}

		g.eventListenerStack.push(l)
		l.OnEdgeDeleted(e)
		g.eventListenerStack.pop()
	}
}

func (g *Graph) NotifyEdgeAdded(e *Edge) {
	for _, l := range g.eventListeners {
		if g.eventListenerStack.contains(l) {
			continue
		}

		g.eventListenerStack.push(l)
		l.OnEdgeAdded(e)
		g.eventListenerStack.pop()
	}
}

func (g *Graph) AddEventListener(l GraphEventListener) {
	g.Lock()
	defer g.Unlock()

	g.eventListeners = append(g.eventListeners, l)
}

func (g *Graph) RemoveEventListener(l GraphEventListener) {
	g.Lock()
	defer g.Unlock()

	for i, el := range g.eventListeners {
		if l == el {
			g.eventListeners = append(g.eventListeners[:i], g.eventListeners[i+1:]...)
			break
		}
	}
}

func (g *Graph) WithContext(c GraphContext) *Graph {
	newG := *g
	newG.context = c
	return &newG
}

func (g *Graph) GetContext() GraphContext {
	return g.context
}

func NewGraph(hostID string, backend GraphBackend) (*Graph, error) {
	return NewGraphWithContext(hostID, backend, GraphContext{})
}

func NewGraphFromConfig(backend GraphBackend) (*Graph, error) {
	hostID := config.GetConfig().GetString("host_id")
	return NewGraph(hostID, backend)
}

func NewGraphWithContext(hostID string, b GraphBackend, c GraphContext) (*Graph, error) {
	return &Graph{
		backend: b,
		host:    hostID,
		context: c,
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
	case "orientdb":
		return NewOrientDBBackendFromConfig()
	default:
		return nil, errors.New("Config file is misconfigured, graph backend unknown: " + backend)
	}
}
