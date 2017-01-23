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
	"sync"
	"time"

	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
)

const (
	Direct int64 = iota
	Shadowed
)

const (
	maxEvents = 50
)

type graphEventType int

const (
	nodeUpdated graphEventType = iota + 1
	nodeAdded
	nodeDeleted
	edgeUpdated
	edgeAdded
	edgeDeleted
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

type graphEvent struct {
	kind     graphEventType
	element  interface{}
	listener GraphEventListener
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

	GetNodes(at *time.Time, m Metadata) []*Node
	GetEdges(at *time.Time, m Metadata) []*Edge

	WithContext(graph *Graph, context GraphContext) (*Graph, error)
}

type GraphContext struct {
	Time *time.Time
}

type Graph struct {
	sync.RWMutex
	backend              GraphBackend
	context              GraphContext
	host                 string
	eventListeners       []GraphEventListener
	eventChan            chan graphEvent
	eventConsumed        bool
	currentEventListener GraphEventListener
}

type MetadataMatcher interface {
	Match(v interface{}) bool
}

type HostNodeTIDMap map[string][]string

func BuildHostNodeTIDMap(nodes []*Node) HostNodeTIDMap {
	hnmap := make(HostNodeTIDMap)
	for _, node := range nodes {
		if host := node.Host(); host != "" {
			hnmap[host] = append(hnmap[host], string(node.ID))
		}
	}
	return hnmap
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

func (g *Graph) SetMetadata(i interface{}, m Metadata) bool {
	ge := graphEvent{element: i}

	switch i.(type) {
	case *Node:
		i.(*Node).metadata = m
		ge.kind = nodeUpdated
	case *Edge:
		i.(*Edge).metadata = m
		ge.kind = edgeUpdated
	}

	if !g.backend.SetMetadata(i, m) {
		return false
	}

	g.notifyEvent(ge)
	return true
}

func (g *Graph) AddMetadata(i interface{}, k string, v interface{}) bool {
	var e graphElement
	ge := graphEvent{element: i}

	switch i.(type) {
	case *Node:
		e = i.(*Node).graphElement
		ge.kind = nodeUpdated
	case *Edge:
		e = i.(*Edge).graphElement
		ge.kind = edgeUpdated
	}

	if o, ok := e.metadata[k]; ok && o == v {
		return false
	}
	e.metadata[k] = v

	if !g.backend.AddMetadata(i, k, v) {
		return false
	}

	g.notifyEvent(ge)
	return true
}

func (t *MetadataTransaction) AddMetadata(k string, v interface{}) {
	t.metadata[k] = v
}

func (t *MetadataTransaction) Commit() {
	var e graphElement
	ge := graphEvent{element: t.graphElement}

	switch t.graphElement.(type) {
	case *Node:
		e = t.graphElement.(*Node).graphElement
		ge.kind = nodeUpdated
	case *Edge:
		e = t.graphElement.(*Edge).graphElement
		ge.kind = edgeUpdated
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
		t.graph.notifyEvent(ge)
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

func (g *Graph) LookupEdges(pm Metadata, cm Metadata, em Metadata) []*Edge {
	edges := []*Edge{}
	t := g.context.GetTime()
	for _, e := range g.backend.GetEdges(t, em) {
		parent, child := g.backend.GetEdgeNodes(e, t)

		if parent != nil && child != nil && parent.MatchMetadata(pm) && child.MatchMetadata(cm) {
			edges = append(edges, e)
		}
	}

	return edges
}

func (g *Graph) AreLinked(n1 *Node, n2 *Node, m ...Metadata) bool {
	t := g.context.GetTime()
	for _, e := range g.backend.GetNodeEdges(n1, t) {
		parent, child := g.backend.GetEdgeNodes(e, t)
		if parent == nil || child == nil {
			continue
		}

		if child.ID == n2.ID || parent.ID == n2.ID {
			if len(m) > 0 {
				if e.MatchMetadata(m[0]) {
					return true
				}
			} else {
				return true
			}
		}
	}

	return false
}

func (g *Graph) Link(n1 *Node, n2 *Node, m ...Metadata) *Edge {
	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(string(n1.ID)+string(n2.ID)))

	if len(m) > 0 {
		return g.NewEdge(Identifier(u.String()), n1, n2, m[0])
	}
	return g.NewEdge(Identifier(u.String()), n1, n2, nil)
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
	g.notifyEvent(graphEvent{element: n, kind: nodeUpdated})

	g.DelNode(o)

	return n
}

func (g *Graph) LookupFirstNode(m Metadata) *Node {
	nodes := g.GetNodes(m)
	if len(nodes) > 0 {
		return nodes[0]
	}

	return nil
}

func (g *Graph) LookupNodesFromKey(key string) []*Node {
	nodes := []*Node{}

	for _, n := range g.backend.GetNodes(g.context.GetTime(), Metadata{}) {
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
	g.notifyEvent(graphEvent{element: e, kind: edgeAdded})

	return true
}

func (g *Graph) GetEdge(i Identifier) *Edge {
	return g.backend.GetEdge(i, g.context.GetTime())
}

func (g *Graph) AddNode(n *Node) bool {
	if !g.backend.AddNode(n) {
		return false
	}
	g.notifyEvent(graphEvent{element: n, kind: nodeAdded})

	return true
}

func (g *Graph) GetNode(i Identifier) *Node {
	return g.backend.GetNode(i, g.context.GetTime())
}

func (g *Graph) NewNode(i Identifier, m Metadata, h ...string) *Node {
	hostname := g.host
	if len(h) > 0 {
		hostname = h[0]
	}
	n := &Node{
		graphElement: graphElement{
			ID:   i,
			host: hostname,
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
		g.notifyEvent(graphEvent{element: e, kind: edgeDeleted})
	}
}

func (g *Graph) DelNode(n *Node) {
	for _, e := range g.backend.GetNodeEdges(n, nil) {
		g.DelEdge(e)
	}

	if g.backend.DelNode(n) {
		g.notifyEvent(graphEvent{element: n, kind: nodeDeleted})
	}
}

func (g *Graph) DelHostGraph(host string) {
	for _, node := range g.GetNodes(Metadata{}) {
		if node.host == host {
			g.DelNode(node)
		}
	}
}

func (g *Graph) GetNodes(m Metadata) []*Node {
	return g.backend.GetNodes(g.context.GetTime(), m)
}

func (g *Graph) GetEdges(m Metadata) []*Edge {
	return g.backend.GetEdges(g.context.GetTime(), m)
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
		Nodes: g.GetNodes(Metadata{}),
		Edges: g.GetEdges(Metadata{}),
	})
}

func (g *Graph) notifyEvent(ge graphEvent) {
	// push event to chan so that nested notification will be sent in the
	// right order. Assiociate the event with the current event listener so
	// we can avoid loop by not triggering event for the current listener.
	ge.listener = g.currentEventListener
	g.eventChan <- ge

	// already a consumer no need to run another consumer
	if g.eventConsumed {
		return
	}
	g.eventConsumed = true

	for len(g.eventChan) > 0 {
		ge = <-g.eventChan

		// notify only once per listener as if more than once we are in a recursion
		// and we wont to notify a listener which generated a graph element
		for _, g.currentEventListener = range g.eventListeners {
			// do not notify the listener which generated the event
			if g.currentEventListener == ge.listener {
				continue
			}

			switch ge.kind {
			case nodeAdded:
				g.currentEventListener.OnNodeAdded(ge.element.(*Node))
			case nodeUpdated:
				g.currentEventListener.OnNodeUpdated(ge.element.(*Node))
			case nodeDeleted:
				g.currentEventListener.OnNodeDeleted(ge.element.(*Node))
			case edgeAdded:
				g.currentEventListener.OnEdgeAdded(ge.element.(*Edge))
			case edgeUpdated:
				g.currentEventListener.OnEdgeUpdated(ge.element.(*Edge))
			case edgeDeleted:
				g.currentEventListener.OnEdgeDeleted(ge.element.(*Edge))
			}
		}
	}
	g.currentEventListener = nil
	g.eventConsumed = false
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

func (g *Graph) WithContext(c GraphContext) (*Graph, error) {
	return g.backend.WithContext(g, c)
}

func (g *Graph) GetContext() GraphContext {
	return g.context
}

func NewGraph(hostID string, backend GraphBackend) *Graph {
	return &Graph{
		backend:   backend,
		host:      hostID,
		context:   GraphContext{},
		eventChan: make(chan graphEvent, maxEvents),
	}
}

func NewGraphFromConfig(backend GraphBackend) *Graph {
	hostID := config.GetConfig().GetString("host_id")
	return NewGraph(hostID, backend)
}

func NewGraphWithContext(hostID string, backend GraphBackend, context GraphContext) (*Graph, error) {
	graph := NewGraph(hostID, backend)
	return graph.WithContext(context)
}

func BackendFromConfig() (backend GraphBackend, err error) {
	cachingMode := Direct
	name := config.GetConfig().GetString("graph.backend")
	if len(name) == 0 {
		name = "memory"
	}

	switch name {
	case "memory":
		backend, err = NewMemoryBackend()
	case "orientdb":
		backend, err = NewOrientDBBackendFromConfig()
		cachingMode = Shadowed
	case "elasticsearch":
		backend, err = NewElasticSearchBackendFromConfig()
		cachingMode = Shadowed
	default:
		return nil, errors.New("Config file is misconfigured, graph backend unknown: " + name)
	}

	if err != nil {
		return nil, err
	}

	switch cachingMode {
	case Shadowed:
		return NewShadowedBackend(backend)
	default:
		return backend, nil
	}
}
