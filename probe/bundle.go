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

// Probe describes a Probe (topology or flow) mechanism API
type Probe interface {
	Start()
	Stop()
}

// ProbeBundle describes a bundle of probes (topology of flow)
type ProbeBundle struct {
	Probes map[string]Probe
}

// Start a bundle of probes
func (p *ProbeBundle) Start() {
	for _, p := range p.Probes {
		p.Start()
	}
}

// Stop a bundle of probes
func (p *ProbeBundle) Stop() {
	for _, p := range p.Probes {
		p.Stop()
	}
}

// GetProbe retrieve a specific probe name
func (p *ProbeBundle) GetProbe(name string) Probe {
	if probe, ok := p.Probes[name]; ok {
		return probe
	}
	return nil
}

// NewProbeBundle creates a new probe bundle
func NewProbeBundle(p map[string]Probe) *ProbeBundle {
	return &ProbeBundle{
		Probes: p,
	}
}
