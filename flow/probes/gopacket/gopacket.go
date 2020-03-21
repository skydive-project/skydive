// +build linux

/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package gopacket

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/flow/probes/targets"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/netns"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
)

const (
	// AFPacket probe type
	AFPacket = "afpacket"
	// PCAP probe type
	PCAP = "pcap"
)

// PacketProbe describes a probe responsible for capturing packets
type PacketProbe interface {
	Stats() (*probes.CaptureStats, error)
	SetBPFFilter(bpf string) error
	PacketSource() *gopacket.PacketSource
	Close()
}

// Probe describes a new probe that store packets from gopacket pcap library in a flowtable
type Probe struct {
	Ctx         probes.Context
	node        *graph.Node
	packetProbe PacketProbe
	state       service.State
	ifName      string
	bpfFilter   string
	nsPath      string
	captureType string
	layerType   gopacket.LayerType
	linkType    layers.LinkType
	headerSize  uint32
}

type ftProbe struct {
	target targets.Target
	probe  *Probe
}

// ProbesHandler describes a flow probe handle in the graph
type ProbesHandler struct {
	Ctx probes.Context
	wg  sync.WaitGroup
}

func (p *Probe) updateStats(statsCallback func(flow.Stats), captureStats *probes.CaptureStats, ticker *time.Ticker, done chan bool, wg *sync.WaitGroup) {
	defer wg.Done()

	g := p.Ctx.Graph
	n := p.node

	for {
		select {
		case <-ticker.C:
			if stats, err := p.packetProbe.Stats(); err != nil {
				p.Ctx.Logger.Error(err)
			} else if p.state.Load() == service.RunningState {
				g.Lock()
				g.UpdateMetadata(n, "Captures", func(obj interface{}) bool {
					captureStats.PacketsDropped = stats.PacketsDropped
					captureStats.PacketsReceived = stats.PacketsReceived
					captureStats.PacketsIfDropped = stats.PacketsIfDropped
					return true
				})
				g.Unlock()

				statsCallback(flow.Stats{
					PacketsDropped:  stats.PacketsDropped,
					PacketsReceived: stats.PacketsReceived,
				})
			}
		case <-done:
			return
		}
	}
}

func (p *Probe) listen(packetCallback func(gopacket.Packet)) error {
	packetSource := p.packetProbe.PacketSource()

	var errs int
	for p.state.Load() == service.RunningState {
		packet, err := packetSource.NextPacket()
		switch err {
		case nil:
			errs = 0

			if packetCallback != nil {
				packetCallback(packet)
			}
		case io.EOF:
			time.Sleep(20 * time.Millisecond)
		case afpacket.ErrTimeout:
			// nothing to do, poll wait for new packet or timeout
		case afpacket.ErrPoll:
			errs++

			if errs > 20 {
				return afpacket.ErrPoll
			}
			time.Sleep(20 * time.Millisecond)
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}

	return nil
}

// Run starts capturing packet, calling the passed callback for every packet
// and notifying the flow probe handler when the capture has started
func (p *Probe) Run(packetCallback func(gopacket.Packet), statsCallback func(flow.Stats), e probes.ProbeEventHandler) error {
	p.state.Store(service.RunningState)

	var nsContext *netns.Context
	var err error
	if p.nsPath != "" {
		p.Ctx.Logger.Debugf("Switching to namespace (path: %s)", p.nsPath)
		if nsContext, err = netns.NewContext(p.nsPath); err != nil {
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
		p.Ctx.Logger.Infof("PCAP Capture started on %s with First layer: %s", p.ifName, p.layerType)
	default:
		if err = retry.Do(func() error {
			p.packetProbe, err = NewAfpacketPacketProbe(p.ifName, int(p.headerSize), p.layerType, p.linkType)
			return err
		}, retry.Attempts(4), retry.Delay(10*time.Millisecond)); err != nil {
			return err
		}
		p.Ctx.Logger.Infof("AfPacket Capture started on %s with First layer: %s", p.ifName, p.layerType)
	}

	// leave the namespace, stay lock in the current thread
	nsContext.Quit()

	var wg sync.WaitGroup
	statsDone := make(chan bool)

	// Go routine to update the interface statistics
	statsUpdate := p.Ctx.Config.GetInt("agent.capture.stats_update")
	statsTicker := time.NewTicker(time.Duration(statsUpdate) * time.Second)

	// manage BPF outside namespace because of syscall
	if p.bpfFilter != "" {
		if err := p.packetProbe.SetBPFFilter(p.bpfFilter); err != nil {
			return fmt.Errorf("Failed to set BPF filter: %s", err)
		}
	}

	metadata := &probes.CaptureMetadata{}
	e.OnStarted(metadata)

	// notify active
	wg.Add(1)
	go p.updateStats(statsCallback, &metadata.CaptureStats, statsTicker, statsDone, &wg)

	err = p.listen(packetCallback)

	close(statsDone)
	wg.Wait()
	statsTicker.Stop()

	p.packetProbe.Close()
	p.state.Store(service.StoppedState)

	return err
}

// Stop capturing packets
func (p *Probe) Stop() {
	p.state.Store(service.StoppingState)
}

// NewCapture returns a new Gopacket flow probe. It can use either `pcap` or `afpacket`
func NewCapture(ctx probes.Context, n *graph.Node, captureType, bpfFilter string, headerSize uint32) (*Probe, error) {
	ifName, _ := n.GetFieldString("Name")
	if ifName == "" {
		return nil, fmt.Errorf("No name for node %v", n)
	}

	firstLayerType, linkType := FirstLayerType(n)

	_, nsPath, err := topology.NamespaceFromNode(ctx.Graph, n)
	if err != nil {
		return nil, err
	}

	return &Probe{
		Ctx:         ctx,
		node:        n,
		ifName:      ifName,
		bpfFilter:   bpfFilter,
		linkType:    linkType,
		layerType:   firstLayerType,
		headerSize:  headerSize,
		state:       service.StoppedState,
		nsPath:      nsPath,
		captureType: captureType,
	}, nil
}

// RegisterProbe registers a gopacket probe
func (p *ProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e probes.ProbeEventHandler) (probes.Probe, error) {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return nil, fmt.Errorf("No name for node %v", n)
	}

	if !topology.IsInterfaceUp(n) {
		return nil, fmt.Errorf("Can't start capture on node down %s", name)
	}

	encapType, _ := n.GetFieldString("EncapType")
	if encapType == "" {
		return nil, fmt.Errorf("No EncapType for node %v", n)
	}

	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return nil, fmt.Errorf("No TID for node %v", n)
	}

	if port, err := n.GetFieldInt64("MPLSUDPPort"); err == nil {
		// All gopacket instance of this agent will classify UDP packets coming
		// from UDP port MPLSUDPPort as MPLS whatever the source interface
		layers.RegisterUDPPortLayerType(layers.UDPPort(port), layers.LayerTypeMPLS)
		p.Ctx.Logger.Infof("MPLSUDP port: %v", port)
	}

	headerSize := flow.DefaultCaptureLength
	if capture.HeaderSize != 0 {
		headerSize = uint32(capture.HeaderSize)
	}

	// exclude my own traffic
	bpfFilter := probes.NormalizeBPFFilter(capture)
	p.Ctx.Logger.Debugf("Normalized capture BPF Filter used: %s", bpfFilter)

	probe, err := NewCapture(p.Ctx, n, capture.Type, bpfFilter, headerSize)
	if err != nil {
		return nil, err
	}

	// Apply temporarely the BPF in userspace to prevent non expected packet
	// between capture creation and the filter apply.
	var bpf *flow.BPF
	if bpfFilter != "" {
		bpf, err = flow.NewBPF(probe.linkType, probe.headerSize, bpfFilter)
		if err != nil {
			return nil, err
		}
	}

	uuids := flow.UUIDs{NodeTID: tid, CaptureID: capture.UUID}
	target, err := targets.NewTarget(capture.TargetType, p.Ctx.Graph, n, capture, uuids, bpf, p.Ctx.FTA)
	if err != nil {
		return nil, err
	}

	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		target.Start()
		defer target.Stop()

		count := 0
		err := probe.Run(
			func(packet gopacket.Packet) {
				target.SendPacket(packet, bpf)
				// NOTE: bpf userspace filter is applied to the few first packets in order to avoid
				// to get unexpected packets between capture start and bpf applying
				if count > 50 {
					bpf = nil
				}
				count++
			},
			target.SendStats,
			e)

		if err != nil {
			p.Ctx.Logger.Error(err)

			e.OnError(err)
		} else {
			e.OnStopped()
		}
	}()

	return &ftProbe{probe: probe, target: target}, nil
}

func (p *ProbesHandler) unregisterProbe(probe *ftProbe) error {
	p.Ctx.Logger.Debugf("Terminating gopacket capture on %s", probe.probe.node.ID)
	probe.probe.Stop()
	return nil
}

// UnregisterProbe unregisters gopacket probe
func (p *ProbesHandler) UnregisterProbe(n *graph.Node, e probes.ProbeEventHandler, probe probes.Probe) error {
	err := p.unregisterProbe(probe.(*ftProbe))
	if err != nil {
		return err
	}
	go e.OnStopped()

	return nil
}

// Start probe
func (p *ProbesHandler) Start() error {
	return nil
}

// Stop probe
func (p *ProbesHandler) Stop() {
	p.wg.Wait()
}

// CaptureTypes supported
func (p *ProbesHandler) CaptureTypes() []string {
	return []string{"afpacket", "pcap"}
}

// NewProbe returns a new GoPacket probe
func NewProbe(ctx probes.Context, bundle *probe.Bundle) (probes.FlowProbeHandler, error) {
	return &ProbesHandler{
		Ctx: ctx,
	}, nil
}

// FirstLayerType returns the first layer of an interface
func FirstLayerType(n *graph.Node) (gopacket.LayerType, layers.LinkType) {
	if encapType, err := n.GetFieldString("EncapType"); err == nil {
		return flow.GetFirstLayerType(encapType)
	}

	return layers.LayerTypeEthernet, layers.LinkTypeEthernet
}
