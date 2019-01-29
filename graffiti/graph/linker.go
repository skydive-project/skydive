/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package graph

import (
	"reflect"

	"github.com/skydive-project/skydive/common"
)

// Linker describes an object that returns incoming edges to a node
// and outgoing edges from that node
type Linker interface {
	GetABLinks(node *Node) []*Edge
	GetBALinks(node *Node) []*Edge
}

// LinkerEventListener defines the event interface for linker
type LinkerEventListener interface {
	OnError(err error)
}

type listener struct {
	DefaultGraphListener
	graph             *Graph
	newLinksFunc      func(node *Node) []*Edge
	existingLinksFunc func(node *Node) []*Edge
	metadata          Metadata
	links             map[Identifier]bool
	resourceLinker    *ResourceLinker
}

func mapOfLinks(edges []*Edge) map[Identifier]*Edge {
	m := make(map[Identifier]*Edge)
	for _, edge := range edges {
		m[edge.ID] = edge
	}
	return m
}

func (l *listener) nodeEvent(node *Node) {
	newLinks := mapOfLinks(l.newLinksFunc(node))
	existingLinks := mapOfLinks(l.existingLinksFunc(node))

	for id, newLink := range newLinks {
		for k, v := range l.metadata {
			newLink.Metadata[k] = v
		}

		if oldLink, found := existingLinks[id]; !found {
			if err := l.graph.AddEdge(newLink); err != nil {
				l.resourceLinker.notifyError(err)
			} else {
				l.links[newLink.ID] = true
			}
		} else {
			if !reflect.DeepEqual(newLink.Metadata, oldLink.Metadata) {
				if err := l.graph.SetMetadata(oldLink, newLink.Metadata); err != nil {
					l.resourceLinker.notifyError(err)
				}
			}
			delete(existingLinks, id)
		}
	}

	for _, oldLink := range existingLinks {
		if _, found := l.links[oldLink.ID]; found {
			if err := l.graph.DelEdge(oldLink); err != nil {
				l.resourceLinker.notifyError(err)
			}
			delete(l.links, oldLink.ID)
		}
	}
}

func (l *listener) OnNodeAdded(node *Node) {
	l.nodeEvent(node)
}

func (l *listener) OnNodeUpdated(node *Node) {
	l.nodeEvent(node)
}

// DefaultLinker returns a linker that does nothing
type DefaultLinker struct {
}

// GetABLinks returns all the outgoing links for a node
func (dl *DefaultLinker) GetABLinks(node *Node) []*Edge {
	return nil
}

// GetBALinks returns all the incoming links for a node
func (dl *DefaultLinker) GetBALinks(node *Node) []*Edge {
	return nil
}

// ResourceLinker returns a resource linker. It listens for events from
// 2 graph events sources to determine if resources from one source should be
// linked with resources of the other source.
type ResourceLinker struct {
	common.RWMutex
	g              *Graph
	abListener     *listener
	baListener     *listener
	glhs1          []ListenerHandler
	glhs2          []ListenerHandler
	linker         Linker
	metadata       Metadata
	links          map[Identifier]bool
	eventListeners []LinkerEventListener
}

func (rl *ResourceLinker) getLinks(node *Node, direction string) []*Edge {
	metadata := Metadata{}
	for k, v := range rl.metadata {
		metadata[k] = v
	}
	metadata[direction] = string(node.ID)
	return rl.g.GetNodeEdges(node, metadata)
}

// Start linking resources by listening for graph events
func (rl *ResourceLinker) Start() {
	links := make(map[Identifier]bool)

	if len(rl.glhs1) > 0 {
		rl.abListener = &listener{
			graph:        rl.g,
			newLinksFunc: rl.linker.GetABLinks,
			existingLinksFunc: func(node *Node) (edges []*Edge) {
				return rl.getLinks(node, "Parent")
			},
			metadata:       rl.metadata,
			links:          links,
			resourceLinker: rl,
		}
		for _, handler := range rl.glhs1 {
			handler.AddEventListener(rl.abListener)
		}
	}

	if len(rl.glhs2) > 0 {
		rl.baListener = &listener{
			graph:        rl.g,
			newLinksFunc: rl.linker.GetBALinks,
			existingLinksFunc: func(node *Node) (edges []*Edge) {
				return rl.getLinks(node, "Child")
			},
			metadata:       rl.metadata,
			links:          links,
			resourceLinker: rl,
		}
		for _, handler := range rl.glhs2 {
			handler.AddEventListener(rl.baListener)
		}
	}
}

// Stop linking resources
func (rl *ResourceLinker) Stop() {
	for _, handler := range rl.glhs1 {
		handler.RemoveEventListener(rl.abListener)
	}

	for _, handler := range rl.glhs2 {
		handler.RemoveEventListener(rl.baListener)
	}
}

func (rl *ResourceLinker) notifyError(err error) {
	rl.RLock()
	defer rl.RUnlock()

	for _, l := range rl.eventListeners {
		l.OnError(err)
	}
}

// AddEventListener subscribe a new linker listener
func (rl *ResourceLinker) AddEventListener(l LinkerEventListener) {
	rl.Lock()
	defer rl.Unlock()

	rl.eventListeners = append(rl.eventListeners, l)
}

// RemoveEventListener unsubscribe a linker listener
func (rl *ResourceLinker) RemoveEventListener(l LinkerEventListener) {
	rl.Lock()
	defer rl.Unlock()

	for i, el := range rl.eventListeners {
		if l == el {
			rl.eventListeners = append(rl.eventListeners[:i], rl.eventListeners[i+1:]...)
			break
		}
	}
}

// NewResourceLinker returns a new resource linker
func NewResourceLinker(g *Graph, glhs1 []ListenerHandler, glhs2 []ListenerHandler, linker Linker, m Metadata) *ResourceLinker {
	return &ResourceLinker{
		g:        g,
		glhs1:    glhs1,
		glhs2:    glhs2,
		linker:   linker,
		metadata: m,
	}
}

// getFieldsAsArray returns an array of corresponding values from a field list
func getFieldsAsArray(obj common.Getter, fields []string) ([]interface{}, error) {
	values := make([]interface{}, len(fields))
	for i, index := range fields {
		v, err := obj.GetField(index)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
}

// MetadataIndexerLinker describes an object that links resources from one indexer
// to resources from an other indexer.
type MetadataIndexerLinker struct {
	*ResourceLinker
	indexer1     *MetadataIndexer
	indexer2     *MetadataIndexer
	edgeMetadata Metadata
}

func (mil *MetadataIndexerLinker) genID(parent, child *Node) Identifier {
	args := []string{string(parent.ID), string(child.ID)}
	for k, v := range mil.edgeMetadata {
		args = append(args, k, v.(string))
	}
	return GenID(args...)
}

func (mil *MetadataIndexerLinker) createEdge(node1, node2 *Node) *Edge {
	return mil.g.CreateEdge(mil.genID(node1, node2), node1, node2, mil.edgeMetadata, TimeUTC(), "")
}

// GetABLinks returns all the outgoing links for a node
func (mil *MetadataIndexerLinker) GetABLinks(node *Node) (edges []*Edge) {
	if fields, err := getFieldsAsArray(node, mil.indexer1.indexes); err == nil {
		nodes, _ := mil.indexer2.Get(fields...)
		for _, n := range nodes {
			edges = append(edges, mil.createEdge(node, n))
		}
	}
	return
}

// GetBALinks returns all the incoming links for a node
func (mil *MetadataIndexerLinker) GetBALinks(node *Node) (edges []*Edge) {
	if fields, err := getFieldsAsArray(node, mil.indexer2.indexes); err == nil {
		nodes, _ := mil.indexer1.Get(fields...)
		for _, n := range nodes {
			edges = append(edges, mil.createEdge(n, node))
		}
	}
	return
}

// NewMetadataIndexerLinker returns a new metadata based linker
func NewMetadataIndexerLinker(g *Graph, indexer1, indexer2 *MetadataIndexer, edgeMetadata Metadata) *MetadataIndexerLinker {
	mil := &MetadataIndexerLinker{
		indexer1: indexer1,
		indexer2: indexer2,
	}

	mil.ResourceLinker = NewResourceLinker(g, []ListenerHandler{indexer1}, []ListenerHandler{indexer2}, mil, edgeMetadata)
	return mil
}
