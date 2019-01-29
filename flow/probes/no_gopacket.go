// +build !linux

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

package probes

import (
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// GoPacketProbesHandler describes a flow probe handle in the graph
type GoPacketProbesHandler struct {
}

// RegisterProbe registers a gopacket probe
func (p *GoPacketProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	return common.ErrNotImplemented
}

// UnregisterProbe unregisters gopacket probe
func (p *GoPacketProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	return common.ErrNotImplemented
}

// Start probe
func (p *GoPacketProbesHandler) Start() {
}

// Stop probe
func (p *GoPacketProbesHandler) Stop() {
}

// NewGoPacketProbesHandler creates a new gopacket probe in the graph
func NewGoPacketProbesHandler(g *graph.Graph, fpta *FlowProbeTableAllocator) (*GoPacketProbesHandler, error) {
	return nil, common.ErrNotImplemented
}
