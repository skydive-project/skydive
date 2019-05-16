// +build !dpdk

/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package probes

import (
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// DPDKProbesHandler describes a flow probe handle in the graph
type DPDKProbesHandler struct {
}

// RegisterProbe registers a gopacket probe
func (p *DPDKProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e ProbeEventHandler) (Probe, error) {
	return nil, nil
}

// UnregisterProbe unregisters gopacket probe
func (p *DPDKProbesHandler) UnregisterProbe(n *graph.Node, e ProbeEventHandler, fp Probe) error {
	return nil
}

// Start probe
func (p *DPDKProbesHandler) Start() {
}

// Stop probe
func (p *DPDKProbesHandler) Stop() {
}

// NewDPDKProbesHandler creates a new gopacket probe in the graph
func NewDPDKProbesHandler(g *graph.Graph, fta *flow.TableAllocator) (*DPDKProbesHandler, error) {
	return nil, ErrProbeNotCompiled
}
