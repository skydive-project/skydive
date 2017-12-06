// +build !ebpf

/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package probes

import (
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/topology/graph"
)

// EBPFProbesHandler describes a flow probe handle in the graph
type EBPFProbesHandler struct {
}

// RegisterProbe registers a gopacket probe
func (p *EBPFProbesHandler) RegisterProbe(n *graph.Node, capture *api.Capture, e FlowProbeEventHandler) error {
	return nil
}

// UnregisterProbe unregisters gopacket probe
func (p *EBPFProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	return nil
}

// Start probe
func (p *EBPFProbesHandler) Start() {
}

// Stop probe
func (p *EBPFProbesHandler) Stop() {
}

// NewEBPFProbesHandler creates a new gopacket probe in the graph
func NewEBPFProbesHandler(g *graph.Graph, fpta *FlowProbeTableAllocator) (*EBPFProbesHandler, error) {
	return nil, ErrProbeNotCompiled
}
