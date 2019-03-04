/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"errors"
	"sync/atomic"
)

// Define the running cache mode, memory and/or persistent
const (
	CacheOnlyMode int = iota
	DefaultMode
)

// Cachebackend graph errors
var (
	ErrCacheBackendModeUnknown = errors.New("Cache backend mode unknown")
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
func (c *CachedBackend) NodeAdded(n *Node) error {
	mode := c.cacheMode.Load()

	if err := c.memory.NodeAdded(n); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.NodeAdded(n); err != nil {
			return err
		}
	}

	return nil
}

// NodeDeleted Delete the node in the cache
func (c *CachedBackend) NodeDeleted(n *Node) error {
	mode := c.cacheMode.Load()

	if err := c.memory.NodeDeleted(n); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.NodeDeleted(n); err != nil {
			return err
		}
	}

	return nil
}

// GetNode retrieve a node from the cache within a time slice
func (c *CachedBackend) GetNode(i Identifier, t Context) []*Node {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetNode(i, t)
	}

	return c.persistent.GetNode(i, t)
}

// GetNodeEdges retrieve a list of edges from a node within a time slice, matching metadata
func (c *CachedBackend) GetNodeEdges(n *Node, t Context, m ElementMatcher) (edges []*Edge) {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetNodeEdges(n, t, m)
	}

	return c.persistent.GetNodeEdges(n, t, m)
}

// EdgeAdded add an edge in the cache
func (c *CachedBackend) EdgeAdded(e *Edge) error {
	mode := c.cacheMode.Load()

	if err := c.memory.EdgeAdded(e); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.EdgeAdded(e); err != nil {
			return err
		}
	}

	return nil
}

// EdgeDeleted delete an edge in the cache
func (c *CachedBackend) EdgeDeleted(e *Edge) error {
	mode := c.cacheMode.Load()

	if err := c.memory.EdgeDeleted(e); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.EdgeDeleted(e); err != nil {
			return err
		}
	}

	return nil
}

// GetEdge retrieve an edge within a time slice
func (c *CachedBackend) GetEdge(i Identifier, t Context) []*Edge {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetEdge(i, t)
	}

	return c.persistent.GetEdge(i, t)
}

// GetEdgeNodes retrieve a list of nodes from an edge within a time slice, matching metadata
func (c *CachedBackend) GetEdgeNodes(e *Edge, t Context, parentMetadata, childMetadata ElementMatcher) ([]*Node, []*Node) {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetEdgeNodes(e, t, parentMetadata, childMetadata)
	}

	return c.persistent.GetEdgeNodes(e, t, parentMetadata, childMetadata)
}

// MetadataUpdated updates metadata
func (c *CachedBackend) MetadataUpdated(i interface{}) error {
	mode := c.cacheMode.Load()

	if err := c.memory.MetadataUpdated(i); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.MetadataUpdated(i); err != nil {
			return err
		}
	}
	return nil
}

// GetNodes returns a list of nodes with a time slice, matching metadata
func (c *CachedBackend) GetNodes(t Context, m ElementMatcher) []*Node {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetNodes(t, m)
	}

	return c.persistent.GetNodes(t, m)
}

// GetEdges returns a list of edges with a time slice, matching metadata
func (c *CachedBackend) GetEdges(t Context, m ElementMatcher) []*Edge {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetEdges(t, m)
	}
	return c.persistent.GetEdges(t, m)
}

// IsHistorySupported returns whether the persistent backend supports history
func (c *CachedBackend) IsHistorySupported() bool {
	return c.persistent != nil && c.persistent.IsHistorySupported()
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
