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

package pcapsocket

import (
	"fmt"
	"net"
	"sync"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/portallocator"
	"github.com/skydive-project/skydive/probe"
)

// Probe describes a TCP packet listener that inject packets in a flowtable
type Probe struct {
	Ctx       probes.Context
	state     service.State
	flowTable *flow.Table
	listener  *net.TCPListener
	port      int
	bpfFilter string
}

// ProbeHandler describes a Pcap socket probe in the graph
type ProbeHandler struct {
	Ctx           probes.Context
	addr          *net.TCPAddr
	wg            sync.WaitGroup
	portAllocator *portallocator.PortAllocator
}

func (p *Probe) run() {
	p.state.Store(service.RunningState)

	packetSeqChan, _, _ := p.flowTable.Start(nil)
	defer p.flowTable.Stop()

	for p.state.Load() == service.RunningState {
		conn, err := p.listener.Accept()
		if err != nil {
			if p.state.Load() == service.RunningState {
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
func (p *ProbeHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e probes.ProbeEventHandler) (probes.Probe, error) {
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
	ft := p.Ctx.FTA.Alloc(uuids, probes.TableOptsFromCapture(capture))

	probe := &Probe{
		Ctx:       p.Ctx,
		state:     service.StoppedState,
		flowTable: ft,
		listener:  listener,
		port:      port,
		bpfFilter: capture.BPFFilter,
	}

	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		e.OnStarted(&probes.CaptureMetadata{PCAPSocket: tcpAddr.String()})

		probe.run()

		e.OnStopped()
	}()

	return probe, nil
}

// UnregisterProbe a probe
func (p *ProbeHandler) UnregisterProbe(n *graph.Node, e probes.ProbeEventHandler, fp probes.Probe) error {
	probe := fp.(*Probe)

	p.Ctx.FTA.Release(probe.flowTable)

	probe.state.Store(service.StoppingState)
	err := probe.listener.Close()
	if err != nil {
		return err
	}
	err = p.portAllocator.Release(probe.port)
	if err != nil {
		return err
	}

	return err
}

// Start the probe
func (p *ProbeHandler) Start() error {
	return nil
}

// Stop the probe
func (p *ProbeHandler) Stop() {
	p.wg.Wait()
}

// CaptureTypes supported
func (p *ProbeHandler) CaptureTypes() []string {
	return []string{"pcapsocket"}
}

// NewProbe returns a new pcapsocket probe
func NewProbe(ctx probes.Context, bundle *probe.Bundle) (probes.FlowProbeHandler, error) {
	listen := ctx.Config.GetString("agent.flow.pcapsocket.bind_address")
	minPort := ctx.Config.GetInt("agent.flow.pcapsocket.min_port")
	maxPort := ctx.Config.GetInt("agent.flow.pcapsocket.max_port")

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", listen, minPort))
	if err != nil {
		return nil, err
	}

	portAllocator, err := portallocator.New(minPort, maxPort)
	if err != nil {
		return nil, err
	}

	return &ProbeHandler{
		Ctx:           ctx,
		addr:          addr,
		portAllocator: portAllocator,
	}, nil
}
