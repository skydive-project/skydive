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
	"errors"
	"fmt"

	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/common"
	api "github.com/skydive-project/skydive/graffiti/api/server"
)

// ErrNotCompiled is thrown when a probe was not compiled within the binary
var ErrNotCompiled = fmt.Errorf("probe not compiled")

// ErrNotStopped is used when a not stopped probe is started
var ErrNotStopped = errors.New("probe is not stopped")

// ErrNotRunning is used when a probe is not running
var ErrNotRunning = errors.New("probe is not running")

// Handler describes a probe
type Handler interface {
	Start() error
	Stop()
}

// ServiceStatus describes the status returned by GetStatus
type ServiceStatus struct {
	Status common.ServiceState
}

// Bundle describes a bundle of probes (topology of flow)
type Bundle struct {
	insanelock.RWMutex
	Handlers map[string]Handler
}

// Start a bundle of probes
func (p *Bundle) Start() error {
	p.RLock()
	defer p.RUnlock()

	for _, probe := range p.Handlers {
		if err := probe.Start(); err != nil {
			return err
		}
	}

	return nil
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

// EnabledProbes returns all enabled probes name
func (p *Bundle) EnabledProbes() []string {
	p.RLock()
	defer p.RUnlock()

	activeProbes := make([]string, 0, len(p.Handlers))
	for k := range p.Handlers {
		activeProbes = append(activeProbes, k)
	}
	return activeProbes
}

// GetStatus returns the status of all the probes
func (p *Bundle) GetStatus() map[string]interface{} {
	p.RLock()
	defer p.RUnlock()

	status := make(map[string]interface{})
	for k, v := range p.Handlers {
		if v, ok := v.(api.StatusReporter); ok {
			status[k] = v.GetStatus()
		} else {
			status[k] = &ServiceStatus{Status: common.RunningState}
		}
	}
	return status
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
