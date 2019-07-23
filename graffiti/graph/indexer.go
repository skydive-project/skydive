/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"github.com/cnf/structhash"

	"github.com/skydive-project/skydive/common"
)

// NodeHasher describes a callback that is called to map a node to
// a set of hash,value pairs
type NodeHasher func(n *Node) map[string]interface{}

// Hash computes the hash of the passed parameters
func Hash(values ...interface{}) string {
	if len(values) == 1 {
		if s, ok := values[0].(string); ok {
			return s
		}
	}
	h, _ := structhash.Hash(values, 1)
	return h
}

// NodeIndex provides a way to index graph nodes. A node can be mapped to
// multiple hash,value pairs. A hash can also be mapped to multiple nodes.
type NodeIndex struct {
	common.RWMutex
	graph        *Graph
	eventHandler *EventHandler
	hashNode     NodeHasher
	appendOnly   bool
	hashToValues map[string]map[Identifier]interface{}
	nodeToHashes map[Identifier]map[string]bool
}

// Indexer listens and indexes nodes using an Index
type Indexer struct {
	*NodeIndex
	DefaultGraphListener
	listenerHandler ListenerHandler
	hashNode        NodeHasher
}

// FromHash returns the nodes mapped by a hash along with their associated values
func (i *NodeIndex) FromHash(hash string) (nodes []*Node, values []interface{}) {
	if ids, found := i.hashToValues[hash]; found {
		for id, obj := range ids {
			nodes = append(nodes, i.graph.GetNode(id))
			values = append(values, obj)
		}
	}
	return
}

// Get computes the hash of the passed parameters and returns the matching
// nodes with their respective value
func (i *NodeIndex) Get(values ...interface{}) ([]*Node, []interface{}) {
	return i.FromHash(Hash(values...))
}

// GetNode computes the hash of the passed parameters and returns the first
// matching node with its respective value
func (i *NodeIndex) GetNode(values ...interface{}) (*Node, interface{}) {
	nodes, values := i.Get(values...)
	if len(nodes) > 0 && len(values) > 0 {
		return nodes[0], values[0]
	}
	return nil, nil
}

func (i *NodeIndex) index(id Identifier, h string, value interface{}) {
	if _, found := i.hashToValues[h]; !found {
		i.hashToValues[h] = make(map[Identifier]interface{})
	}
	i.hashToValues[h][id] = value
	i.nodeToHashes[id][h] = true
}

func (i *NodeIndex) unindex(id Identifier, h string) {
	delete(i.hashToValues[h], id)
	if len(i.hashToValues[h]) == 0 {
		delete(i.hashToValues, h)
	}
}

// Index indexes a node with a set of hash -> value map
func (i *NodeIndex) Index(id Identifier, n *Node, kv map[string]interface{}) {
	i.Lock()
	defer i.Unlock()

	if hashes, found := i.nodeToHashes[id]; !found {
		// Node was not in the cache
		i.nodeToHashes[id] = make(map[string]bool)
		for k, v := range kv {
			i.index(id, k, v)
		}

		i.eventHandler.NotifyEvent(NodeAdded, n)
	} else {
		// Node already was in the cache
		if !i.appendOnly {
			for h := range hashes {
				if _, found := kv[h]; !found {
					i.unindex(id, h)
				}
			}
		}

		for k, v := range kv {
			i.index(id, k, v)
		}

		i.eventHandler.NotifyEvent(NodeUpdated, n)
	}
}

// Unindex removes the node and its associated hashes from the index
func (i *NodeIndex) Unindex(id Identifier, n *Node) {
	i.Lock()
	defer i.Unlock()

	if hashes, found := i.nodeToHashes[id]; found {
		delete(i.nodeToHashes, id)
		for h := range hashes {
			delete(i.hashToValues[h], id)
		}

		i.eventHandler.NotifyEvent(NodeDeleted, n)
	}
}

// AddEventListener subscribes a new graph listener
func (i *NodeIndex) AddEventListener(l EventListener) {
	i.eventHandler.AddEventListener(l)
}

// RemoveEventListener unsubscribe a graph listener
func (i *NodeIndex) RemoveEventListener(l EventListener) {
	i.eventHandler.RemoveEventListener(l)
}

// NewNodeIndex returns a new node index
func NewNodeIndex(g *Graph, appendOnly bool) *NodeIndex {
	return &NodeIndex{
		graph:        g,
		eventHandler: NewEventHandler(maxEvents),
		hashToValues: make(map[string]map[Identifier]interface{}),
		nodeToHashes: make(map[Identifier]map[string]bool),
		appendOnly:   appendOnly,
	}
}

// OnNodeAdded event
func (i *Indexer) OnNodeAdded(n *Node) {
	if kv := i.hashNode(n); len(kv) != 0 {
		i.Index(n.ID, n, kv)
	}
}

// OnNodeUpdated event
func (i *Indexer) OnNodeUpdated(n *Node) {
	if kv := i.hashNode(n); len(kv) != 0 {
		i.Index(n.ID, n, kv)
	} else {
		i.Unindex(n.ID, n)
	}
}

// OnNodeDeleted event
func (i *Indexer) OnNodeDeleted(n *Node) {
	i.Unindex(n.ID, n)
}

// Start registers the graph indexer as a graph listener
func (i *Indexer) Start() error {
	if i.listenerHandler != nil {
		i.listenerHandler.AddEventListener(i)
	}
	return nil
}

// Stop removes the graph indexer from the graph listeners
func (i *Indexer) Stop() {
	if i.listenerHandler != nil {
		i.listenerHandler.RemoveEventListener(i)
	}
}

// NewIndexer returns a new graph indexer with the associated hashing callback
func NewIndexer(g *Graph, listenerHandler ListenerHandler, hashNode NodeHasher, appendOnly bool) *Indexer {
	if hashNode == nil {
		hashNode = func(n *Node) map[string]interface{} { return nil }
	}
	indexer := &Indexer{
		NodeIndex:       NewNodeIndex(g, appendOnly),
		listenerHandler: listenerHandler,
		hashNode:        hashNode,
	}
	return indexer
}

// MetadataIndexer describes a metadata based graph indexer
type MetadataIndexer struct {
	*Indexer
	indexes []string
	matcher ElementMatcher
}

// NewMetadataIndexer returns a new metadata graph indexer for the nodes
// matching the graph filter `m`, indexing the metadata with `indexes`
func NewMetadataIndexer(g *Graph, listenerHandler ListenerHandler, m ElementMatcher, indexes ...string) (indexer *MetadataIndexer) {
	if len(indexes) == 0 {
		panic("MetadataIndexer object can't be created with no indexes")
	}
	indexer = &MetadataIndexer{
		matcher: m,
		indexes: indexes,
		Indexer: NewIndexer(g, listenerHandler, func(n *Node) (kv map[string]interface{}) {
			if match := n.MatchMetadata(m); match {
				kv = make(map[string]interface{})
				values, err := getFieldsAsArray(n, indexes)
				if err == nil {
					for _, fields := range values {
						if len(indexes) == len(fields) {
							kv[Hash(fields...)] = fields
						}
					}
				}
			}
			return
		}, false),
	}
	return
}

// Sync synchronizes the index with the graph
func (i *MetadataIndexer) Sync() {
	i.hashToValues = make(map[string]map[Identifier]interface{})
	i.nodeToHashes = make(map[Identifier]map[string]bool)
	for _, n := range i.graph.GetNodes(i.matcher) {
		if kv := i.hashNode(n); len(kv) != 0 {
			i.Index(n.ID, n, kv)
		}
	}
}
