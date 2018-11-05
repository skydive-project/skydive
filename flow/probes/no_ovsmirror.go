// +build !linux

/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

// OvsMirrorProbesHandler describes a flow probe handle in the graph
type OvsMirrorProbesHandler struct {
}

// RegisterProbe registers a gopacket probe
func (p *OvsMirrorProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	return common.ErrNotImplemented
}

// UnregisterProbe unregisters gopacket probe
func (p *OvsMirrorProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	return common.ErrNotImplemented
}

// Start probe
func (p *OvsMirrorProbesHandler) Start() {
}

// Stop probe
func (p *OvsMirrorProbesHandler) Stop() {
}

// NewOvsMirrorProbesHandler creates a new OVS Mirror probes
func NewOvsMirrorProbesHandler(g *graph.Graph, tb, fb *probe.Bundle) (*OvsMirrorProbesHandler, error) {
	return nil, common.ErrNotImplemented
}
