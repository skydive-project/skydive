/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/api"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
	"github.com/redhat-cip/skydive/topology/probes"
)

type PcapProbe struct {
	handle    *pcap.Handle
	channel   chan gopacket.Packet
	probePath string
}

type PcapProbesHandler struct {
	graph               *graph.Graph
	analyzerClient      *analyzer.Client
	flowTable           *flow.FlowTable
	flowMappingPipeline *mappings.FlowMappingPipeline
	wg                  sync.WaitGroup
	probes              map[string]*PcapProbe
	probesLock          sync.RWMutex
}

const (
	snaplen int32 = 256
)

func (p *PcapProbesHandler) handlePacket(pcapProbe *PcapProbe, packet gopacket.Packet) {
	flows := []*flow.Flow{flow.FlowFromGoPacket(p.flowTable, &packet, pcapProbe.probePath)}
	p.flowTable.Update(flows)
	p.flowMappingPipeline.Enhance(flows)

	if p.analyzerClient != nil {
		// FIX(safchain) add flow state cache in order to send only flow changes
		// to not flood the analyzer
		p.analyzerClient.SendFlows(flows)
	}
}

func (p *PcapProbesHandler) RegisterProbe(n *graph.Node, capture *api.Capture) error {
	logging.GetLogger().Debugf("Starting pcap capture on %s", n.Metadata()["Name"])

	if name, ok := n.Metadata()["Name"]; ok && name != "" {
		ifName := name.(string)

		if _, ok := p.probes[ifName]; ok {
			return errors.New(fmt.Sprintf("A pcap probe already exists for %s", ifName))
		}

		nodes := p.graph.LookupShortestPath(n, graph.Metadata{"Type": "host"}, topology.IsOwnershipEdge)
		if len(nodes) == 0 {
			return errors.New(fmt.Sprintf("Failed to determine probePath for %s", ifName))
		}

		handle, err := pcap.OpenLive(ifName, snaplen, true, time.Second)
		if err != nil {
			return err
		}

		if capture.BPFFilter != "" {
			handle.SetBPFFilter(capture.BPFFilter)
		}

		probePath := topology.NodePath{nodes}.Marshal()

		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		packetChannel := packetSource.Packets()

		probe := &PcapProbe{
			handle:    handle,
			channel:   packetChannel,
			probePath: probePath,
		}

		p.probesLock.Lock()
		p.probes[ifName] = probe
		p.probesLock.Unlock()

		p.wg.Add(1)
		go func() {
			defer p.wg.Done()

			for packet := range probe.channel {
				p.handlePacket(probe, packet)
			}
		}()
	}
	return nil
}

func (p *PcapProbesHandler) unregisterProbe(ifName string) error {
	if probe, ok := p.probes[ifName]; ok {
		probe.handle.Close()
		delete(p.probes, ifName)
	}

	return nil
}

func (p *PcapProbesHandler) UnregisterProbe(n *graph.Node) error {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	if name, ok := n.Metadata()["Name"]; ok && name != "" {
		ifName := name.(string)
		err := p.unregisterProbe(ifName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *PcapProbesHandler) Start() {
}

func (p *PcapProbesHandler) Stop() {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	for name := range p.probes {
		p.unregisterProbe(name)
	}
	p.wg.Wait()
}

func (o *PcapProbesHandler) Flush() {
}

func NewPcapProbesHandler(tb *probes.TopologyProbeBundle, g *graph.Graph, p *mappings.FlowMappingPipeline, a *analyzer.Client) *PcapProbesHandler {
	handler := &PcapProbesHandler{
		graph:               g,
		analyzerClient:      a,
		flowMappingPipeline: p,
		flowTable:           flow.NewFlowTable(),
		probes:              make(map[string]*PcapProbe),
	}
	return handler
}
