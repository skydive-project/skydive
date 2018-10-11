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
	"github.com/cnf/structhash"

	"github.com/skydive-project/skydive/common"
)

// NodeHasher describes a callback that is called to map a node to
// a set of hash,value pairs
type NodeHasher func(n *Node) map[string]interface{}

// Indexer provides a way to index graph nodes. A node can be mapped to
// multiple hash,value pairs. A hash can also be mapped to multiple nodes.
type Indexer struct {
	common.RWMutex
	DefaultGraphListener
	graph           *Graph
	eventHandler    *EventHandler
	listenerHandler ListenerHandler
	hashNode        NodeHasher
	appendOnly      bool
	hashToValues    map[string]map[Identifier]interface{}
	nodeToHashes    map[Identifier]map[string]bool
}

func (i *Indexer) index(id Identifier, h string, value interface{}) {
	if _, found := i.hashToValues[h]; !found {
		i.hashToValues[h] = make(map[Identifier]interface{})
	}
	i.hashToValues[h][id] = value
	i.nodeToHashes[id][h] = true
}

func (i *Indexer) unindex(id Identifier, h string) {
	delete(i.hashToValues[h], id)
	if len(i.hashToValues[h]) == 0 {
		delete(i.hashToValues, h)
	}
}

// cacheNode indexes a node with a set of hash -> value map
func (i *Indexer) cacheNode(n *Node, kv map[string]interface{}) {
	i.Lock()
	defer i.Unlock()

	if hashes, found := i.nodeToHashes[n.ID]; !found {
		// Node was not in the cache
		i.nodeToHashes[n.ID] = make(map[string]bool)
		for k, v := range kv {
			i.index(n.ID, k, v)
		}

		i.eventHandler.NotifyEvent(NodeAdded, n)
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

		i.eventHandler.NotifyEvent(NodeUpdated, n)
	}
}

// forgetNode removes the node and its associated hashes from the index
func (i *Indexer) forgetNode(n *Node) {
	i.Lock()
	defer i.Unlock()

	if hashes, found := i.nodeToHashes[n.ID]; found {
		delete(i.nodeToHashes, n.ID)
		for h := range hashes {
			delete(i.hashToValues[h], n.ID)
		}

		i.eventHandler.NotifyEvent(NodeDeleted, n)
	}
}

// OnNodeAdded event
func (i *Indexer) OnNodeAdded(n *Node) {
	if kv := i.hashNode(n); len(kv) != 0 {
		i.cacheNode(n, kv)
	}
}

// OnNodeUpdated event
func (i *Indexer) OnNodeUpdated(n *Node) {
	if kv := i.hashNode(n); len(kv) != 0 {
		i.cacheNode(n, kv)
	} else {
		i.forgetNode(n)
	}
}

// OnNodeDeleted event
func (i *Indexer) OnNodeDeleted(n *Node) {
	i.forgetNode(n)
}

// FromHash returns the nodes mapped by a hash along with their associated values
func (i *Indexer) FromHash(hash string) (nodes []*Node, values []interface{}) {
	if ids, found := i.hashToValues[hash]; found {
		for id, obj := range ids {
			nodes = append(nodes, i.graph.GetNode(id))
			values = append(values, obj)
		}
	}
	return
}

// Start registers the graph indexer as a graph listener
func (i *Indexer) Start() {
	i.listenerHandler.AddEventListener(i)
}

// Stop removes the graph indexer from the graph listeners
func (i *Indexer) Stop() {
	i.listenerHandler.RemoveEventListener(i)
}

// AddEventListener subscibe a new graph listener
func (i *Indexer) AddEventListener(l EventListener) {
	i.eventHandler.AddEventListener(l)
}

// RemoveEventListener unsubscribe a graph listener
func (i *Indexer) RemoveEventListener(l EventListener) {
	i.eventHandler.RemoveEventListener(l)
}

// NewIndexer returns a new graph indexer with the associated hashing callback
func NewIndexer(g *Graph, listenerHandler ListenerHandler, hashNode NodeHasher, appendOnly bool) *Indexer {
	indexer := &Indexer{
		graph:           g,
		eventHandler:    NewEventHandler(maxEvents),
		listenerHandler: listenerHandler,
		hashNode:        hashNode,
		hashToValues:    make(map[string]map[Identifier]interface{}),
		nodeToHashes:    make(map[Identifier]map[string]bool),
		appendOnly:      appendOnly,
	}
	return indexer
}

// MetadataIndexer describes a metadata based graph indexer
type MetadataIndexer struct {
	*Indexer
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
func NewMetadataIndexer(g *Graph, listenerHandler ListenerHandler, m ElementMatcher, indexes ...string) (indexer *MetadataIndexer) {
	indexer = &MetadataIndexer{
		indexes: indexes,
		Indexer: NewIndexer(g, listenerHandler, func(n *Node) (kv map[string]interface{}) {
			if match := n.MatchMetadata(m); match {
				switch len(indexes) {
				case 0:
					return map[string]interface{}{string(n.ID): nil}
				default:
					kv = make(map[string]interface{})
					if values, err := getFieldsAsArray(n, indexes); err == nil {
						kv[indexer.Hash(values...)] = values
					}
				}
			}
			return
		}, false),
	}
	return
}
