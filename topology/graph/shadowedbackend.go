/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"time"

	"github.com/skydive-project/skydive/logging"
)

type ShadowedBackend struct {
	memory     *MemoryBackend
	persistent GraphBackend
}

func (c *ShadowedBackend) AddNode(n *Node) bool {
	c.memory.AddNode(n)
	return c.persistent.AddNode(n)
}

func (c *ShadowedBackend) DelNode(n *Node) bool {
	c.memory.DelNode(n)
	return c.persistent.DelNode(n)
}

func (c *ShadowedBackend) GetNode(i Identifier, t *time.Time) *Node {
	if t == nil {
		return c.memory.GetNode(i, t)
	}
	return c.persistent.GetNode(i, t)
}

func (c *ShadowedBackend) GetNodeEdges(n *Node, t *time.Time) (edges []*Edge) {
	if t == nil {
		return c.memory.GetNodeEdges(n, t)
	}
	return c.persistent.GetNodeEdges(n, t)
}

func (c *ShadowedBackend) AddEdge(e *Edge) bool {
	c.memory.AddEdge(e)
	return c.persistent.AddEdge(e)
}

func (c *ShadowedBackend) DelEdge(e *Edge) bool {
	c.memory.DelEdge(e)
	return c.persistent.DelEdge(e)
}

func (c *ShadowedBackend) GetEdge(i Identifier, t *time.Time) *Edge {
	if t == nil {
		return c.memory.GetEdge(i, t)
	}
	return c.persistent.GetEdge(i, t)
}

func (c *ShadowedBackend) GetEdgeNodes(e *Edge, t *time.Time) (*Node, *Node) {
	if t == nil {
		return c.memory.GetEdgeNodes(e, t)
	}
	return c.persistent.GetEdgeNodes(e, t)
}

func (c *ShadowedBackend) AddMetadata(i interface{}, k string, v interface{}) bool {
	c.memory.AddMetadata(i, k, v)
	return c.persistent.AddMetadata(i, k, v)
}

func (c *ShadowedBackend) SetMetadata(i interface{}, metadata Metadata) bool {
	c.memory.SetMetadata(i, metadata)
	return c.persistent.SetMetadata(i, metadata)
}

func (c *ShadowedBackend) GetNodes(t *time.Time, m Metadata) []*Node {
	if t == nil {
		return c.memory.GetNodes(t, m)
	}
	return c.persistent.GetNodes(t, m)
}

func (c *ShadowedBackend) GetEdges(t *time.Time, m Metadata) []*Edge {
	if t == nil {
		return c.memory.GetEdges(t, m)
	}
	return c.persistent.GetEdges(t, m)
}

func (c *ShadowedBackend) WithContext(graph *Graph, context GraphContext) (*Graph, error) {
	return c.persistent.WithContext(graph, context)
}

func (c *ShadowedBackend) populateMemoryBackend() {
	for _, node := range c.persistent.GetNodes(nil, Metadata{}) {
		c.memory.AddNode(node)
	}

	for _, edge := range c.persistent.GetEdges(nil, Metadata{}) {
		c.memory.AddEdge(edge)
	}

	logging.GetLogger().Debug("Population of memory backend from persistent backend done")
}

func NewShadowedBackend(persistent GraphBackend) (*ShadowedBackend, error) {
	memory, err := NewMemoryBackend()
	if err != nil {
		return nil, err
	}

	sb := &ShadowedBackend{
		persistent: persistent,
		memory:     memory,
	}

	sb.populateMemoryBackend()

	return sb, nil
}
