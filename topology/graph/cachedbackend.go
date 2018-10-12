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
	"sync/atomic"
)

// Define the running cache mode, memory and/or persistent
const (
	CacheOnlyMode int = iota
	PersistentOnlyMode
	DefaultMode
)

// CachedBackend describes a cache mechanism in memory and/or persistent database
type CachedBackend struct {
	memory     *MemoryBackend
	persistent Backend
	cacheMode  atomic.Value
}

// SetMode set cache mode
func (c *CachedBackend) SetMode(mode int) {
	c.cacheMode.Store(mode)
}

// NodeAdded same the node in the cache
func (c *CachedBackend) NodeAdded(n *Node) bool {
	mode := c.cacheMode.Load()

	r := false
	if mode != PersistentOnlyMode {
		r = c.memory.NodeAdded(n)
	}

	if mode != CacheOnlyMode {
		r = c.persistent.NodeAdded(n)
	}

	return r
}

// NodeDeleted Delete the node in the cache
func (c *CachedBackend) NodeDeleted(n *Node) bool {
	mode := c.cacheMode.Load()

	r := false
	if mode != PersistentOnlyMode {
		r = c.memory.NodeDeleted(n)
	}

	if mode != CacheOnlyMode {
		r = c.persistent.NodeDeleted(n)
	}

	return r
}

// GetNode retrieve a node from the cache within a time slice
func (c *CachedBackend) GetNode(i Identifier, t Context) []*Node {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil && mode != PersistentOnlyMode {
		return c.memory.GetNode(i, t)
	}

	if mode != CacheOnlyMode {
		return c.persistent.GetNode(i, t)
	}

	return nil
}

// GetNodeEdges retrieve a list of edges from a node within a time slice, matching metadata
func (c *CachedBackend) GetNodeEdges(n *Node, t Context, m ElementMatcher) (edges []*Edge) {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil && mode != PersistentOnlyMode {
		return c.memory.GetNodeEdges(n, t, m)
	}

	if mode != CacheOnlyMode {
		return c.persistent.GetNodeEdges(n, t, m)
	}

	return edges
}

// EdgeAdded add an edge in the cache
func (c *CachedBackend) EdgeAdded(e *Edge) bool {
	mode := c.cacheMode.Load()

	r := false
	if mode != PersistentOnlyMode {
		r = c.memory.EdgeAdded(e)
	}

	if mode != CacheOnlyMode {
		r = c.persistent.EdgeAdded(e)
	}

	return r
}

// EdgeDeleted delete an edge in the cache
func (c *CachedBackend) EdgeDeleted(e *Edge) bool {
	mode := c.cacheMode.Load()

	r := false
	if mode != PersistentOnlyMode {
		r = c.memory.EdgeDeleted(e)
	}

	if mode != CacheOnlyMode {
		r = c.persistent.EdgeDeleted(e)
	}

	return r
}

// GetEdge retrieve an edge within a time slice
func (c *CachedBackend) GetEdge(i Identifier, t Context) []*Edge {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil && mode != PersistentOnlyMode {
		return c.memory.GetEdge(i, t)
	}

	if mode != CacheOnlyMode {
		return c.persistent.GetEdge(i, t)
	}

	return nil
}

// GetEdgeNodes retrieve a list of nodes from an edge within a time slice, matching metadata
func (c *CachedBackend) GetEdgeNodes(e *Edge, t Context, parentMetadata, childMetadata ElementMatcher) ([]*Node, []*Node) {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil && mode != PersistentOnlyMode {
		return c.memory.GetEdgeNodes(e, t, parentMetadata, childMetadata)
	}

	if mode != CacheOnlyMode {
		return c.persistent.GetEdgeNodes(e, t, parentMetadata, childMetadata)
	}

	return nil, nil
}

// MetadataUpdated updates metadata
func (c *CachedBackend) MetadataUpdated(i interface{}) bool {
	mode := c.cacheMode.Load()

	r := false
	if mode != CacheOnlyMode {
		r = c.persistent.MetadataUpdated(i)
	}

	if mode != PersistentOnlyMode {
		r = c.memory.MetadataUpdated(i)
	}

	return r
}

// GetNodes returns a list of nodes with a time slice, matching metadata
func (c *CachedBackend) GetNodes(t Context, m ElementMatcher) []*Node {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil && mode != PersistentOnlyMode {
		return c.memory.GetNodes(t, m)
	}

	if mode != CacheOnlyMode {
		return c.persistent.GetNodes(t, m)
	}

	return []*Node{}
}

// GetEdges returns a list of edges with a time slice, matching metadata
func (c *CachedBackend) GetEdges(t Context, m ElementMatcher) []*Edge {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil && mode != PersistentOnlyMode {
		return c.memory.GetEdges(t, m)
	}

	if mode != CacheOnlyMode {
		return c.persistent.GetEdges(t, m)
	}

	return []*Edge{}
}

// IsHistorySupported returns whether the persistent backend supports history
func (c *CachedBackend) IsHistorySupported() bool {
	return c.persistent.IsHistorySupported()
}

// NewCachedBackend creates new graph cache mechanism
func NewCachedBackend(persistent Backend) (*CachedBackend, error) {
	memory, err := NewMemoryBackend()
	if err != nil {
		return nil, err
	}

	sb := &CachedBackend{
		persistent: persistent,
		memory:     memory,
	}

	sb.cacheMode.Store(DefaultMode)

	return sb, nil
}
