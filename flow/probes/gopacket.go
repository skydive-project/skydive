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
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/vishvananda/netns"
)

type packetHandle interface {
	Close()
}

type GoPacketProbe struct {
	handle       packetHandle
	packetSource *gopacket.PacketSource
	NodeUUID     string
	flowTable    *flow.Table
	state        int64
}

type GoPacketProbesHandler struct {
	graph      *graph.Graph
	wg         sync.WaitGroup
	probes     map[string]*GoPacketProbe
	probesLock sync.RWMutex
}

const (
	snaplen int32 = 256
)

func (p *GoPacketProbe) SetProbeNode(flow *flow.Flow) bool {
	flow.NodeUUID = p.NodeUUID
	return true
}

func (p *GoPacketProbe) packetsToChan(ch chan gopacket.Packet) {
	defer close(ch)

	for atomic.LoadInt64(&p.state) == common.RunningState {
		packet, err := p.packetSource.NextPacket()
		if err == io.EOF {
			time.Sleep(20 * time.Millisecond)
		} else if err == nil {
			ch <- packet
		} else {
			// sleep awhile in case of error to reduce the presure on cpu
			time.Sleep(100 * time.Millisecond)
		}
	}

	p.handle.Close()
}

func (p *GoPacketProbe) start() {
	atomic.StoreInt64(&p.state, common.RunningState)

	ch := make(chan gopacket.Packet, 1000)
	go p.packetsToChan(ch)

	timer := time.NewTicker(100 * time.Millisecond)
	feedFlowTable := func() {
		select {
		case packet, ok := <-ch:
			if ok {
				flow.FlowsFromGoPacket(p.flowTable, &packet, 0, p)
			}
		case <-timer.C:
		}
	}
	p.flowTable.RegisterDefault(feedFlowTable)
	p.flowTable.Start()
}

func (p *GoPacketProbe) stop() {
	if atomic.CompareAndSwapInt64(&p.state, common.RunningState, common.StoppingState) {
		p.flowTable.Stop()
	}
}

func getGoPacketFirstLayerType(n *graph.Node) gopacket.LayerType {
	switch n.Metadata()["Type"] {
	case "bridge", "bond", "can", "dummy", "hsr", "ifb", "macvlan", "macvtap",
		"veth", "vlan", "vxlan", "gretap", "ip6gretap", "geneve":
		return layers.LayerTypeEthernet
	case "ipoib", "vcan", "ipip", "ipvlan", "lowpan":
		return layers.LayerTypeIPv4
	case "ip6tnl", "ip6gre", "sit":
		return layers.LayerTypeIPv6
	}
	return layers.LayerTypeEthernet
}

func (p *GoPacketProbesHandler) RegisterProbe(n *graph.Node, capture *api.Capture, ft *flow.Table) error {
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

	probe := &GoPacketProbe{
		NodeUUID:  id,
		state:     common.StoppedState,
		flowTable: ft,
	}

	switch capture.Type {
	case "pcap":
		handle, err := pcap.OpenLive(ifName, snaplen, true, time.Second)
		if err != nil {
			return fmt.Errorf("Error while opening device %s: %s", ifName, err.Error())
		}

		if err := handle.SetBPFFilter(capture.BPFFilter); err != nil {
			return fmt.Errorf("BPF Filter failed: %s", err)
		}

		probe.handle = handle
		probe.packetSource = gopacket.NewPacketSource(handle, getGoPacketFirstLayerType(n))

		logging.GetLogger().Infof("PCAP Capture type %s started on %s", capture.Type, n.Metadata()["Name"])
	default:
		var handle *AFPacketHandle
		fnc := func() error {
			handle, err = NewAFPacketHandle(ifName, snaplen)
			if err != nil {
				return fmt.Errorf("Error while opening device %s: %s", ifName, err.Error())
			}
			return nil
		}

		if err = common.Retry(fnc, 2, 100*time.Millisecond); err != nil {
			return err
		}

		probe.handle = handle
		probe.packetSource = gopacket.NewPacketSource(handle, getGoPacketFirstLayerType(n))

		logging.GetLogger().Infof("AfPacket Capture started on %s", n.Metadata()["Name"])
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

func (p *GoPacketProbesHandler) unregisterProbe(id string) error {
	if probe, ok := p.probes[id]; ok {
		logging.GetLogger().Debugf("Terminating gopacket capture on %s", id)
		probe.stop()
		delete(p.probes, id)
	}

	return nil
}

func (p *GoPacketProbesHandler) UnregisterProbe(n *graph.Node) error {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	err := p.unregisterProbe(string(n.ID))
	if err != nil {
		return err
	}

	return nil
}

func (p *GoPacketProbesHandler) Start() {
}

func (p *GoPacketProbesHandler) Stop() {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	for id := range p.probes {
		p.unregisterProbe(id)
	}
	p.wg.Wait()
}

func NewGoPacketProbesHandler(g *graph.Graph) *GoPacketProbesHandler {
	handler := &GoPacketProbesHandler{
		graph:  g,
		probes: make(map[string]*GoPacketProbe),
	}
	return handler
}
