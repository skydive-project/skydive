/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package flow

// Enhancer should Enhance the flow via this interface
type Enhancer interface {
	Enhance(flow *Flow)
}

// EnhancerPipeline describes a list of flow enhancer
type EnhancerPipeline struct {
	Enhancers []Enhancer
}

// EnhanceFlow enhance a flow from with all registered enhancer
func (e *EnhancerPipeline) EnhanceFlow(flow *Flow) {
	for _, enhancer := range e.Enhancers {
		enhancer.Enhance(flow)
	}
}

// Enhance a list of flows
func (e *EnhancerPipeline) Enhance(flows []*Flow) {
	for _, flow := range flows {
		e.EnhanceFlow(flow)
	}
}

// AddEnhancer registers a new flow enhancer
func (e *EnhancerPipeline) AddEnhancer(en Enhancer) {
	e.Enhancers = append(e.Enhancers, en)
}

// NewEnhancerPipeline registers a list of flow Enhancer
func NewEnhancerPipeline(enhancers ...Enhancer) *EnhancerPipeline {
	return &EnhancerPipeline{
		Enhancers: enhancers,
	}
}
