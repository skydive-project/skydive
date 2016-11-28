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
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

type packetHandle interface {
	Close()
}

type GoPacketProbe struct {
	handle       packetHandle
	packetSource *gopacket.PacketSource
	NodeTID      string
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

func (p *GoPacketProbe) feedFlowTable(packetsChan chan flow.FlowPackets) {
	for atomic.LoadInt64(&p.state) == common.RunningState {
		packet, err := p.packetSource.NextPacket()
		if err == io.EOF {
			time.Sleep(20 * time.Millisecond)
		} else if err == nil {
			if packets := flow.FlowPacketsFromGoPacket(&packet, 0); len(packets) > 0 {
				packetsChan <- packets
			}
		} else {
			// sleep awhile in case of error to reduce the presure on cpu
			time.Sleep(100 * time.Millisecond)
		}
	}

	p.handle.Close()
}

func (p *GoPacketProbe) run(g *graph.Graph, n *graph.Node, capture *api.Capture) error {
	atomic.StoreInt64(&p.state, common.RunningState)

	g.RLock()
	ifName := n.Metadata()["Name"].(string)
	firstLayerType := getGoPacketFirstLayerType(n)

	nscontext, err := topology.NewNetNSContextByNode(g, n)
	g.RUnlock()

	defer nscontext.Close()

	if err != nil {
		return err
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

		p.handle = handle
		p.packetSource = gopacket.NewPacketSource(handle, handle.LinkType())

		logging.GetLogger().Infof("PCAP Capture started on %s with First layer: %s", ifName, firstLayerType)
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

		p.handle = handle
		p.packetSource = gopacket.NewPacketSource(handle, firstLayerType)

		logging.GetLogger().Infof("AfPacket Capture started on %s with First layer: %s", ifName, firstLayerType)
	}

	// leave the namespace, stay lock in the current thread
	nscontext.Quit()

	packetsChan := p.flowTable.Start()
	defer p.flowTable.Stop()

	p.feedFlowTable(packetsChan)

	return nil
}

func (p *GoPacketProbe) stop() {
	atomic.StoreInt64(&p.state, common.StoppingState)
}

func getGoPacketFirstLayerType(n *graph.Node) gopacket.LayerType {
	if encapType, ok := n.Metadata()["EncapType"]; ok {
		switch encapType.(string) {
		case "ether":
			return layers.LayerTypeEthernet
		case "gre":
			return flow.LayerTypeInGRE
		case "sit", "ipip":
			return layers.LayerTypeIPv4
		case "tunnel6", "gre6":
			return layers.LayerTypeIPv6
		default:
			logging.GetLogger().Warningf("Encapsulation unknown %s on link %s, defaulting to Ethernet", encapType, n.Metadata()["Name"])
		}
	} else {
		logging.GetLogger().Warningf("EncapType not found on link %s, defaulting to Ethernet", n.Metadata()["Name"])
	}
	return layers.LayerTypeEthernet
}

func (p *GoPacketProbesHandler) RegisterProbe(n *graph.Node, capture *api.Capture, ft *flow.Table) error {
	name, ok := n.Metadata()["Name"]
	if !ok || name == "" {
		return fmt.Errorf("No name for node %v", n)
	}

	encapType, ok := n.Metadata()["EncapType"]
	if !ok || encapType == "" {
		return fmt.Errorf("No EncapType for node %v", n)
	}

	tid, ok := n.Metadata()["TID"]
	if !ok {
		return fmt.Errorf("No TID for node %v", n)
	}

	id := string(n.ID)
	ifName := name.(string)

	if _, ok = p.probes[id]; ok {
		return fmt.Errorf("Already registered %s", ifName)
	}

	port, ok := n.Metadata()["MPLSUDPPort"].(int)
	if ok {
		// All gopacket instance of this agent will classify UDP packets coming
		// from UDP port MPLSUDPPort as MPLS whatever the source interface
		layers.RegisterUDPPortLayerType(layers.UDPPort(port), layers.LayerTypeMPLS)
		logging.GetLogger().Infof("MPLSoUDP port: %v", port)
	}

	probe := &GoPacketProbe{
		NodeTID:   tid.(string),
		state:     common.StoppedState,
		flowTable: ft,
	}

	p.probesLock.Lock()
	p.probes[id] = probe
	p.probesLock.Unlock()
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		probe.run(p.graph, n, capture)
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
