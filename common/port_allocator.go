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

package common

import (
	"errors"
)

var (
	// ErrInvalidPortRange invalid port range
	ErrInvalidPortRange = errors.New("Invalid port range")
	// ErrNoPortLeft no port left in the range
	ErrNoPortLeft = errors.New("No free port left")
)

// PortAllocator describes a threads safe port list that can be allocated
type PortAllocator struct {
	RWMutex
	MinPort int
	MaxPort int
	PortMap map[int]bool
}

// Allocate returns a new port between min and max ports.
func (p *PortAllocator) Allocate() (int, error) {
	p.Lock()
	defer p.Unlock()

	for i := p.MinPort; i <= p.MaxPort; i++ {
		if _, ok := p.PortMap[i]; !ok {
			p.PortMap[i] = true
			return i, nil
		}
	}
	return 0, ErrNoPortLeft
}

// Release a port
func (p *PortAllocator) Release(i int) error {
	p.Lock()
	defer p.Unlock()

	if i < p.MinPort || i > p.MaxPort {
		return ErrInvalidPortRange
	}

	delete(p.PortMap, i)
	return nil
}

// ReleaseAll ports
func (p *PortAllocator) ReleaseAll() {
	p.Lock()
	defer p.Unlock()

	p.PortMap = make(map[int]bool)
}

// NewPortAllocator creates a new port allocator range
func NewPortAllocator(min, max int) (*PortAllocator, error) {
	if min <= 0 || max < min {
		return nil, ErrInvalidPortRange
	}

	return &PortAllocator{
		MinPort: min,
		MaxPort: max,
		PortMap: make(map[int]bool),
	}, nil
}
