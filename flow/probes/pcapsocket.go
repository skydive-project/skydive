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

package probes

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
)

// PcapSocketProbe describes a TCP packet listener that inject packets in a flowtable
type PcapSocketProbe struct {
	Ctx       Context
	state     int64
	flowTable *flow.Table
	listener  *net.TCPListener
	port      int
	bpfFilter string
}

// PcapSocketProbeHandler describes a Pcap socket probe in the graph
type PcapSocketProbeHandler struct {
	Ctx           Context
	addr          *net.TCPAddr
	wg            sync.WaitGroup
	portAllocator *common.PortAllocator
}

func (p *PcapSocketProbe) run() {
	atomic.StoreInt64(&p.state, common.RunningState)

	packetSeqChan, _, _ := p.flowTable.Start()
	defer p.flowTable.Stop()

	for atomic.LoadInt64(&p.state) == common.RunningState {
		conn, err := p.listener.Accept()
		if err != nil {
			if atomic.LoadInt64(&p.state) == common.RunningState {
				p.Ctx.Logger.Errorf("Error while accepting connection: %s", err)
			}
			break
		}

		feeder, err := flow.NewPcapTableFeeder(conn, packetSeqChan, true, p.bpfFilter)
		if err != nil {
			p.Ctx.Logger.Errorf("Failed to create pcap table feeder: %s", err)
			return
		}

		feeder.Start()
		defer feeder.Stop()
	}
}

// RegisterProbe registers a new probe in the graph
func (p *PcapSocketProbeHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e ProbeEventHandler) (Probe, error) {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return nil, fmt.Errorf("No TID for node %v", n)
	}

	var listener *net.TCPListener
	var err error
	var tcpAddr = *p.addr
	fnc := func(p int) error {
		listener, err = net.ListenTCP("tcp", &tcpAddr)
		if err != nil {
			return err
		}
		tcpAddr.Port = p

		return nil
	}

	port, err := p.portAllocator.Allocate(fnc)
	if err != nil {
		p.Ctx.Logger.Errorf("Failed to listen on TCP socket %s: %s", tcpAddr.String(), err)
	}

	uuids := flow.UUIDs{NodeTID: tid, CaptureID: capture.UUID}
	ft := p.Ctx.FTA.Alloc(uuids, tableOptsFromCapture(capture))

	probe := &PcapSocketProbe{
		Ctx:       p.Ctx,
		state:     common.StoppedState,
		flowTable: ft,
		listener:  listener,
		port:      port,
		bpfFilter: capture.BPFFilter,
	}

	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		e.OnStarted(&CaptureMetadata{PCAPSocket: tcpAddr.String()})

		probe.run()

		e.OnStopped()
	}()

	return probe, nil
}

// UnregisterProbe a probe
func (p *PcapSocketProbeHandler) UnregisterProbe(n *graph.Node, e ProbeEventHandler, fp Probe) error {
	probe := fp.(*PcapSocketProbe)

	p.Ctx.FTA.Release(probe.flowTable)

	atomic.StoreInt64(&probe.state, common.StoppingState)
	probe.listener.Close()

	p.portAllocator.Release(probe.port)

	return nil
}

// Start the probe
func (p *PcapSocketProbeHandler) Start() {
}

// Stop the probe
func (p *PcapSocketProbeHandler) Stop() {
	p.wg.Wait()
}

// CaptureTypes supported
func (p *PcapSocketProbeHandler) CaptureTypes() []string {
	return []string{"pcapsocket"}
}

// Init initializes a new pcap socket probe
func (p *PcapSocketProbeHandler) Init(ctx Context, bundle *probe.Bundle) (FlowProbeHandler, error) {
	listen := ctx.Config.GetString("agent.flow.pcapsocket.bind_address")
	minPort := ctx.Config.GetInt("agent.flow.pcapsocket.min_port")
	maxPort := ctx.Config.GetInt("agent.flow.pcapsocket.max_port")

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", listen, minPort))
	if err != nil {
		return nil, err
	}

	portAllocator, err := common.NewPortAllocator(minPort, maxPort)
	if err != nil {
		return nil, err
	}

	p.Ctx = ctx
	p.addr = addr
	p.portAllocator = portAllocator

	return p, nil
}
