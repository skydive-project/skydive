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
	Name() string
	Enhance(flow *Flow)
	Start() error
	Stop()
}

// EnhancerPipeline describes a list of flow enhancer
type EnhancerPipeline struct {
	Enhancers map[string]Enhancer
}

// EnhancerPipelineConfig describes configuration of enabled enhancers
type EnhancerPipelineConfig struct {
	enabled map[string]bool
}

// NewEnhancerPipelineConfig create a new enhancer pipeline config
func NewEnhancerPipelineConfig() *EnhancerPipelineConfig {
	return &EnhancerPipelineConfig{enabled: make(map[string]bool)}
}

// Disable the named enhancer
func (epc *EnhancerPipelineConfig) Disable(name string) {
	epc.enabled[name] = false
}

// Enable the named enhancer
func (epc *EnhancerPipelineConfig) Enable(name string) {
	epc.enabled[name] = true
}

// IsEnabled return true if the named enhancer
func (epc *EnhancerPipelineConfig) IsEnabled(name string) bool {
	v, ok := epc.enabled[name]
	if !ok {
		return true
	}
	return v
}

// EnhanceFlow enhance a flow from with all registered enhancer
func (e *EnhancerPipeline) EnhanceFlow(cfg *EnhancerPipelineConfig, flow *Flow) {
	for _, enhancer := range e.Enhancers {
		if cfg.IsEnabled(enhancer.Name()) {
			enhancer.Enhance(flow)
		}
	}
}

// Enhance a list of flows
func (e *EnhancerPipeline) Enhance(cfg *EnhancerPipelineConfig, flows []*Flow) {
	for _, flow := range flows {
		e.EnhanceFlow(cfg, flow)
	}
}

// Start starts all the enhancers
func (e *EnhancerPipeline) Start() {
	for _, enhancer := range e.Enhancers {
		enhancer.Start()
	}
}

// Stop stops all the enhancers
func (e *EnhancerPipeline) Stop() {
	for _, enhancer := range e.Enhancers {
		enhancer.Stop()
	}
}

// AddEnhancer registers a new flow enhancer
func (e *EnhancerPipeline) AddEnhancer(en Enhancer) {
	e.Enhancers[en.Name()] = en
}

// NewEnhancerPipeline registers a list of flow Enhancer
func NewEnhancerPipeline(enhancers ...Enhancer) *EnhancerPipeline {
	ep := &EnhancerPipeline{
		Enhancers: make(map[string]Enhancer),
	}
	for _, en := range enhancers {
		ep.AddEnhancer(en)
	}
	return ep
}
