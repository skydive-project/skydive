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

package probe

import (
	"fmt"

	"github.com/skydive-project/skydive/common"
)

// ErrProbeNotCompiled is thrown when a flow probe was not compiled within the binary
var ErrProbeNotCompiled = fmt.Errorf("probe not compiled")

// Handler describes a ProbeHandler. A ProbeHandler aims to create probe
type Handler interface {
	Start()
	Stop()
}

// ServiceStatus describes the status returned by GetStatus
type ServiceStatus struct {
	Status common.ServiceState
}

// StatusReporter can be implemented by probes to report their status
type StatusReporter interface {
	GetStatus() interface{}
}

// Bundle describes a bundle of probes (topology of flow)
type Bundle struct {
	common.RWMutex
	Handlers map[string]Handler
}

// Start a bundle of probes
func (p *Bundle) Start() {
	p.RLock()
	defer p.RUnlock()

	for _, probe := range p.Handlers {
		probe.Start()
	}
}

// Stop a bundle of probes
func (p *Bundle) Stop() {
	p.RLock()
	defer p.RUnlock()

	for _, probe := range p.Handlers {
		probe.Stop()
	}
}

// GetHandler retrieve a specific handler
func (p *Bundle) GetHandler(typ string) Handler {
	p.RLock()
	defer p.RUnlock()

	if probe, ok := p.Handlers[typ]; ok {
		return probe
	}
	return nil
}

// ActiveProbes returns all active probes name
func (p *Bundle) ActiveProbes() []string {
	p.RLock()
	defer p.RUnlock()

	activeProbes := make([]string, 0, len(p.Handlers))
	for k := range p.Handlers {
		activeProbes = append(activeProbes, k)
	}
	return activeProbes
}

// AddHandler adds a probe to the bundle
func (p *Bundle) AddHandler(typ string, handler Handler) {
	p.Lock()
	defer p.Unlock()

	p.Handlers[typ] = handler
}

// NewBundle creates a new probe handler bundle
func NewBundle() *Bundle {
	return &Bundle{
		Handlers: make(map[string]Handler),
	}
}
