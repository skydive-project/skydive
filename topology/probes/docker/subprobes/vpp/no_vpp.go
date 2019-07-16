// +build !linux !docker_vpp

// Copyright (c) 2019 PANTHEON.tech s.r.o.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vpp

import (
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/probes/docker/subprobes"
	"github.com/skydive-project/skydive/topology/probes/netns"
)

// DummySubprobe is VPP subprobe that does nothing. It is used in cases when build tags prohibit to use real VPP subprobe
type DummySubprobe struct{}

// NewSubprobe creates a new topology Docker subprobe
func NewSubprobe(dockerNSProbe *netns.Probe) (*DummySubprobe, error) {
	logging.GetLogger().Debug("Dummy VPP probe creating...")
	return &DummySubprobe{}, nil
}

// Start the dummy probe
func (p *DummySubprobe) Start() {
	logging.GetLogger().Debug("Dummy VPP subprobe starting...")
}

// Stop the dummy probe
func (p *DummySubprobe) Stop() {
	logging.GetLogger().Debug("Dummy VPP subprobe stopping...")
}

// RegisterContainer is called by docker probe to notify subprobe about container addition detection (registration)
func (p *DummySubprobe) RegisterContainer(data *subprobes.ContainerRegistrationData) error {
	return nil // Dummy implementation
}

// UnregisterContainer is called by docker probe to notify subprobe about container removal detection (unregistration)
func (p *DummySubprobe) UnregisterContainer(data *subprobes.ContainerUnregistrationData) error {
	return nil // Dummy implementation
}
