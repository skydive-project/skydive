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
	"github.com/skydive-project/skydive/filters"
)

const (
	// Namespace used for WebSocket message
	Namespace = "Graph"
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

// Identifier graph ID
type Identifier string

// GraphEventListener describes the graph events interface mechanism
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

// Metadata describes the graph node metadata type
type Metadata map[string]interface{}

// MetadataTransaction describes a metadata(s) transaction in the graph
type MetadataTransaction struct {
	graph        *Graph
	graphElement interface{}
	Metadata     Metadata
}

type graphElement struct {
	ID        Identifier
	metadata  Metadata
	host      string
	createdAt time.Time
	updatedAt time.Time
	deletedAt time.Time
	revision  int64
}

// Node of the graph
type Node struct {
	graphElement
}

// Edge of the graph linked by a parent and a child
type Edge struct {
	graphElement
	parent Identifier
	child  Identifier
}

// GraphBackend interface mechanism used as storage
type GraphBackend interface {
	NodeAdded(n *Node) bool
	NodeDeleted(n *Node) bool
	GetNode(i Identifier, at *common.TimeSlice) []*Node
	GetNodeEdges(n *Node, at *common.TimeSlice, m Metadata) []*Edge

	EdgeAdded(e *Edge) bool
	EdgeDeleted(e *Edge) bool
	GetEdge(i Identifier, at *common.TimeSlice) []*Edge
	GetEdgeNodes(e *Edge, at *common.TimeSlice, parentMetadata, childMetadata Metadata) ([]*Node, []*Node)

	MetadataUpdated(e interface{}) bool

	GetNodes(t *common.TimeSlice, m Metadata) []*Node
	GetEdges(t *common.TimeSlice, m Metadata) []*Edge

	WithContext(graph *Graph, context GraphContext) (*Graph, error)
}

// GraphContext describes within time slice
type GraphContext struct {
	TimeSlice *common.TimeSlice
}

// Graph describes the graph object based on events and context mechanism
// An associated backend is used as storage
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

// HostNodeTIDMap a map of host and node ID
type HostNodeTIDMap map[string][]string

// BuildHostNodeTIDMap creates a map filled with host and associated node.ID
func BuildHostNodeTIDMap(nodes []*Node) HostNodeTIDMap {
	hnmap := make(HostNodeTIDMap)
	for _, node := range nodes {
		if host := node.Host(); host != "" {
			hnmap[host] = append(hnmap[host], string(node.ID))
		}
	}
	return hnmap
}

// DefaultGraphListener default implementation of a graph listener, can be used when not implementing
// the whole set of callbacks
type DefaultGraphListener struct {
}

// OnNodeUpdated event
func (c *DefaultGraphListener) OnNodeUpdated(n *Node) {
}

// OnNodeAdded event
func (c *DefaultGraphListener) OnNodeAdded(n *Node) {
}

// OnNodeDeleted event
func (c *DefaultGraphListener) OnNodeDeleted(n *Node) {
}

// OnEdgeUpdated event
func (c *DefaultGraphListener) OnEdgeUpdated(e *Edge) {
}

// OnEdgeAdded event
func (c *DefaultGraphListener) OnEdgeAdded(e *Edge) {
}

// OnEdgeDeleted event
func (c *DefaultGraphListener) OnEdgeDeleted(e *Edge) {
}

// GenID helper generate a node Identifier
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
	f, err := e.GetField(field)
	if err != nil {
		return 0, err
	}
	return common.ToInt64(f)
}

func (e *graphElement) GetFieldString(field string) (_ string, err error) {
	f, err := e.GetField(field)
	if err != nil {
		return "", err
	}
	s, ok := f.(string)
	if !ok {
		return "", common.ErrFieldNotFound
	}
	return s, nil
}

func (e *graphElement) GetField(name string) (interface{}, error) {
	switch name {
	case "ID":
		return string(e.ID), nil
	case "Host":
		return e.host, nil
	case "CreatedAt":
		return common.UnixMillis(e.createdAt), nil
	case "UpdatedAt":
		return common.UnixMillis(e.updatedAt), nil
	case "DeletedAt":
		return common.UnixMillis(e.deletedAt), nil
	case "Revision":
		return e.revision, nil
	default:
		return common.GetField(e.metadata, name)
	}
}

func (e *graphElement) GetFieldStringList(name string) ([]string, error) {
	v, err := e.GetField(name)
	if err != nil {
		return nil, err
	}

	switch l := v.(type) {
	case []interface{}:
		var l2 []string
		for _, i := range l {
			s, ok := i.(string)
			if !ok {
				return nil, common.ErrFieldWrongType
			}
			l2 = append(l2, s)
		}
		return l2, nil
	case []string:
		return l, nil
	default:
		return nil, common.ErrFieldWrongType
	}
}

// Clone a metadata
func (m Metadata) Clone() Metadata {
	n := Metadata{}

	for k, v := range m {
		n[k] = v
	}

	return n
}

// Metadata returns a copy in order to avoid direct modification of metadata leading in
// loosing notification.
func (e *graphElement) Metadata() Metadata {
	return e.metadata.Clone()
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
			if !ok || !reflect.DeepEqual(nv, v) {
				return false
			}
		}
	}

	return true
}

func parseTime(i interface{}) (t time.Time, err error) {
	var ms int64
	switch i := i.(type) {
	case int64:
		ms = i
	case json.Number:
		ms, err = i.Int64()
		if err != nil {
			return t, err
		}
	default:
		return t, fmt.Errorf("Invalid time: %+v", i)
	}
	return time.Unix(0, ms*int64(time.Millisecond)), err
}

func decodeMap(m map[string]interface{}) {
	for field, value := range m {
		switch v := value.(type) {
		case json.Number:
			var err error
			if value, err = v.Int64(); err != nil {
				if value, err = v.Float64(); err != nil {
					value = v.String()
				}
			}
			m[field] = value
		case map[string]interface{}:
			decodeMap(v)
		default:
			m[field] = value
		}
	}
}

func (e *graphElement) Decode(i interface{}) (err error) {
	objMap, ok := i.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Unable to decode graph element: %v, %+v", i, reflect.TypeOf(i))
	}

	if _, ok = objMap["ID"]; !ok {
		return errors.New("No ID found for graph element")
	}

	id, ok := objMap["ID"].(string)
	if !ok {
		return errors.New("Wrong type for ID")
	}
	e.ID = Identifier(id)

	if _, ok := objMap["Host"]; ok {
		if host, ok := objMap["Host"].(string); ok {
			e.host = host
		} else {
			return errors.New("Wrong type for Host")
		}
	}

	if createdAt, ok := objMap["CreatedAt"]; ok {
		if e.createdAt, err = parseTime(createdAt); err != nil {
			return err
		}
	} else {
		e.createdAt = time.Now().UTC()
	}

	if updatedAt, ok := objMap["UpdatedAt"]; ok {
		if e.updatedAt, err = parseTime(updatedAt); err != nil {
			return err
		}
	} else {
		e.updatedAt = e.createdAt
	}

	if deletedAt, ok := objMap["DeletedAt"]; ok {
		if e.deletedAt, err = parseTime(deletedAt); err != nil {
			return err
		}
	}

	if revision, ok := objMap["Revision"]; ok {
		if r, ok := revision.(json.Number); ok {
			if e.revision, err = r.Int64(); err != nil {
				return errors.New("Wrong type for Revision")
			}
		} else {
			return errors.New("Wrong type for Revision")
		}
	}

	if m, ok := objMap["Metadata"]; ok {
		metadata := m.(map[string]interface{})
		decodeMap(metadata)
		e.metadata = metadata
	}

	return nil
}

func (n *Node) String() string {
	b, err := n.MarshalJSON()
	if err != nil {
		return ""
	}
	return string(b)
}

// MarshalJSON serialize in JSON
func (n *Node) MarshalJSON() ([]byte, error) {
	deletedAt := int64(0)
	if !n.deletedAt.IsZero() {
		deletedAt = common.UnixMillis(n.deletedAt)
	}

	return json.Marshal(&struct {
		ID        Identifier
		Metadata  Metadata `json:",omitempty"`
		Host      string
		CreatedAt int64
		UpdatedAt int64 `json:",omitempty"`
		DeletedAt int64 `json:",omitempty"`
		Revision  int64
	}{
		ID:        n.ID,
		Metadata:  n.metadata,
		Host:      n.host,
		CreatedAt: common.UnixMillis(n.createdAt),
		UpdatedAt: common.UnixMillis(n.updatedAt),
		DeletedAt: deletedAt,
		Revision:  n.revision,
	})
}

// JSONRawMessage creates JSON raw message
func (n *Node) JSONRawMessage() *json.RawMessage {
	r, _ := n.MarshalJSON()
	raw := json.RawMessage(r)
	return &raw
}

// Decode deserialize the node
func (n *Node) Decode(i interface{}) error {
	return n.graphElement.Decode(i)
}

// GetFieldString returns the associated Field name
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

func (e *Edge) String() string {
	b, err := e.MarshalJSON()
	if err != nil {
		return ""
	}
	return string(b)
}

// MarshalJSON serialize in JSON
func (e *Edge) MarshalJSON() ([]byte, error) {
	deletedAt := int64(0)
	if !e.deletedAt.IsZero() {
		deletedAt = common.UnixMillis(e.deletedAt)
	}

	return json.Marshal(&struct {
		ID        Identifier
		Metadata  Metadata `json:",omitempty"`
		Parent    Identifier
		Child     Identifier
		Host      string
		CreatedAt int64
		UpdatedAt int64 `json:",omitempty"`
		DeletedAt int64 `json:",omitempty"`
	}{
		ID:        e.ID,
		Metadata:  e.metadata,
		Parent:    e.parent,
		Child:     e.child,
		Host:      e.host,
		CreatedAt: common.UnixMillis(e.createdAt),
		UpdatedAt: common.UnixMillis(e.updatedAt),
		DeletedAt: deletedAt,
	})
}

// JSONRawMessage creates a JSON raw message
func (e *Edge) JSONRawMessage() *json.RawMessage {
	r, _ := e.MarshalJSON()
	raw := json.RawMessage(r)
	return &raw
}

// Decode deserialize the current edge
func (e *Edge) Decode(i interface{}) error {
	if err := e.graphElement.Decode(i); err != nil {
		return err
	}

	objMap := i.(map[string]interface{})
	id, ok := objMap["Parent"]
	if !ok {
		return errors.New("parent ID missing")
	}
	parentID, ok := id.(string)
	if !ok {
		return errors.New("parent ID wrong format")
	}
	e.parent = Identifier(parentID)

	id, ok = objMap["Child"]
	if !ok {
		return errors.New("child ID missing")
	}
	childID, ok := id.(string)
	if !ok {
		return errors.New("child ID wrong format")
	}
	e.child = Identifier(childID)

	return nil
}

// GetParent returns parent
func (e *Edge) GetParent() Identifier {
	return e.parent
}

// GetChild returns child
func (e *Edge) GetChild() Identifier {
	return e.child
}

// NodeUpdated updates a node
func (g *Graph) NodeUpdated(n *Node) bool {
	if node := g.GetNode(n.ID); node != nil {
		node.metadata = n.metadata
		node.updatedAt = n.updatedAt
		node.revision = n.revision

		if !g.backend.MetadataUpdated(node) {
			return false
		}

		g.notifyEvent(graphEvent{kind: nodeUpdated, element: node})
		return true
	}
	return false
}

// EdgeUpdated updates an edge
func (g *Graph) EdgeUpdated(e *Edge) bool {
	if edge := g.GetEdge(e.ID); edge != nil {
		edge.metadata = e.metadata
		edge.updatedAt = e.updatedAt

		if !g.backend.MetadataUpdated(edge) {
			return false
		}

		g.notifyEvent(graphEvent{kind: edgeUpdated, element: edge})
		return true
	}
	return false
}

// SetMetadata associate metadata to an edge or node
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

	if reflect.DeepEqual(m, e.metadata) {
		return false
	}

	e.metadata = m
	e.updatedAt = time.Now().UTC()
	e.revision++

	if !g.backend.MetadataUpdated(i) {
		return false
	}

	g.notifyEvent(ge)
	return true
}

// DelMetadata delete a metadata to an associated edge or node
func (g *Graph) DelMetadata(i interface{}, k string) bool {
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

	common.DelField(e.metadata, k)

	e.updatedAt = time.Now().UTC()
	e.revision++

	if !g.backend.MetadataUpdated(i) {
		return false
	}

	g.notifyEvent(ge)
	return true
}

func (g *Graph) addMetadata(i interface{}, k string, v interface{}, t time.Time) bool {
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

	if o, ok := e.metadata[k]; ok && reflect.DeepEqual(o, v) {
		return false
	}

	if !common.SetField(e.metadata, k, v) {
		return false
	}

	e.updatedAt = t
	e.revision++

	if !g.backend.MetadataUpdated(i) {
		return false
	}

	g.notifyEvent(ge)
	return true
}

// AddMetadata add a metadata to an associated edge or node
func (g *Graph) AddMetadata(i interface{}, k string, v interface{}) bool {
	return g.addMetadata(i, k, v, time.Now().UTC())
}

// AddMetadata in the current transaction
func (t *MetadataTransaction) AddMetadata(k string, v interface{}) {
	common.SetField(t.Metadata, k, v)
}

// Commit the current transaction to the graph
func (t *MetadataTransaction) Commit() {
	t.graph.SetMetadata(t.graphElement, t.Metadata)
}

// StartMetadataTransaction start a new transaction
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
		Metadata:     make(Metadata),
	}
	for k, v := range e.metadata {
		t.Metadata[k] = v
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

	t := g.context.TimeSlice
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

// LookupShortestPath returns the shortest path (list of node)
func (g *Graph) LookupShortestPath(n *Node, m Metadata, em Metadata) []*Node {
	return g.lookupShortestPath(n, m, []*Node{}, make(map[Identifier]bool), em)
}

// LookupParents returns the associated parents edge of a node
func (g *Graph) LookupParents(n *Node, f Metadata, em Metadata) (nodes []*Node) {
	t := g.context.TimeSlice
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

// LookupFirstChild returns the child
func (g *Graph) LookupFirstChild(n *Node, f Metadata) *Node {
	nodes := g.LookupChildren(n, f, Metadata{})
	if len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}

// LookupChildren returns a list of children nodes
func (g *Graph) LookupChildren(n *Node, f Metadata, em Metadata) (nodes []*Node) {
	t := g.context.TimeSlice
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

// AreLinked returns true if nodes n1, n2 are linked
func (g *Graph) AreLinked(n1 *Node, n2 *Node, m Metadata) bool {
	t := g.context.TimeSlice
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

// Link the nodes n1, n2 with a new edge
func (g *Graph) Link(n1 *Node, n2 *Node, m Metadata) *Edge {
	if len(m) > 0 {
		return g.NewEdge(GenID(), n1, n2, m)
	}
	return g.NewEdge(GenID(), n1, n2, nil)
}

// Unlink the nodes n1, n2 ; delete the associated edge
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

// LookupFirstNode returns the fist node matching metadata
func (g *Graph) LookupFirstNode(m Metadata) *Node {
	nodes := g.GetNodes(m)
	if len(nodes) > 0 {
		return nodes[0]
	}

	return nil
}

// EdgeAdded add an edge
func (g *Graph) EdgeAdded(e *Edge) bool {
	if g.GetEdge(e.ID) == nil {
		return g.AddEdge(e)
	}
	return false
}

// AddEdge in the graph
func (g *Graph) AddEdge(e *Edge) bool {
	if !g.backend.EdgeAdded(e) {
		return false
	}
	g.notifyEvent(graphEvent{element: e, kind: edgeAdded})

	return true
}

// GetEdge with Identifier i
func (g *Graph) GetEdge(i Identifier) *Edge {
	if edges := g.backend.GetEdge(i, g.context.TimeSlice); len(edges) != 0 {
		return edges[0]
	}
	return nil
}

// NodeAdded in the graph
func (g *Graph) NodeAdded(n *Node) bool {
	if g.GetNode(n.ID) == nil {
		return g.AddNode(n)
	}
	return false
}

// AddNode in the graph
func (g *Graph) AddNode(n *Node) bool {
	if !g.backend.NodeAdded(n) {
		return false
	}
	g.notifyEvent(graphEvent{element: n, kind: nodeAdded})

	return true
}

// GetNode from Identifier
func (g *Graph) GetNode(i Identifier) *Node {
	if nodes := g.backend.GetNode(i, g.context.TimeSlice); len(nodes) != 0 {
		return nodes[0]
	}
	return nil
}

func newNode(i Identifier, m Metadata, t time.Time, h string) *Node {
	n := &Node{
		graphElement: graphElement{
			ID:        i,
			host:      h,
			createdAt: t,
			updatedAt: t,
			revision:  1,
		},
	}

	if m != nil {
		n.metadata = m
	} else {
		n.metadata = make(Metadata)
	}

	return n
}

func (g *Graph) newNode(i Identifier, m Metadata, t time.Time, h ...string) *Node {
	hostname := g.host
	if len(h) > 0 {
		hostname = h[0]
	}

	n := newNode(i, m, t, hostname)

	if !g.AddNode(n) {
		return nil
	}

	return n
}

// NewNode creates a new node in the graph with attached metadata
func (g *Graph) NewNode(i Identifier, m Metadata, h ...string) *Node {
	return g.newNode(i, m, time.Now().UTC(), h...)
}

func newEdge(i Identifier, p *Node, c *Node, m Metadata, t time.Time, h string) *Edge {
	e := &Edge{
		parent: p.ID,
		child:  c.ID,
		graphElement: graphElement{
			ID:        i,
			host:      h,
			createdAt: t,
			updatedAt: t,
			revision:  1,
		},
	}

	if m != nil {
		e.metadata = m
	} else {
		e.metadata = make(Metadata)
	}

	return e
}

func (g *Graph) newEdge(i Identifier, p *Node, c *Node, m Metadata, t time.Time, h ...string) *Edge {
	hostname := g.host
	if len(h) > 0 {
		hostname = h[0]
	}

	e := newEdge(i, p, c, m, t, hostname)

	if !g.AddEdge(e) {
		return nil
	}

	return e
}

// NewEdge creates a new edge in the graph based on Identifier, parent, child nodes and metadata
func (g *Graph) NewEdge(i Identifier, p *Node, c *Node, m Metadata) *Edge {
	return g.newEdge(i, p, c, m, time.Now().UTC())
}

// EdgeDeleted event
func (g *Graph) EdgeDeleted(e *Edge) {
	if g.backend.EdgeDeleted(e) {
		g.notifyEvent(graphEvent{element: e, kind: edgeDeleted})
	}
}

func (g *Graph) delEdge(e *Edge, t time.Time) {
	e.deletedAt = t
	if g.backend.EdgeDeleted(e) {
		g.notifyEvent(graphEvent{element: e, kind: edgeDeleted})
	}
}

// DelEdge delete an edge
func (g *Graph) DelEdge(e *Edge) {
	g.delEdge(e, time.Now().UTC())
}

// NodeDeleted event
func (g *Graph) NodeDeleted(n *Node) {
	if g.backend.NodeDeleted(n) {
		g.notifyEvent(graphEvent{element: n, kind: nodeDeleted})
	}
}

func (g *Graph) delNode(n *Node, t time.Time) {
	for _, e := range g.backend.GetNodeEdges(n, nil, Metadata{}) {
		g.delEdge(e, t)
	}

	n.deletedAt = t
	if g.backend.NodeDeleted(n) {
		g.notifyEvent(graphEvent{element: n, kind: nodeDeleted})
	}
}

// DelNode delete the node n in the graph
func (g *Graph) DelNode(n *Node) {
	g.delNode(n, time.Now().UTC())
}

// DelHostGraph delete the associated node with the hostname host
func (g *Graph) DelHostGraph(host string) {
	t := time.Now().UTC()
	for _, node := range g.GetNodes(Metadata{}) {
		if node.host == host {
			g.delNode(node, t)
		}
	}
}

// GetNodes returns a list of nodes
func (g *Graph) GetNodes(m Metadata) []*Node {
	return g.backend.GetNodes(g.context.TimeSlice, m)
}

// GetEdges returns a list of edges
func (g *Graph) GetEdges(m Metadata) []*Edge {
	return g.backend.GetEdges(g.context.TimeSlice, m)
}

// GetEdgeNodes returns a list of nodes of an edge
func (g *Graph) GetEdgeNodes(e *Edge, parentMetadata, childMetadata Metadata) ([]*Node, []*Node) {
	return g.backend.GetEdgeNodes(e, g.context.TimeSlice, parentMetadata, childMetadata)
}

// GetNodeEdges returns a list of edges of a node
func (g *Graph) GetNodeEdges(n *Node, m Metadata) []*Edge {
	return g.backend.GetNodeEdges(n, g.context.TimeSlice, m)
}

func (g *Graph) String() string {
	j, _ := json.Marshal(g)
	return string(j)
}

// MarshalJSON serialize the graph in JSON
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

// AddEventListener subscibe a new graph listener
func (g *Graph) AddEventListener(l GraphEventListener) {
	g.Lock()
	defer g.Unlock()

	g.eventListeners = append(g.eventListeners, l)
}

// RemoveEventListener unsubscribe a graph listener
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

// WithContext select a graph within a context
func (g *Graph) WithContext(c GraphContext) (*Graph, error) {
	return g.backend.WithContext(g, c)
}

// GetContext returns the current context
func (g *Graph) GetContext() GraphContext {
	return g.context
}

// GetHost returns the graph host
func (g *Graph) GetHost() string {
	return g.host
}

// NewGraph creates a new graph based on the backend
func NewGraph(host string, backend GraphBackend) *Graph {
	return &Graph{
		backend:   backend,
		host:      host,
		context:   GraphContext{},
		eventChan: make(chan graphEvent, maxEvents),
	}
}

// NewGraphFromConfig creates a new graph based on configuration
func NewGraphFromConfig(backend GraphBackend) *Graph {
	host := config.GetConfig().GetString("host_id")
	return NewGraph(host, backend)
}

// NewGraphWithContext creates a new graph based on backedn within the context
func NewGraphWithContext(hostID string, backend GraphBackend, context GraphContext) (*Graph, error) {
	graph := NewGraph(hostID, backend)
	return graph.WithContext(context)
}

// BackendFromConfig creates a new graph backend based on configuration
// memory, orientdb, elasticsearch backend are supported
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
