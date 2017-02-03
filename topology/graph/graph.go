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
	"strings"
	"sync"
	"time"

	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
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
	ID        Identifier
	metadata  Metadata
	host      string
	createdAt time.Time
	deletedAt time.Time
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
	GetNode(i Identifier, at *common.TimeSlice) []*Node
	GetNodeEdges(n *Node, at *common.TimeSlice, m Metadata) []*Edge

	AddEdge(e *Edge) bool
	DelEdge(e *Edge) bool
	GetEdge(i Identifier, at *common.TimeSlice) []*Edge
	GetEdgeNodes(e *Edge, at *common.TimeSlice, parentMetadata, childMetadata Metadata) ([]*Node, []*Node)

	AddMetadata(e interface{}, k string, v interface{}) bool
	SetMetadata(e interface{}, m Metadata) bool

	GetNodes(t *common.TimeSlice, m Metadata) []*Node
	GetEdges(t *common.TimeSlice, m Metadata) []*Edge

	WithContext(graph *Graph, context GraphContext) (*Graph, error)
}

type GraphContext struct {
	TimeSlice *common.TimeSlice
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

func (e *graphElement) GetFieldInt64(field string) (_ int64, err error) {
	f, found := e.GetField(field)
	if !found {
		return 0, common.ErrFieldNotFound
	}
	return common.ToInt64(f)
}

func (e *graphElement) GetFieldString(field string) (_ string, err error) {
	f, found := e.GetField(field)
	if !found {
		return "", common.ErrFieldNotFound
	}
	s, ok := f.(string)
	if !ok {
		return "", common.ErrFieldNotFound
	}
	return s, nil
}

func (e *graphElement) GetField(name string) (interface{}, bool) {
	switch name {
	case "ID":
		return string(e.ID), true
	case "Host":
		return e.host, true
	case "CreatedAt":
		return e.createdAt.Unix(), true
	case "DeletedAt":
		return e.deletedAt.Unix(), true
	default:
		if strings.HasPrefix(name, "Metadata/") {
			name = name[9:]
		}
		v, ok := e.Metadata()[name]
		return v, ok
	}
}

func (e *graphElement) Metadata() Metadata {
	return e.metadata
}

func (e *graphElement) MatchMetadata(f Metadata) bool {
	for k, v := range f {
		switch v := v.(type) {
		case *filters.Filter:
			if !v.Eval(e) {
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
	deletedAt := ""
	if !e.deletedAt.IsZero() {
		deletedAt = e.deletedAt.String()
	}

	j, _ := json.Marshal(&struct {
		ID        Identifier
		Metadata  Metadata `json:",omitempty"`
		Host      string
		CreatedAt string
		DeletedAt string `json:",omitempty"`
	}{
		ID:        e.ID,
		Metadata:  e.metadata,
		Host:      e.host,
		CreatedAt: e.createdAt.String(),
		DeletedAt: deletedAt,
	})
	return string(j)
}

func parseTime(i interface{}) (t time.Time, err error) {
	var epoch int64
	switch i := i.(type) {
	case int64:
		epoch = i
	case json.Number:
		epoch, err = i.Int64()
		if err != nil {
			return t, err
		}
	default:
		return t, fmt.Errorf("Invalid time: %+v", i)
	}
	return time.Unix(epoch, 0), err
}

func (e *graphElement) Decode(i interface{}) (err error) {
	objMap, ok := i.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Unable to decode graph element: %v, %+v", i, reflect.TypeOf(i))
	}

	e.ID = Identifier(objMap["ID"].(string))
	e.host = objMap["Host"].(string)

	if createdAt, ok := objMap["CreatedAt"]; ok {
		if e.createdAt, err = parseTime(createdAt); err != nil {
			return err
		}
	}

	if deletedAt, ok := objMap["DeletedAt"]; ok {
		if e.deletedAt, err = parseTime(deletedAt); err != nil {
			return err
		}
	}

	if m, ok := objMap["Metadata"]; ok {
		e.metadata = make(Metadata)
		for field, value := range m.(map[string]interface{}) {
			if n, ok := value.(json.Number); ok {
				if value, err = n.Int64(); err == nil {
					value = value.(int64)
				} else {
					value, _ = n.Float64()
				}
			}
			e.metadata[field] = value
		}
	}

	return nil
}

func (n *Node) MarshalJSON() ([]byte, error) {
	deletedAt := int64(0)
	if !n.deletedAt.IsZero() {
		deletedAt = n.deletedAt.Unix()
	}

	return json.Marshal(&struct {
		ID        Identifier
		Metadata  Metadata `json:",omitempty"`
		Host      string
		CreatedAt int64
		DeletedAt int64 `json:",omitempty"`
	}{
		ID:        n.ID,
		Metadata:  n.metadata,
		Host:      n.host,
		CreatedAt: n.createdAt.Unix(),
		DeletedAt: deletedAt,
	})
}

func (n *Node) JsonRawMessage() *json.RawMessage {
	r, _ := n.MarshalJSON()
	raw := json.RawMessage(r)
	return &raw
}

func (n *Node) Decode(i interface{}) error {
	return n.graphElement.Decode(i)
}

func (e *Edge) GetFieldString(name string) (string, error) {
	switch name {
	case "Parent":
		return string(e.parent), nil
	case "Child":
		return string(e.child), nil
	default:
		return e.graphElement.GetFieldString(name)
	}
}

func (e *Edge) MarshalJSON() ([]byte, error) {
	deletedAt := int64(0)
	if !e.deletedAt.IsZero() {
		deletedAt = e.deletedAt.Unix()
	}

	return json.Marshal(&struct {
		ID        Identifier
		Metadata  Metadata `json:",omitempty"`
		Parent    Identifier
		Child     Identifier
		Host      string
		CreatedAt int64
		DeletedAt int64 `json:",omitempty"`
	}{
		ID:        e.ID,
		Metadata:  e.metadata,
		Parent:    e.parent,
		Child:     e.child,
		Host:      e.host,
		CreatedAt: e.createdAt.Unix(),
		DeletedAt: deletedAt,
	})
}

func (e *Edge) JsonRawMessage() *json.RawMessage {
	r, _ := e.MarshalJSON()
	raw := json.RawMessage(r)
	return &raw
}

func (e *Edge) Decode(i interface{}) error {
	if err := e.graphElement.Decode(i); err != nil {
		return err
	}

	objMap := i.(map[string]interface{})
	e.parent = Identifier(objMap["Parent"].(string))
	e.child = Identifier(objMap["Child"].(string))
	return nil
}

func (e *Edge) GetParent() Identifier {
	return e.parent
}

func (e *Edge) GetChild() Identifier {
	return e.child
}

func (c *GraphContext) GetTimeSlice() *common.TimeSlice {
	return c.TimeSlice
}

func (g *Graph) SetMetadata(i interface{}, m Metadata) bool {
	var e *graphElement
	ge := graphEvent{element: i}

	switch i := i.(type) {
	case *Node:
		e = &i.graphElement
		ge.kind = nodeUpdated
	case *Edge:
		e = &i.graphElement
		ge.kind = edgeUpdated
	}

	if len(m) == len(e.metadata) {
		unchanged := true
		for k, v := range m {
			if e.metadata[k] != v {
				unchanged = false
				break
			}
		}
		if unchanged {
			return false
		}
	}

	if !g.backend.SetMetadata(i, m) {
		return false
	}

	e.metadata = m

	g.notifyEvent(ge)
	return true
}

func (g *Graph) AddMetadata(i interface{}, k string, v interface{}) bool {
	var e *graphElement
	ge := graphEvent{element: i}

	switch i.(type) {
	case *Node:
		e = &i.(*Node).graphElement
		ge.kind = nodeUpdated
	case *Edge:
		e = &i.(*Edge).graphElement
		ge.kind = edgeUpdated
	}

	if o, ok := e.metadata[k]; ok && o == v {
		return false
	}

	if !g.backend.AddMetadata(i, k, v) {
		return false
	}

	e.metadata[k] = v

	g.notifyEvent(ge)
	return true
}

func (g *Graph) DelMetadata(i interface{}, k string) bool {
	var m Metadata
	ge := graphEvent{element: i}

	switch i.(type) {
	case *Node:
		m = i.(*Node).graphElement.metadata
		ge.kind = nodeUpdated
	case *Edge:
		m = i.(*Edge).graphElement.metadata
		ge.kind = edgeUpdated
	}

	if _, ok := m[k]; !ok {
		return false
	}

	if !g.backend.SetMetadata(i, m) {
		return false
	}

	delete(m, k)

	g.notifyEvent(ge)
	return true
}

func (t *MetadataTransaction) AddMetadata(k string, v interface{}) {
	t.metadata[k] = v
}

func (t *MetadataTransaction) DelMetadata(k string, v interface{}) {
	delete(t.metadata, k)
}

func (t *MetadataTransaction) Metadata() Metadata {
	return t.metadata
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

func (g *Graph) lookupShortestPath(n *Node, m Metadata, path []*Node, v map[Identifier]bool, em Metadata) []*Node {
	v[n.ID] = true

	newPath := make([]*Node, len(path)+1)
	copy(newPath, path)
	newPath[len(path)] = n

	if n.MatchMetadata(m) {
		return newPath
	}

	t := g.context.GetTimeSlice()
	shortest := []*Node{}
	for _, e := range g.backend.GetNodeEdges(n, t, em) {
		parents, children := g.backend.GetEdgeNodes(e, t, nil, nil)
		if len(parents) == 0 || len(children) == 0 {
			continue
		}

		parent, child := parents[0], children[0]
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

			sub := g.lookupShortestPath(neighbor, m, newPath, nv, em)
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

func (g *Graph) LookupShortestPath(n *Node, m Metadata, em Metadata) []*Node {
	return g.lookupShortestPath(n, m, []*Node{}, make(map[Identifier]bool), em)
}

func (g *Graph) LookupParents(n *Node, f Metadata, em Metadata) (nodes []*Node) {
	t := g.context.GetTimeSlice()
	for _, e := range g.backend.GetNodeEdges(n, t, em) {
		if e.GetChild() == n.ID {
			parents, _ := g.backend.GetEdgeNodes(e, t, f, Metadata{})
			for _, parent := range parents {
				nodes = append(nodes, parent)
			}
		}
	}

	return
}

func (g *Graph) LookupFirstChild(n *Node, f Metadata) *Node {
	nodes := g.LookupChildren(n, f, Metadata{})
	if len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}

func (g *Graph) LookupChildren(n *Node, f Metadata, em Metadata) (nodes []*Node) {
	t := g.context.GetTimeSlice()
	for _, e := range g.backend.GetNodeEdges(n, t, em) {
		if e.GetParent() == n.ID {
			_, children := g.backend.GetEdgeNodes(e, t, Metadata{}, f)
			for _, child := range children {
				nodes = append(nodes, child)
			}
		}
	}

	return nodes
}

func (g *Graph) AreLinked(n1 *Node, n2 *Node, m Metadata) bool {
	t := g.context.GetTimeSlice()
	for _, e := range g.backend.GetNodeEdges(n1, t, m) {
		parents, children := g.backend.GetEdgeNodes(e, t, Metadata{}, Metadata{})
		if len(parents) == 0 || len(children) == 0 {
			continue
		}

		for i, parent := range parents {
			if children[i].ID == n2.ID || parent.ID == n2.ID {
				return true
			}
		}
	}

	return false
}

func (g *Graph) Link(n1 *Node, n2 *Node, m Metadata) *Edge {
	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(string(n1.ID)+string(n2.ID)))

	if len(m) > 0 {
		return g.NewEdge(Identifier(u.String()), n1, n2, m)
	}
	return g.NewEdge(Identifier(u.String()), n1, n2, nil)
}

func (g *Graph) Unlink(n1 *Node, n2 *Node) {
	for _, e := range g.backend.GetNodeEdges(n1, nil, Metadata{}) {
		parents, children := g.backend.GetEdgeNodes(e, nil, Metadata{}, Metadata{})
		if len(parents) == 0 || len(children) == 0 {
			continue
		}

		parent, child := parents[0], children[0]
		if child.ID == n2.ID || parent.ID == n2.ID {
			g.DelEdge(e)
		}
	}
}

func (g *Graph) Replace(o *Node, n *Node) *Node {
	for _, e := range g.backend.GetNodeEdges(o, nil, Metadata{}) {
		parents, children := g.backend.GetEdgeNodes(e, nil, Metadata{}, Metadata{})
		if len(parents) == 0 || len(children) == 0 {
			continue
		}
		parent, child := parents[0], children[0]

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

func (g *Graph) AddEdge(e *Edge) bool {
	if !g.backend.AddEdge(e) {
		return false
	}
	g.notifyEvent(graphEvent{element: e, kind: edgeAdded})

	return true
}

func (g *Graph) GetEdge(i Identifier) *Edge {
	if edges := g.backend.GetEdge(i, g.context.GetTimeSlice()); len(edges) != 0 {
		return edges[0]
	}
	return nil
}

func (g *Graph) AddNode(n *Node) bool {
	if !g.backend.AddNode(n) {
		return false
	}
	g.notifyEvent(graphEvent{element: n, kind: nodeAdded})

	return true
}

func (g *Graph) GetNode(i Identifier) *Node {
	if nodes := g.backend.GetNode(i, g.context.GetTimeSlice()); len(nodes) != 0 {
		return nodes[0]
	}
	return nil
}

func (g *Graph) NewNode(i Identifier, m Metadata, h ...string) *Node {
	hostname := g.host
	if len(h) > 0 {
		hostname = h[0]
	}
	n := &Node{
		graphElement: graphElement{
			ID:        i,
			host:      hostname,
			createdAt: time.Now().UTC(),
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
			ID:        i,
			host:      g.host,
			createdAt: time.Now().UTC(),
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
		e.deletedAt = time.Now().UTC()
		g.notifyEvent(graphEvent{element: e, kind: edgeDeleted})
	}
}

func (g *Graph) DelNode(n *Node) {
	for _, e := range g.backend.GetNodeEdges(n, nil, Metadata{}) {
		g.DelEdge(e)
	}

	if g.backend.DelNode(n) {
		n.deletedAt = time.Now().UTC()
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
	return g.backend.GetNodes(g.context.GetTimeSlice(), m)
}

func (g *Graph) GetEdges(m Metadata) []*Edge {
	return g.backend.GetEdges(g.context.GetTimeSlice(), m)
}

func (g *Graph) GetEdgeNodes(e *Edge, parentMetadata, childMetadata Metadata) ([]*Node, []*Node) {
	return g.backend.GetEdgeNodes(e, g.context.GetTimeSlice(), parentMetadata, childMetadata)
}

func (g *Graph) GetNodeEdges(n *Node, m Metadata) []*Edge {
	return g.backend.GetNodeEdges(n, g.context.GetTimeSlice(), m)
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

func (g *Graph) GetHost() string {
	return g.host
}

func NewGraph(host string, backend GraphBackend) *Graph {
	return &Graph{
		backend:   backend,
		host:      host,
		context:   GraphContext{},
		eventChan: make(chan graphEvent, maxEvents),
	}
}

func NewGraphFromConfig(backend GraphBackend) *Graph {
	host := config.GetConfig().GetString("host_id")
	return NewGraph(host, backend)
}

func NewGraphWithContext(hostID string, backend GraphBackend, context GraphContext) (*Graph, error) {
	graph := NewGraph(hostID, backend)
	return graph.WithContext(context)
}

func BackendFromConfig() (backend GraphBackend, err error) {
	name := config.GetConfig().GetString("graph.backend")
	if len(name) == 0 {
		name = "memory"
	}

	switch name {
	case "memory":
		backend, err = NewMemoryBackend()
	case "orientdb":
		backend, err = NewOrientDBBackendFromConfig()
	case "elasticsearch":
		backend, err = NewElasticSearchBackendFromConfig()
	default:
		return nil, errors.New("Config file is misconfigured, graph backend unknown: " + name)
	}

	if err != nil {
		return nil, err
	}
	return backend, nil
}
