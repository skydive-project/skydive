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
	"runtime"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/api"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
	"github.com/redhat-cip/skydive/topology/probes"
	"github.com/vishvananda/netns"
)

type PcapProbe struct {
	handle              *pcap.Handle
	channel             chan gopacket.Packet
	probeNodeUUID       string
	analyzerClient      *analyzer.Client
	flowTable           *flow.Table
	flowMappingPipeline *mappings.FlowMappingPipeline
	flowTableAllocator  *flow.TableAllocator
}

type PcapProbesHandler struct {
	graph               *graph.Graph
	analyzerClient      *analyzer.Client
	flowMappingPipeline *mappings.FlowMappingPipeline
	flowTableAllocator  *flow.TableAllocator
	wg                  sync.WaitGroup
	probes              map[string]*PcapProbe
	probesLock          sync.RWMutex
}

const (
	snaplen int32 = 256
)

func (p *PcapProbe) SetProbeNode(flow *flow.Flow) bool {
	flow.ProbeNodeUUID = p.probeNodeUUID
	return true
}

func (p *PcapProbe) asyncFlowPipeline(flows []*flow.Flow) {
	if p.flowMappingPipeline != nil {
		p.flowMappingPipeline.Enhance(flows)
	}
	if p.analyzerClient != nil {
		p.analyzerClient.SendFlows(flows)
	}
}

func (p *PcapProbe) start() {
	p.flowTable = p.flowTableAllocator.Alloc()
	defer p.flowTable.UnregisterAll()
	defer p.flowTableAllocator.Release(p.flowTable)

	agentExpire := config.GetAgentExpire()
	p.flowTable.RegisterExpire(p.asyncFlowPipeline, agentExpire, agentExpire)

	agentUpdate := config.GetAgentUpdate()
	p.flowTable.RegisterUpdated(p.asyncFlowPipeline, agentUpdate, agentUpdate)

	feedFlowTable := func() {
		select {
		case packet, ok := <-p.channel:
			if ok {
				flow.FlowFromGoPacket(p.flowTable, &packet, p)
			}
		}
	}
	p.flowTable.RegisterDefault(feedFlowTable)

	p.flowTable.Start()
}

func (p *PcapProbe) stop() {
	p.handle.Close()
	p.flowTable.Stop()
}

func (p *PcapProbesHandler) RegisterProbe(n *graph.Node, capture *api.Capture) error {
	logging.GetLogger().Debugf("Starting pcap capture on %s", n.Metadata()["Name"])

	if name, ok := n.Metadata()["Name"]; ok && name != "" {
		ifName := name.(string)

		if _, ok := p.probes[ifName]; ok {
			return errors.New(fmt.Sprintf("A pcap probe already exists for %s", ifName))
		}

		nodes := p.graph.LookupShortestPath(n, graph.Metadata{"Type": "host"}, graph.Metadata{"RelationType": "ownership"})
		if len(nodes) == 0 {
			return errors.New(fmt.Sprintf("Failed to determine probePath for %s", ifName))
		}

		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		origns, err := netns.Get()
		if err != nil {
			return fmt.Errorf("Error while getting current ns: %s", err.Error())
		}
		defer origns.Close()

		for _, node := range nodes {
			if node.Metadata()["Type"] == "netns" {
				name := node.Metadata()["Name"].(string)
				path := node.Metadata()["Path"].(string)
				logging.GetLogger().Debugf("Switching to namespace %s (path: %s)", name, path)

				newns, err := netns.GetFromPath(path)
				if err != nil {
					return fmt.Errorf("Error while opening ns %s (path: %s): %s", name, path, err.Error())
				}
				defer newns.Close()

				if err := netns.Set(newns); err != nil {
					return fmt.Errorf("Error while switching from root ns to %s (path: %s): %s", name, path, err.Error())
				}
				defer netns.Set(origns)
			}
		}

		handle, err := pcap.OpenLive(ifName, snaplen, true, time.Second)
		if err != nil {
			return fmt.Errorf("Error while opening device %s: %s", ifName, err.Error())
		}

		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		packetChannel := packetSource.Packets()

		probe := &PcapProbe{
			handle:              handle,
			channel:             packetChannel,
			probeNodeUUID:       string(n.ID),
			flowMappingPipeline: p.flowMappingPipeline,
			flowTableAllocator:  p.flowTableAllocator,
			analyzerClient:      p.analyzerClient,
		}
		p.probesLock.Lock()
		p.probes[ifName] = probe
		p.probesLock.Unlock()
		p.wg.Add(1)

		go func() {
			defer p.wg.Done()

			probe.start()
		}()
	}
	return nil
}

func (p *PcapProbesHandler) unregisterProbe(ifName string) error {
	if probe, ok := p.probes[ifName]; ok {
		logging.GetLogger().Debugf("Terminating pcap capture on %s", ifName)
		probe.stop()
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

func NewPcapProbesHandler(tb *probes.TopologyProbeBundle, g *graph.Graph,
	p *mappings.FlowMappingPipeline, a *analyzer.Client, fta *flow.TableAllocator) *PcapProbesHandler {
	handler := &PcapProbesHandler{
		graph:               g,
		analyzerClient:      a,
		flowMappingPipeline: p,
		flowTableAllocator:  fta,
		probes:              make(map[string]*PcapProbe),
	}
	return handler
}
