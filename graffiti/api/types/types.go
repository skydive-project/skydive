//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

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

package types

import "github.com/skydive-project/skydive/graffiti/graph"

// Edge object
// easyjson:json
// swagger:model
type Edge graph.Edge

// GetID returns the edge ID
func (e *Edge) GetID() string {
	return string(e.ID)
}

// SetID sets the node ID
func (e *Edge) SetID(i string) {
	e.ID = graph.Identifier(i)
}

// GetName returns the edge resource name
func (e *Edge) GetName() string {
	return "Edge"
}

// Validate integrity of the resource
func (e *Edge) Validate() error {
	return nil
}

// Node object
// easyjson:json
// swagger:model
type Node graph.Node

// GetID returns the node ID
func (n *Node) GetID() string {
	return string(n.ID)
}

// SetID sets the resource ID
func (n *Node) SetID(i string) {
	n.ID = graph.Identifier(i)
}

// GetName returns the node resource name
func (n *Node) GetName() string {
	return "Node"
}

// Validate integrity of the resource
func (n *Node) Validate() error {
	return nil
}
