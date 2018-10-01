/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"reflect"
	"time"

	"github.com/skydive-project/skydive/filters"
)

type Linker interface {
	GetABLinks(node *Node) []*Edge
	GetBALinks(node *Node) []*Edge
}

type listener struct {
	DefaultGraphListener
	graph             *Graph
	newLinksFunc      func(node *Node) []*Edge
	existingLinksFunc func(node *Node) []*Edge
	metadata          Metadata
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
			newLink.metadata[k] = v
		}

		if oldLink, found := existingLinks[id]; !found {
			l.graph.AddEdge(newLink)
		} else {
			if !reflect.DeepEqual(newLink.metadata, oldLink.metadata) {
				l.graph.SetMetadata(oldLink, newLink.metadata)
			}
			delete(existingLinks, id)
		}
	}

	for _, oldLink := range existingLinks {
		l.graph.DelEdge(oldLink)
	}
}

func (l *listener) OnNodeAdded(node *Node) {
	l.nodeEvent(node)
}

func (l *listener) OnNodeUpdated(node *Node) {
	l.nodeEvent(node)
}

type DefaultLinker struct {
}

func (dl *DefaultLinker) GetABLinks(node *Node) []*Edge {
	return nil
}

func (dl *DefaultLinker) GetBALinks(node *Node) []*Edge {
	return nil
}

type ResourceLinker struct {
	g          *Graph
	abListener *listener
	baListener *listener
	glh1       GraphListenerHandler
	glh2       GraphListenerHandler
	linker     Linker
	metadata   Metadata
}

func (rl *ResourceLinker) getLinks(node *Node, direction string) []*Edge {
	metadata := Metadata{}
	for k, v := range rl.metadata {
		metadata[k] = v
	}
	metadata[direction] = string(node.ID)
	return rl.g.GetNodeEdges(node, metadata)
}

func (rl *ResourceLinker) Start() {
	if rl.glh1 != nil {
		rl.abListener = &listener{
			graph:        rl.g,
			newLinksFunc: rl.linker.GetABLinks,
			existingLinksFunc: func(node *Node) (edges []*Edge) {
				return rl.getLinks(node, "Parent")
			},
			metadata: rl.metadata,
		}
		rl.glh1.AddEventListener(rl.abListener)
	}

	if rl.glh2 != nil {
		rl.baListener = &listener{
			graph:        rl.g,
			newLinksFunc: rl.linker.GetBALinks,
			existingLinksFunc: func(node *Node) (edges []*Edge) {
				return rl.getLinks(node, "Child")
			},
			metadata: rl.metadata,
		}
		rl.glh2.AddEventListener(rl.baListener)
	}
}

func (rl *ResourceLinker) Stop() {
	if rl.glh1 != nil {
		rl.glh1.RemoveEventListener(rl.abListener)
	}

	if rl.glh2 != nil {
		rl.glh2.RemoveEventListener(rl.baListener)
	}
}

func NewResourceLinker(g *Graph, glh1 GraphListenerHandler, glh2 GraphListenerHandler, linker Linker, m Metadata) *ResourceLinker {
	return &ResourceLinker{
		g:        g,
		glh1:     glh1,
		glh2:     glh2,
		linker:   linker,
		metadata: m,
	}
}

// getFieldsAsArray returns an array of corresponding values from a field list
func getFieldsAsArray(obj filters.Getter, fields []string) ([]interface{}, error) {
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

func (mil *MetadataIndexerLinker) GetABLinks(node *Node) (edges []*Edge) {
	if fields, err := getFieldsAsArray(node, mil.indexer1.indexes); err == nil {
		nodes, _ := mil.indexer2.Get(fields...)
		for _, n := range nodes {
			edges = append(edges, mil.g.CreateEdge(mil.genID(node, n), node, n, mil.edgeMetadata, time.Now(), ""))
		}
	}
	return
}

func (mil *MetadataIndexerLinker) GetBALinks(node *Node) (edges []*Edge) {
	if fields, err := getFieldsAsArray(node, mil.indexer2.indexes); err == nil {
		nodes, _ := mil.indexer1.Get(fields...)
		for _, n := range nodes {
			edges = append(edges, mil.g.CreateEdge(mil.genID(n, node), n, node, mil.edgeMetadata, time.Now(), ""))
		}
	}
	return
}

func NewMetadataIndexerLinker(g *Graph, indexer1, indexer2 *MetadataIndexer, edgeMetadata Metadata) *MetadataIndexerLinker {
	mil := &MetadataIndexerLinker{
		indexer1: indexer1,
		indexer2: indexer2,
	}

	mil.ResourceLinker = NewResourceLinker(g, indexer1, indexer2, mil, edgeMetadata)
	return mil
}
