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

	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	// AFPacket probe type
	AFPacket = "afpacket"
	// PCAP probe type
	PCAP = "pcap"
)

// PacketProbe describes a probe responsible for capturing packets
type PacketProbe interface {
	Stats() (graph.Metadata, error)
	SetBPFFilter(bpf string) error
	PacketSource() *gopacket.PacketSource
	Close()
}

// GoPacketProbe describes a new probe that store packets from gopacket pcap library in a flowtable
type GoPacketProbe struct {
	graph       *graph.Graph
	n           *graph.Node
	packetProbe PacketProbe
	state       int64
	ifName      string
	bpf         string
	nsPath      string
	captureType string
	layerType   gopacket.LayerType
	linkType    layers.LinkType
	headerSize  uint32
}

type ftProbe struct {
	flowTable *flow.Table
	probe     *GoPacketProbe
}

// GoPacketProbesHandler describes a flow probe handle in the graph
type GoPacketProbesHandler struct {
	graph      *graph.Graph
	fpta       *FlowProbeTableAllocator
	wg         sync.WaitGroup
	probes     map[string]*ftProbe
	probesLock common.RWMutex
}

func (p *GoPacketProbe) updateStats(g *graph.Graph, n *graph.Node, ticker *time.Ticker, done chan bool, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ticker.C:
			if stats, err := p.packetProbe.Stats(); err != nil {
				logging.GetLogger().Error(err)
			} else if atomic.LoadInt64(&p.state) == common.RunningState {
				g.Lock()
				t := g.StartMetadataTransaction(n)
				for k, v := range stats {
					t.AddMetadata("Capture."+k, v)
				}
				t.Commit()
				g.Unlock()
			}
		case <-done:
			return
		}
	}
}

func (p *GoPacketProbe) listen(packetCallback func(gopacket.Packet)) {
	packetSource := p.packetProbe.PacketSource()

	for atomic.LoadInt64(&p.state) == common.RunningState {
		packet, err := packetSource.NextPacket()
		switch err {
		case nil:
			if packetCallback != nil {
				packetCallback(packet)
			}
		case io.EOF:
			time.Sleep(20 * time.Millisecond)
		case afpacket.ErrTimeout:
			// nothing to do, poll wait for new packet or timeout
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// Run starts capturing packet, calling the passed callback for every packet
// and notifying the flow probe handler when the capture has started
func (p *GoPacketProbe) Run(packetCallback func(gopacket.Packet), e FlowProbeEventHandler) error {
	atomic.StoreInt64(&p.state, common.RunningState)

	var nsContext *common.NetNSContext
	var err error
	if p.nsPath != "" {
		logging.GetLogger().Debugf("Switching to namespace (path: %s)", p.nsPath)
		if nsContext, err = common.NewNetNsContext(p.nsPath); err != nil {
			return err
		}
	}
	defer nsContext.Close()

	switch p.captureType {
	case PCAP:
		p.packetProbe, err = NewPcapPacketProbe(p.ifName, int(p.headerSize))
		if err != nil {
			return err
		}
		logging.GetLogger().Infof("PCAP Capture started on %s with First layer: %s", p.ifName, p.layerType)
	default:
		if err = common.Retry(func() error {
			p.packetProbe, err = NewAfpacketPacketProbe(p.ifName, int(p.headerSize), p.layerType, p.linkType)
			return err
		}, 2, 100*time.Millisecond); err != nil {
			return err
		}
		logging.GetLogger().Infof("AfPacket Capture started on %s with First layer: %s", p.ifName, p.layerType)
	}

	// leave the namespace, stay lock in the current thread
	nsContext.Quit()

	var wg sync.WaitGroup
	statsDone := make(chan bool)

	// Go routine to update the interface statistics
	statsUpdate := config.GetInt("agent.capture.stats_update")
	statsTicker := time.NewTicker(time.Duration(statsUpdate) * time.Second)

	wg.Add(1)

	// manage BPF outside namespace because of syscall
	if p.bpf != "" {
		if err := p.packetProbe.SetBPFFilter(p.bpf); err != nil {
			return fmt.Errorf("Failed to set BPF filter: %s", err)
		}
	}

	// notify active
	if e != nil {
		go p.updateStats(p.graph, p.n, statsTicker, statsDone, &wg)

		e.OnStarted()
	}

	p.listen(packetCallback)

	if statsTicker != nil {
		close(statsDone)
		wg.Wait()
		statsTicker.Stop()
	}

	p.packetProbe.Close()
	atomic.StoreInt64(&p.state, common.StoppedState)

	return nil
}

// Stop capturing packets
func (p *GoPacketProbe) Stop() {
	atomic.StoreInt64(&p.state, common.StoppingState)
}

// NewGoPacketProbe returns a new Gopacket flow probe. It can use either `pcap` or `afpacket`
func NewGoPacketProbe(g *graph.Graph, n *graph.Node, captureType string, bpf string, headerSize uint32) (*GoPacketProbe, error) {
	ifName, _ := n.GetFieldString("Name")
	if ifName == "" {
		return nil, fmt.Errorf("No name for node %v", n)
	}

	firstLayerType, linkType := GoPacketFirstLayerType(n)

	_, nsPath, err := topology.NamespaceFromNode(g, n)
	if err != nil {
		return nil, err
	}

	return &GoPacketProbe{
		graph:       g,
		n:           n,
		ifName:      ifName,
		bpf:         bpf,
		linkType:    linkType,
		layerType:   firstLayerType,
		headerSize:  headerSize,
		state:       common.StoppedState,
		nsPath:      nsPath,
		captureType: captureType,
	}, nil
}

func (p *GoPacketProbesHandler) registerProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return fmt.Errorf("No name for node %v", n)
	}

	if capture.Type == "pcap" && !topology.IsInterfaceUp(n) {
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

	opts := tableOptsFromCapture(capture)
	flowTable := p.fpta.Alloc(tid, opts)

	headerSize := flow.DefaultCaptureLength
	if capture.HeaderSize != 0 {
		headerSize = uint32(capture.HeaderSize)
	}

	probe, err := NewGoPacketProbe(p.graph, n, capture.Type, capture.BPFFilter, headerSize)
	if err != nil {
		return err
	}

	// Apply temporarely the BPF in userspace to prevent non expected packet
	// between capture creation and the filter apply.
	var bpfFilter *flow.BPF
	if capture.BPFFilter != "" {
		bpfFilter, err = flow.NewBPF(probe.linkType, probe.headerSize, probe.bpf)
		if err != nil {
			return err
		}
	}

	p.probesLock.Lock()
	p.probes[id] = &ftProbe{probe: probe, flowTable: flowTable}
	p.probesLock.Unlock()
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		flowTable.Start()
		defer flowTable.Stop()

		count := 0
		err := probe.Run(func(packet gopacket.Packet) {
			flowTable.FeedWithGoPacket(packet, bpfFilter)
			// NOTE: bpf usperspace filter is applied to the few first packets in order to avoid
			// to get unexpected packets between capture start and bpf applying
			if count > 50 {
				bpfFilter = nil
			}
			count++
		}, e)
		if err != nil {
			logging.GetLogger().Error(err)

			e.OnError(err)
		} else {
			e.OnStopped()
		}
	}()

	return nil
}

// RegisterProbe registers a gopacket probe
func (p *GoPacketProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	err := p.registerProbe(n, capture, e)
	if err != nil {
		go e.OnError(err)
	}
	return err
}

func (p *GoPacketProbesHandler) unregisterProbe(id string) error {
	if probe, ok := p.probes[id]; ok {
		logging.GetLogger().Debugf("Terminating gopacket capture on %s", id)
		probe.probe.Stop()
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
	go e.OnStopped()

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
		probes: make(map[string]*ftProbe),
	}, nil
}

// GoPacketFirstLayerType returns the first layer of an interface
func GoPacketFirstLayerType(n *graph.Node) (gopacket.LayerType, layers.LinkType) {
	if encapType, err := n.GetFieldString("EncapType"); err == nil {
		return flow.GetFirstLayerType(encapType)
	}

	return layers.LayerTypeEthernet, layers.LinkTypeEthernet
}
