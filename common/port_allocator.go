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
	"sync"
)

var (
	InvalidPortRange = errors.New("Invalid port range")
	NoPortLeft       = errors.New("No free port left")
)

type PortAllocator struct {
	sync.RWMutex
	MinPort int
	MaxPort int
	PortMap map[int]interface{}
}

func (p *PortAllocator) Allocate() (int, error) {
	p.Lock()
	defer p.Unlock()

	for i := p.MinPort; i <= p.MaxPort; i++ {
		if _, ok := p.PortMap[i]; !ok {
			p.PortMap[i] = struct{}{}
			return i, nil
		}
	}
	return 0, NoPortLeft
}

func (p *PortAllocator) Set(i int, obj interface{}) {
	p.Lock()
	defer p.Unlock()

	p.PortMap[i] = obj
}

func (p *PortAllocator) Release(i int) {
	p.Lock()
	defer p.Unlock()

	delete(p.PortMap, i)
}

func (p *PortAllocator) ReleaseAll() {
	p.Lock()
	defer p.Unlock()

	p.PortMap = make(map[int]interface{})
}

func NewPortAllocator(min, max int) (*PortAllocator, error) {
	if min <= 0 || max < min {
		return nil, InvalidPortRange
	}

	return &PortAllocator{
		MinPort: min,
		MaxPort: max,
		PortMap: make(map[int]interface{}),
	}, nil
}
