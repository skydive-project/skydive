// +build !linux

/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package enhancers

import (
	"time"

	"github.com/skydive-project/skydive/flow"
)

// SocketInfoEnhancer describes a SocketInfo Enhancer with TCP caches
type SocketInfoEnhancer struct {
}

// Name returns the SocketInfo enhancer name
func (s *SocketInfoEnhancer) Name() string {
	return "SocketInfo"
}

// Enhance the graph with process info
func (s *SocketInfoEnhancer) Enhance(f *flow.Flow) {
}

// Start the flow enhancer
func (s *SocketInfoEnhancer) Start() error {
	return nil
}

// Stop the flow enhancer
func (s *SocketInfoEnhancer) Stop() {
}

// NewSocketInfoEnhancer create a new SocketInfo Enhancer
func NewSocketInfoEnhancer(expire, cleanup time.Duration) *SocketInfoEnhancer {
	return &SocketInfoEnhancer{}
}
