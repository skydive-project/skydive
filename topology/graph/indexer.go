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

// NodeHasher describes a callback that is called to map a node to
// a set of hash,value pairs
type NodeHasher func(n *Node) map[string]interface{}

// GraphIndexer provides a way to index graph nodes. A node can be mapped to
// multiple hash,value pairs. A hash can also be mapped to multiple nodes.
type GraphIndexer struct {
	sync.Mutex
	*GraphEventHandler
	DefaultGraphListener
	graph        *Graph
	hashNode     NodeHasher
	appendOnly   bool
	hashToValues map[string]map[Identifier]interface{}
	nodeToHashes map[Identifier]map[string]bool
}

func (i *GraphIndexer) index(id Identifier, h string, value interface{}) {
	if _, found := i.hashToValues[h]; !found {
		i.hashToValues[h] = make(map[Identifier]interface{})
	}
	i.hashToValues[h][id] = value
	i.nodeToHashes[id][h] = true
}

func (i *GraphIndexer) unindex(id Identifier, h string) {
	delete(i.hashToValues[h], id)
	if len(i.hashToValues[h]) == 0 {
		delete(i.hashToValues, h)
	}
}

// cacheNode indexes a node with a set of hash -> value map
func (i *GraphIndexer) cacheNode(n *Node, kv map[string]interface{}) {
	i.Lock()
	defer i.Unlock()

	if hashes, found := i.nodeToHashes[n.ID]; !found {
		// Node was not in the cache
		i.nodeToHashes[n.ID] = make(map[string]bool)
		for k, v := range kv {
			i.index(n.ID, k, v)
		}

		i.notifyEvent(graphEvent{element: n, kind: nodeAdded})
	} else {
		// Node already was in the cache
		if !i.appendOnly {
			for h := range hashes {
				if _, found := kv[h]; !found {
					i.unindex(n.ID, h)
				}
			}
		}

		for k, v := range kv {
			i.index(n.ID, k, v)
		}

		i.notifyEvent(graphEvent{element: n, kind: nodeUpdated})
	}
}

// forgetNode removes the node and its associated hashes from the index
func (i *GraphIndexer) forgetNode(n *Node) {
	i.Lock()
	defer i.Unlock()

	if hashes, found := i.nodeToHashes[n.ID]; found {
		delete(i.nodeToHashes, n.ID)
		for h := range hashes {
			delete(i.hashToValues[h], n.ID)
		}

		i.notifyEvent(graphEvent{element: n, kind: nodeDeleted})
	}
}

// OnNodeAdded event
func (i *GraphIndexer) OnNodeAdded(n *Node) {
	if kv := i.hashNode(n); len(kv) != 0 {
		i.cacheNode(n, kv)
	}
}

// OnNodeUpdated event
func (i *GraphIndexer) OnNodeUpdated(n *Node) {
	if kv := i.hashNode(n); len(kv) != 0 {
		i.cacheNode(n, kv)
	} else {
		i.forgetNode(n)
	}
}

// OnNodeDeleted event
func (i *GraphIndexer) OnNodeDeleted(n *Node) {
	i.forgetNode(n)
}

// Fromhash returns the nodes mapped by a hash along with their associated values
func (i *GraphIndexer) FromHash(hash string) (nodes []*Node, values []interface{}) {
	if ids, found := i.hashToValues[hash]; found {
		for id, obj := range ids {
			nodes = append(nodes, i.graph.GetNode(id))
			values = append(values, obj)
		}
	}
	return
}

// Start registers the graph indexer as a graph listener
func (i *GraphIndexer) Start() {
	i.graph.AddEventListener(i)
}

// Stop removes the graph indexer from the graph listeners
func (i *GraphIndexer) Stop() {
	i.graph.RemoveEventListener(i)
}

// NewGraphIndexer returns a new graph indexer with the associated hashing callback
func NewGraphIndexer(g *Graph, hashNode NodeHasher, appendOnly bool) *GraphIndexer {
	indexer := &GraphIndexer{
		GraphEventHandler: NewGraphEventHandler(maxEvents),
		graph:             g,
		hashNode:          hashNode,
		hashToValues:      make(map[string]map[Identifier]interface{}),
		nodeToHashes:      make(map[Identifier]map[string]bool),
		appendOnly:        appendOnly,
	}
	return indexer
}

// MetadataIndexer describes a metadata based graph indexer
type MetadataIndexer struct {
	*GraphIndexer
	indexes []string
}

// Hash computes the hash of the passed parameters
func (m *MetadataIndexer) Hash(values ...interface{}) string {
	if len(values) == 1 {
		if s, ok := values[0].(string); ok {
			return s
		}
	}
	h, _ := structhash.Hash(values, 1)
	return h
}

// Get computes the hash of the passed parameters and returns the matching
// nodes with their respective value
func (m *MetadataIndexer) Get(values ...interface{}) ([]*Node, []interface{}) {
	return m.FromHash(m.Hash(values...))
}

// NewMetadataIndexer returns a new metadata graph indexer for the nodes
// matching the graph filter `m`, indexing the metadata with `indexes`
func NewMetadataIndexer(g *Graph, m GraphElementMatcher, indexes ...string) (indexer *MetadataIndexer) {
	indexer = &MetadataIndexer{
		indexes: indexes,
		GraphIndexer: NewGraphIndexer(g, func(n *Node) (kv map[string]interface{}) {
			if match := n.MatchMetadata(m); match {
				switch len(indexes) {
				case 0:
					return map[string]interface{}{string(n.ID): nil}
				default:
					kv = make(map[string]interface{})
					values := make([]interface{}, len(indexes))
					for i, index := range indexes {
						v, err := n.GetField(index)
						if err != nil {
							return
						}
						values[i] = v
					}

					kv[indexer.Hash(values...)] = values
				}
			}
			return
		}, false),
	}
	return
}
