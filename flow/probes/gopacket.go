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
	"github.com/skydive-project/skydive/config"
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

func pcapUpdateStats(g *graph.Graph, n *graph.Node, handle *pcap.Handle, ticker *time.Ticker) {
	for _ = range ticker.C {
		if stats, e := handle.Stats(); e != nil {
			logging.GetLogger().Errorf("Can not get pcap capture stats")
		} else {
			g.Lock()
			t := g.StartMetadataTransaction(n)
			t.AddMetadata("Capture/PacketsReceived", stats.PacketsReceived)
			t.AddMetadata("Capture/PacketsDropped", stats.PacketsDropped)
			t.AddMetadata("Capture/PacketsIfDropped", stats.PacketsIfDropped)
			t.Commit()
			g.Unlock()
		}
	}
}

func (p *GoPacketProbe) feedFlowTable(packetsChan chan *flow.FlowPackets) {
	for atomic.LoadInt64(&p.state) == common.RunningState {
		packet, err := p.packetSource.NextPacket()
		if err == io.EOF {
			time.Sleep(20 * time.Millisecond)
		} else if err == nil {
			if flowPackets := flow.FlowPacketsFromGoPacket(&packet, 0, -1); len(flowPackets.Packets) > 0 {
				packetsChan <- flowPackets
			}
		} else {
			// sleep awhile in case of error to reduce the presure on cpu
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (p *GoPacketProbe) run(g *graph.Graph, n *graph.Node, capture *api.Capture) {
	var ticker *time.Ticker
	atomic.StoreInt64(&p.state, common.RunningState)

	g.RLock()
	ifName, _ := n.GetFieldString("Name")
	if ifName == "" {
		g.RUnlock()
		logging.GetLogger().Errorf("No name for node %v", n)
		return
	}

	firstLayerType := getGoPacketFirstLayerType(n)

	nscontext, err := topology.NewNetNSContextByNode(g, n)
	g.RUnlock()

	defer nscontext.Close()

	if err != nil {
		logging.GetLogger().Error(err)
		return
	}

	switch capture.Type {
	case "pcap":
		handle, err := pcap.OpenLive(ifName, snaplen, true, time.Second)
		if err != nil {
			logging.GetLogger().Errorf("Error while opening device %s: %s", ifName, err.Error())
			return
		}

		if err := handle.SetBPFFilter(capture.BPFFilter); err != nil {
			logging.GetLogger().Errorf("BPF Filter failed: %s", err)
			return
		}

		p.handle = handle
		p.packetSource = gopacket.NewPacketSource(handle, handle.LinkType())

		// Go routine to update the interface statistics
		statsUpdate := config.GetConfig().GetInt("agent.flow.stats_update")
		ticker = time.NewTicker(time.Duration(statsUpdate) * time.Second)
		go pcapUpdateStats(g, n, handle, ticker)

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
			logging.GetLogger().Error(err)
			return
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
	if ticker != nil {
		ticker.Stop()
	}
	p.handle.Close()
}

func (p *GoPacketProbe) stop() {
	atomic.StoreInt64(&p.state, common.StoppingState)
}

func getGoPacketFirstLayerType(n *graph.Node) gopacket.LayerType {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return layers.LayerTypeEthernet
	}

	if encapType, err := n.GetFieldString("EncapType"); err == nil {
		switch encapType {
		case "ether":
			return layers.LayerTypeEthernet
		case "gre":
			return flow.LayerTypeInGRE
		case "sit", "ipip":
			return layers.LayerTypeIPv4
		case "tunnel6", "gre6":
			return layers.LayerTypeIPv6
		default:
			logging.GetLogger().Warningf("Encapsulation unknown %s on link %s, defaulting to Ethernet", encapType, name)
		}
	} else {
		logging.GetLogger().Warningf("EncapType not found on link %s, defaulting to Ethernet", name)
	}
	return layers.LayerTypeEthernet
}

func (p *GoPacketProbesHandler) RegisterProbe(n *graph.Node, capture *api.Capture, ft *flow.Table) error {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return fmt.Errorf("No name for node %v", n)
	}

	if state, _ := n.GetFieldString("State"); capture.Type == "pcap" && state != "UP" {
		return fmt.Errorf("Can't start pcap capture on node down %s", name)
	}

	encapType, _ := n.GetFieldString("EncapType")
	if encapType == "" {
		return fmt.Errorf("No EncapType for node %v", n)
	}

	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}

	id := string(n.ID)

	if _, ok := p.probes[id]; ok {
		return fmt.Errorf("Already registered %s", name)
	}

	if port, err := n.GetFieldInt64("MPLSUDPPort"); err == nil {
		// All gopacket instance of this agent will classify UDP packets coming
		// from UDP port MPLSUDPPort as MPLS whatever the source interface
		layers.RegisterUDPPortLayerType(layers.UDPPort(port), layers.LayerTypeMPLS)
		logging.GetLogger().Infof("MPLSoUDP port: %v", port)
	}

	probe := &GoPacketProbe{
		NodeTID:   tid,
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

func NewGoPacketProbesHandler(g *graph.Graph) (*GoPacketProbesHandler, error) {
	return &GoPacketProbesHandler{
		graph:  g,
		probes: make(map[string]*GoPacketProbe),
	}, nil
}
