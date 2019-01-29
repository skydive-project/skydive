/*
 * Copyright (C) 2018 Orange
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
	"github.com/skydive-project/skydive/common"
)

// NodeAction is a callback to perform on a node. The action is kept
// active as long as it returns true.
type NodeAction interface {
	ProcessNode(g *Graph, n *Node) bool
}

// deferred represents a node action with additional info if needed
// for cancellation.
type deferred struct {
	action NodeAction
}

// Processor encapsulates an indexer that will process NodeActions
// on the nodes that filter
type Processor struct {
	common.RWMutex
	DefaultGraphListener
	MetadataIndexer *MetadataIndexer
	actions         map[string][]deferred
}

// NewProcessor creates a Processor on the graph g, a stream of
// events controlled by listenerHandler, that match a first set
// of metadata m. Actions will be associated to a given set
// of values for indexes.
func NewProcessor(g *Graph, listenerHandler ListenerHandler, m ElementMatcher, indexes ...string) (processor *Processor) {
	processor = &Processor{
		MetadataIndexer: NewMetadataIndexer(g, listenerHandler, m, indexes...),
		actions:         make(map[string][]deferred),
	}
	processor.MetadataIndexer.AddEventListener(processor)
	return
}

// DoAction will perform the action for nodes matching values.
func (processor *Processor) DoAction(action NodeAction, values ...interface{}) {
	processor.Lock()
	defer processor.Unlock()
	nodes, _ := processor.MetadataIndexer.Get(values...)
	kont := true
	for _, node := range nodes {
		kont = action.ProcessNode(processor.MetadataIndexer.graph, node)
		if !kont {
			break
		}
	}
	if kont {
		act := deferred{action: action}
		hash := Hash(values...)
		if actions, ok := processor.actions[hash]; ok {
			actions = append(actions, act)
		} else {
			actions := []deferred{act}
			processor.actions[hash] = actions
		}
	}
}

// Start starts the processor
func (processor *Processor) Start() {
	processor.MetadataIndexer.Start()
}

// Stop stops the processor
func (processor *Processor) Stop() {
	processor.MetadataIndexer.Stop()
}

// Cancel the actions attached to a given set of values.
func (processor *Processor) Cancel(values ...interface{}) {
	processor.Lock()
	delete(processor.actions, Hash(values...))
	processor.Unlock()
}

// OnNodeAdded event
func (processor *Processor) OnNodeAdded(n *Node) {
	if values, err := getFieldsAsArray(n, processor.MetadataIndexer.indexes); err == nil {
		hash := Hash(values...)
		processor.Lock()
		defer processor.Unlock()
		if actions, ok := processor.actions[hash]; ok {
			var keep []deferred
			for _, action := range actions {
				if action.action.ProcessNode(processor.MetadataIndexer.graph, n) {
					keep = append(keep, action)
				}
			}
			if len(keep) == 0 {
				delete(processor.actions, hash)
			} else {
				processor.actions[hash] = keep
			}
		}
	}
}
