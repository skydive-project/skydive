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

package probe

import "sync"

// Probe describes a Probe (topology or flow) mechanism API
type Probe interface {
	Start()
	Stop()
}

// ProbeBundle describes a bundle of probes (topology of flow)
type ProbeBundle struct {
	sync.RWMutex
	probes map[string]Probe
}

// Start a bundle of probes
func (p *ProbeBundle) Start() {
	p.RLock()
	defer p.RUnlock()

	for _, probe := range p.probes {
		probe.Start()
	}
}

// Stop a bundle of probes
func (p *ProbeBundle) Stop() {
	p.RLock()
	defer p.RUnlock()

	for _, probe := range p.probes {
		probe.Stop()
	}
}

// GetProbe retrieve a specific probe name
func (p *ProbeBundle) GetProbe(name string) Probe {
	p.RLock()
	defer p.RUnlock()

	if probe, ok := p.probes[name]; ok {
		return probe
	}
	return nil
}

//ActiveProbes returns all active probes name
func (p *ProbeBundle) ActiveProbes() []string {
	p.RLock()
	defer p.RUnlock()

	activeProbes := make([]string, 0, len(p.probes))
	for k := range p.probes {
		activeProbes = append(activeProbes, k)
	}
	return activeProbes
}

// AddProbe adds a probe to the bundle
func (p *ProbeBundle) AddProbe(name string, probe Probe) {
	p.RLock()
	defer p.RUnlock()

	p.probes[name] = probe
}

// NewProbeBundle creates a new probe bundle
func NewProbeBundle(p map[string]Probe) *ProbeBundle {
	return &ProbeBundle{
		probes: p,
	}
}
