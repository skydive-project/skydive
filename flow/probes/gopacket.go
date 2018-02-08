// +build linux

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

	"golang.org/x/net/bpf"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes/afpacket"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

type packetHandle interface {
	Close()
}

// GoPacketProbe describes a new probe that store packets from gopacket pcap library in a flowtable
type GoPacketProbe struct {
	handle       packetHandle
	packetSource *gopacket.PacketSource
	NodeTID      string
	flowTable    *flow.Table
	state        int64
}

// GoPacketProbesHandler describes a flow probe handle in the graph
type GoPacketProbesHandler struct {
	graph      *graph.Graph
	fpta       *FlowProbeTableAllocator
	wg         sync.WaitGroup
	probes     map[string]*GoPacketProbe
	probesLock sync.RWMutex
}

func (p *GoPacketProbe) pcapUpdateStats(g *graph.Graph, n *graph.Node, handle *pcap.Handle, ticker *time.Ticker, done chan bool, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ticker.C:
			if stats, e := handle.Stats(); e != nil {
				logging.GetLogger().Errorf("Can not get pcap capture stats")
			} else if atomic.LoadInt64(&p.state) == common.RunningState {
				g.Lock()
				t := g.StartMetadataTransaction(n)
				t.AddMetadata("Capture.PacketsReceived", stats.PacketsReceived)
				t.AddMetadata("Capture.PacketsDropped", stats.PacketsDropped)
				t.AddMetadata("Capture.PacketsIfDropped", stats.PacketsIfDropped)
				t.Commit()
				g.Unlock()
			}
		case <-done:
			return
		}
	}
}

func (p *GoPacketProbe) afpacketUpdateStats(g *graph.Graph, n *graph.Node, handle *AFPacketHandle, ticker *time.Ticker, done chan bool, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ticker.C:
			if _, v3, e := handle.tpacket.SocketStats(); e != nil {
				logging.GetLogger().Errorf("Can not get pcap capture stats")
			} else if atomic.LoadInt64(&p.state) == common.RunningState {
				g.Lock()
				t := g.StartMetadataTransaction(n)
				t.AddMetadata("Capture.PacketsReceived", v3.Packets())
				t.AddMetadata("Capture.PacketsDropped", v3.Drops())
				t.Commit()
				g.Unlock()
			}
		case <-done:
			return
		}
	}
}

func (p *GoPacketProbe) feedFlowTable(packetSeqChan chan *flow.PacketSequence, bpf *flow.BPF) {
	var count int

	for atomic.LoadInt64(&p.state) == common.RunningState {
		packet, err := p.packetSource.NextPacket()
		switch err {
		case nil:
			if ps := flow.PacketSeqFromGoPacket(&packet, 0, bpf); len(ps.Packets) > 0 {
				packetSeqChan <- ps
			}
		case io.EOF:
			time.Sleep(20 * time.Millisecond)
		case afpacket.ErrTimeout:
			// nothing to do, poll wait for new packet or timeout
		default:
			time.Sleep(200 * time.Millisecond)
		}

		// NOTE: bpf usperspace filter is applied to the few first packets in order to avoid
		// to get unexpected packets between capture start and bpf applying
		if count > 50 {
			bpf = nil
		}
		count++
	}
}

func (p *GoPacketProbe) run(g *graph.Graph, n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) {
	atomic.StoreInt64(&p.state, common.RunningState)

	headerSize := flow.DefaultCaptureLength
	if capture.HeaderSize != 0 {
		headerSize = uint32(capture.HeaderSize)
	}

	g.RLock()
	ifName, _ := n.GetFieldString("Name")
	if ifName == "" {
		g.RUnlock()
		logging.GetLogger().Errorf("No name for node %v", n)
		return
	}

	firstLayerType, linkType := getGoPacketFirstLayerType(n)

	nscontext, err := topology.NewNetNSContextByNode(g, n)
	g.RUnlock()

	defer nscontext.Close()

	if err != nil {
		logging.GetLogger().Error(err)
		return
	}

	// Apply temporary the pbf in the userspace to prevent non expected packet
	// between capture creation and the filter apply.
	var bpfFilter *flow.BPF
	if capture.BPFFilter != "" {
		bpfFilter, err = flow.NewBPF(linkType, headerSize, capture.BPFFilter)
		if err != nil {
			logging.GetLogger().Error(err)
			return
		}
	}

	var wg sync.WaitGroup
	statsDone := make(chan bool)

	// Go routine to update the interface statistics
	statsUpdate := config.GetInt("agent.flow.stats_update")
	statsTicker := time.NewTicker(time.Duration(statsUpdate) * time.Second)

	switch capture.Type {
	case "pcap":
		handle, err := pcap.OpenLive(ifName, int32(headerSize), true, time.Second)
		if err != nil {
			logging.GetLogger().Errorf("Error while opening device %s: %s", ifName, err.Error())
			return
		}

		p.handle = handle
		p.packetSource = gopacket.NewPacketSource(handle, handle.LinkType())

		wg.Add(1)
		go p.pcapUpdateStats(g, n, handle, statsTicker, statsDone, &wg)

		logging.GetLogger().Infof("PCAP Capture started on %s with First layer: %s", ifName, firstLayerType)
	default:
		var handle *AFPacketHandle
		fnc := func() error {
			handle, err = NewAFPacketHandle(ifName, int32(headerSize))
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

		wg.Add(1)
		go p.afpacketUpdateStats(g, n, handle, statsTicker, statsDone, &wg)

		logging.GetLogger().Infof("AfPacket Capture started on %s with First layer: %s", ifName, firstLayerType)
	}

	// leave the namespace, stay lock in the current thread
	nscontext.Quit()

	// manage BPF outside namespace because of syscall
	if capture.BPFFilter != "" {
		switch capture.Type {
		case "pcap":
			h := p.handle.(*pcap.Handle)
			err = h.SetBPFFilter(capture.BPFFilter)
		default:
			h := p.handle.(*AFPacketHandle)
			var rawBPF []bpf.RawInstruction
			if rawBPF, err = flow.BPFFilterToRaw(linkType, uint32(headerSize), capture.BPFFilter); err == nil {
				err = h.tpacket.SetBPF(rawBPF)
			}
		}

		if err != nil {
			logging.GetLogger().Errorf("BPF Filter failed: %s", err)
			return
		}
	}

	packetSeqChan, _ := p.flowTable.Start()
	defer p.flowTable.Stop()

	// notify active
	e.OnStarted()

	p.feedFlowTable(packetSeqChan, bpfFilter)

	if statsTicker != nil {
		close(statsDone)
		wg.Wait()
		statsTicker.Stop()
	}
	p.handle.Close()
	atomic.StoreInt64(&p.state, common.StoppedState)
}

func (p *GoPacketProbe) stop() {
	atomic.StoreInt64(&p.state, common.StoppingState)
}

func getGoPacketFirstLayerType(n *graph.Node) (gopacket.LayerType, layers.LinkType) {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return layers.LayerTypeEthernet, layers.LinkTypeEthernet
	}

	if encapType, err := n.GetFieldString("EncapType"); err == nil {
		switch encapType {
		case "ether":
			return layers.LayerTypeEthernet, layers.LinkTypeEthernet
		case "gre":
			return flow.LayerTypeInGRE, layers.LinkTypeIPv4
		case "sit", "ipip":
			return layers.LayerTypeIPv4, layers.LinkTypeIPv4
		case "tunnel6", "gre6":
			return layers.LayerTypeIPv6, layers.LinkTypeIPv6
		default:
			logging.GetLogger().Warningf("Encapsulation unknown %s on link %s, defaulting to Ethernet", encapType, name)
		}
	} else {
		logging.GetLogger().Warningf("EncapType not found on link %s, defaulting to Ethernet", name)
	}
	return layers.LayerTypeEthernet, layers.LinkTypeEthernet
}

// RegisterProbe registers a gopacket probe
func (p *GoPacketProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
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
		logging.GetLogger().Infof("MPLSUDP port: %v", port)
	}

	opts := flow.TableOpts{
		RawPacketLimit: int64(capture.RawPacketLimit),
		TCPMetric:      capture.ExtraTCPMetric,
		SocketInfo:     capture.SocketInfo,
	}
	ft := p.fpta.Alloc(tid, opts)

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

		probe.run(p.graph, n, capture, e)

		e.OnStopped()
	}()

	return nil
}

func (p *GoPacketProbesHandler) unregisterProbe(id string) error {
	if probe, ok := p.probes[id]; ok {
		logging.GetLogger().Debugf("Terminating gopacket capture on %s", id)
		probe.stop()
		p.fpta.Release(probe.flowTable)
		delete(p.probes, id)
	}

	return nil
}

// UnregisterProbe unregisters gopacket probe
func (p *GoPacketProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	err := p.unregisterProbe(string(n.ID))
	if err != nil {
		return err
	}

	return nil
}

// Start probe
func (p *GoPacketProbesHandler) Start() {
}

// Stop probe
func (p *GoPacketProbesHandler) Stop() {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	for id := range p.probes {
		p.unregisterProbe(id)
	}
	p.wg.Wait()
}

// NewGoPacketProbesHandler creates a new gopacket probe in the graph
func NewGoPacketProbesHandler(g *graph.Graph, fpta *FlowProbeTableAllocator) (*GoPacketProbesHandler, error) {
	return &GoPacketProbesHandler{
		graph:  g,
		fpta:   fpta,
		probes: make(map[string]*GoPacketProbe),
	}, nil
}
