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
	"github.com/skydive-project/skydive/sflow"
)

const (
	defaultPort = 6343
)

type sflowProbe struct {
	tid string
	ft  *flow.Table
}

// SFlowProbesHandler describes a SFlow probe in the graph
type SFlowProbesHandler struct {
	graph       *graph.Graph
	fta         *flow.TableAllocator
	allocator   *sflow.AgentAllocator
	staticPorts map[string]string
}

// UnregisterProbe unregisters a probe from the graph
func (d *SFlowProbesHandler) UnregisterProbe(n *graph.Node, e ProbeEventHandler, p Probe) error {
	probe := p.(*sflowProbe)

	d.fta.Release(probe.ft)
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
	ft := d.fta.Alloc(uuids, tableOptsFromCapture(capture))

	addr := common.ServiceAddress{Addr: address, Port: port}
	if _, err := d.allocator.Alloc(tid, ft, capture.BPFFilter, headerSize, &addr, n, d.graph); err != nil {
		return nil, err
	}

	go e.OnStarted(&CaptureMetadata{SFlowSocket: addr.String()})

	return &sflowProbe{
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

// NewSFlowProbesHandler creates a new SFlow probe in the graph
func NewSFlowProbesHandler(g *graph.Graph, fta *flow.TableAllocator) (*SFlowProbesHandler, error) {
	allocator, err := sflow.NewAgentAllocator()
	if err != nil {
		return nil, err
	}

	return &SFlowProbesHandler{
		graph:     g,
		fta:       fta,
		allocator: allocator,
	}, nil
}
