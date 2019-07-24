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
	"fmt"
	"strings"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/sflow"
)

const (
	defaultPort = 6343
)

type sFlowProbe struct {
	tid string
	ft  *flow.Table
}

// SFlowProbesHandler describes a sFlow probe in the graph
type SFlowProbesHandler struct {
	Ctx         Context
	allocator   *sflow.AgentAllocator
	staticPorts map[string]string
}

// UnregisterProbe unregisters a probe from the graph
func (d *SFlowProbesHandler) UnregisterProbe(n *graph.Node, e ProbeEventHandler, p Probe) error {
	probe := p.(*sFlowProbe)

	d.Ctx.FTA.Release(probe.ft)
	d.allocator.Release(probe.tid)

	if e != nil {
		go e.OnStopped()
	}

	return nil
}

// RegisterProbe registers a probe in the graph
func (d *SFlowProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e ProbeEventHandler) (Probe, error) {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return nil, fmt.Errorf("No TID for node %v", n)
	}

	addresses, _ := n.GetFieldStringList("IPV4")
	if len(addresses) == 0 {
		return nil, fmt.Errorf("No IP for node %v", n)
	}

	address := "0.0.0.0"
	if len(addresses) == 1 {
		address = strings.Split(addresses[0], "/")[0]
	}

	port := capture.Port
	if capture.Port <= 0 {
		port = defaultPort
	}

	headerSize := flow.DefaultCaptureLength
	if capture.HeaderSize != 0 {
		headerSize = uint32(capture.HeaderSize)
	}

	uuids := flow.UUIDs{NodeTID: tid, CaptureID: capture.UUID}
	ft := d.Ctx.FTA.Alloc(uuids, tableOptsFromCapture(capture))

	addr := common.ServiceAddress{Addr: address, Port: port}
	if _, err := d.allocator.Alloc(tid, ft, capture.BPFFilter, headerSize, &addr, n, d.Ctx.Graph); err != nil {
		return nil, err
	}

	go e.OnStarted(&CaptureMetadata{SFlowSocket: addr.String()})

	return &sFlowProbe{
		ft:  ft,
		tid: tid,
	}, nil
}

// Start a probe
func (d *SFlowProbesHandler) Start() {
}

// Stop a probe
func (d *SFlowProbesHandler) Stop() {
	d.allocator.ReleaseAll()
}

// CaptureTypes supported
func (d *SFlowProbesHandler) CaptureTypes() []string {
	return []string{"sflow"}
}

// Init initializes a new sFlow probe
func (d *SFlowProbesHandler) Init(ctx Context, bundle *probe.Bundle) (FlowProbeHandler, error) {
	allocator, err := sflow.NewAgentAllocator()
	if err != nil {
		return nil, err
	}

	d.Ctx = ctx
	d.allocator = allocator

	return d, nil
}
