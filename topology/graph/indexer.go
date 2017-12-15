/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"sync"

	"github.com/cnf/structhash"
)

type NodeHasher func(n *Node) string

type GraphIndexer struct {
	sync.Mutex
	*GraphEventHandler
	DefaultGraphListener
	graph       *Graph
	hashNode    NodeHasher
	hashToNodes map[string][]Identifier
	nodeToHash  map[Identifier]string
}

func (i *GraphIndexer) cacheNode(n *Node, h string) {
	i.Lock()

	if _, found := i.hashToNodes[h]; !found {
		i.hashToNodes[h] = []Identifier{}
	}

	if _, found := i.nodeToHash[n.ID]; !found {
		i.nodeToHash[n.ID] = h
		i.hashToNodes[h] = append(i.hashToNodes[h], n.ID)
		i.notifyEvent(graphEvent{element: n, kind: nodeAdded})
	} else {
		i.notifyEvent(graphEvent{element: n, kind: nodeUpdated})
	}

	i.Unlock()
}

func (i *GraphIndexer) forgetNode(n *Node) {
	i.Lock()

	if h := i.nodeToHash[n.ID]; h != "" {
		delete(i.nodeToHash, n.ID)
		for index, id := range i.hashToNodes[h] {
			if id == n.ID {
				if len(i.hashToNodes) == 1 {
					delete(i.hashToNodes, h)
				} else {
					i.hashToNodes[h] = append(i.hashToNodes[h][:index], i.hashToNodes[h][index+1:]...)
				}
				break
			}
		}

		i.notifyEvent(graphEvent{element: n, kind: nodeDeleted})
	}

	i.Unlock()
}

func (i *GraphIndexer) OnNodeAdded(n *Node) {
	if h := i.hashNode(n); h != "" {
		i.cacheNode(n, h)
	}
}

func (i *GraphIndexer) OnNodeUpdated(n *Node) {
	if h := i.hashNode(n); h != "" {
		i.cacheNode(n, h)
	} else {
		i.forgetNode(n)
	}
}

func (i *GraphIndexer) OnNodeDeleted(n *Node) {
	i.forgetNode(n)
}

func (i *GraphIndexer) FromHash(hash string) (nodes []*Node) {
	if ids, found := i.hashToNodes[hash]; found {
		for _, id := range ids {
			nodes = append(nodes, i.graph.GetNode(id))
		}
	}
	return
}

func NewGraphIndexer(g *Graph, hashNode NodeHasher) *GraphIndexer {
	indexer := &GraphIndexer{
		GraphEventHandler: NewGraphEventHandler(maxEvents),
		graph:             g,
		hashNode:          hashNode,
		hashToNodes:       make(map[string][]Identifier),
		nodeToHash:        make(map[Identifier]string),
	}
	g.AddEventListener(indexer)
	return indexer
}

type MetadataIndexer struct {
	*GraphIndexer
	indexes []string
}

func (m *MetadataIndexer) Get(values ...interface{}) []*Node {
	return m.FromHash(m.Hash(values...))
}

func (m *MetadataIndexer) Hash(values ...interface{}) string {
	h, _ := structhash.Hash(values, 1)
	return h
}

func NewMetadataIndexer(g *Graph, m Metadata, indexes ...string) (indexer *MetadataIndexer) {
	indexer = &MetadataIndexer{
		indexes: indexes,
		GraphIndexer: NewGraphIndexer(g, func(n *Node) (hash string) {
			if match := n.MatchMetadata(m); match {
				if len(indexes) == 0 {
					return string(n.ID)
				}
				values := make([]interface{}, len(indexes))
				for i, index := range indexes {
					v, _ := n.GetField(index)
					values[i] = v
				}
				hash = indexer.Hash(values...)
			}
			return
		}),
	}
	return
}
