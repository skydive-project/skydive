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
	"fmt"
	"strings"
	"sync"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/sflow"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	defaultPort = 6343
)

// SFlowProbesHandler describes a SFlow probe in the graph
type SFlowProbesHandler struct {
	Graph      *graph.Graph
	fpta       *FlowProbeTableAllocator
	probes     map[string]*flow.Table
	probesLock sync.RWMutex
	allocator  *sflow.SFlowAgentAllocator
}

// UnregisterProbe unregisters a probe from the graph
func (d *SFlowProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	d.probesLock.Lock()
	defer d.probesLock.Unlock()

	var tid string
	if tid, _ = n.GetFieldString("TID"); tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}

	ft, ok := d.probes[tid]
	if !ok {
		return fmt.Errorf("No registered probe for %s", tid)
	}
	d.fpta.Release(ft)

	d.allocator.Release(tid)

	delete(d.probes, tid)

	if e != nil {
		go e.OnStopped()
	}

	return nil
}

// RegisterProbe registers a probe in the graph
func (d *SFlowProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	var tid string
	if tid, _ = n.GetFieldString("TID"); tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}

	if _, ok := d.probes[tid]; ok {
		return fmt.Errorf("Already registered %s", tid)
	}

	addresses, _ := n.GetFieldStringList("IPV4")
	if len(addresses) == 0 {
		return fmt.Errorf("No IP for node %v", n)
	}

	address := "0.0.0.0"
	if len(addresses) == 1 {
		address = strings.Split(addresses[0], "/")[0]
	}

	if capture.Port <= 0 {
		capture.Port = defaultPort
	}

	headerSize := flow.DefaultCaptureLength
	if capture.HeaderSize != 0 {
		headerSize = uint32(capture.HeaderSize)
	}

	opts := flow.TableOpts{
		RawPacketLimit: int64(capture.RawPacketLimit),
		TCPMetric:      capture.ExtraTCPMetric,
		SocketInfo:     capture.SocketInfo,
	}
	ft := d.fpta.Alloc(tid, opts)

	addr := common.ServiceAddress{Addr: address, Port: capture.Port}
	if _, err := d.allocator.Alloc(tid, ft, capture.BPFFilter, headerSize, &addr); err != nil {
		return err
	}

	d.probesLock.Lock()
	d.probes[tid] = ft
	d.probesLock.Unlock()

	go e.OnStarted()

	d.Graph.AddMetadata(n, "Capture.SflowSocket", addr.String())

	return nil
}

// Start a probe
func (d *SFlowProbesHandler) Start() {
}

// Stop a probe
func (d *SFlowProbesHandler) Stop() {
	d.probesLock.Lock()
	for _, ft := range d.probes {
		d.fpta.Release(ft)
	}
	d.probesLock.Unlock()
	d.allocator.ReleaseAll()
}

// NewSFlowProbesHandler creates a new SFlow probe in the graph
func NewSFlowProbesHandler(g *graph.Graph, fpta *FlowProbeTableAllocator) (*SFlowProbesHandler, error) {
	allocator, err := sflow.NewSFlowAgentAllocator()
	if err != nil {
		return nil, err
	}

	return &SFlowProbesHandler{
		Graph:     g,
		fpta:      fpta,
		allocator: allocator,
		probes:    make(map[string]*flow.Table),
	}, nil
}
