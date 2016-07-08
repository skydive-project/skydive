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
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/redhat-cip/skydive/api"
	"github.com/redhat-cip/skydive/common"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
	"github.com/vishvananda/netns"
)

type PcapProbe struct {
	handle        *pcap.Handle
	probeNodeUUID string
	flowTable     *flow.Table
	state         int64
}

type PcapProbesHandler struct {
	graph      *graph.Graph
	wg         sync.WaitGroup
	probes     map[string]*PcapProbe
	probesLock sync.RWMutex
}

const (
	snaplen int32 = 256
)

func (p *PcapProbe) SetProbeNode(flow *flow.Flow) bool {
	flow.ProbeNodeUUID = p.probeNodeUUID
	return true
}

func (p *PcapProbe) packetsToChan(ch chan gopacket.Packet) {
	defer close(ch)

	packetSource := gopacket.NewPacketSource(p.handle, p.handle.LinkType())
	for atomic.LoadInt64(&p.state) == common.RunningState {
		packet, err := packetSource.NextPacket()
		if err == io.EOF {
			time.Sleep(20 * time.Millisecond)
		} else if err == nil {
			ch <- packet
		}
	}
}

func (p *PcapProbe) start() {
	ch := make(chan gopacket.Packet, 1000)
	go p.packetsToChan(ch)

	timer := time.NewTicker(100 * time.Millisecond)
	feedFlowTable := func() {
		select {
		case packet, ok := <-ch:
			if ok {
				flow.FlowFromGoPacket(p.flowTable, &packet, 0, p)
			}
		case <-timer.C:
		}
	}
	p.flowTable.RegisterDefault(feedFlowTable)

	atomic.StoreInt64(&p.state, common.RunningState)
	p.flowTable.Start()
}

func (p *PcapProbe) stop() {
	if atomic.CompareAndSwapInt64(&p.state, common.RunningState, common.StoppingState) {
		p.flowTable.Stop()
		p.handle.Close()
	}
}

func (p *PcapProbesHandler) RegisterProbe(n *graph.Node, capture *api.Capture, ft *flow.Table) error {
	name, ok := n.Metadata()["Name"]
	if !ok || name == "" {
		return fmt.Errorf("No name for node %v", n)
	}

	id := string(n.ID)
	ifName := name.(string)

	if _, ok := p.probes[id]; ok {
		return fmt.Errorf("Already registered %s", ifName)
	}

	nodes := p.graph.LookupShortestPath(n, graph.Metadata{"Type": "host"}, graph.Metadata{"RelationType": "ownership"})
	if len(nodes) == 0 {
		return fmt.Errorf("Failed to determine probePath for %s", ifName)
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

	logging.GetLogger().Debugf("PCAP capture started on %s", n.Metadata()["Name"])

	probe := &PcapProbe{
		handle:        handle,
		probeNodeUUID: id,
		state:         common.StoppedState,
		flowTable:     ft,
	}
	p.probesLock.Lock()
	p.probes[id] = probe
	p.probesLock.Unlock()
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		probe.start()
	}()

	return nil
}

func (p *PcapProbesHandler) unregisterProbe(id string) error {
	if probe, ok := p.probes[id]; ok {
		logging.GetLogger().Debugf("Terminating pcap capture on %s", id)
		probe.stop()
		delete(p.probes, id)
	}

	return nil
}

func (p *PcapProbesHandler) UnregisterProbe(n *graph.Node) error {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	err := p.unregisterProbe(string(n.ID))
	if err != nil {
		return err
	}

	return nil
}

func (p *PcapProbesHandler) Start() {
}

func (p *PcapProbesHandler) Stop() {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	for id := range p.probes {
		p.unregisterProbe(id)
	}
	p.wg.Wait()
}

func NewPcapProbesHandler(g *graph.Graph) *PcapProbesHandler {
	handler := &PcapProbesHandler{
		graph:  g,
		probes: make(map[string]*PcapProbe),
	}
	return handler
}
