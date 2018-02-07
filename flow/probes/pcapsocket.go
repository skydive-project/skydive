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
	"net"
	"sync"
	"sync/atomic"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// PcapSocketProbe describes a TCP packet listener that inject packets in a flowtable
type PcapSocketProbe struct {
	node      *graph.Node
	state     int64
	flowTable *flow.Table
	listener  *net.TCPListener
	port      int
	bpfFilter string
}

// PcapSocketProbeHandler describes a Pcap socket probe in the graph
type PcapSocketProbeHandler struct {
	graph         *graph.Graph
	fpta          *FlowProbeTableAllocator
	addr          *net.TCPAddr
	wg            sync.WaitGroup
	probes        map[string]*PcapSocketProbe
	probesLock    sync.RWMutex
	portAllocator *common.PortAllocator
}

func (p *PcapSocketProbe) run() {
	atomic.StoreInt64(&p.state, common.RunningState)

	packetSeqChan, _ := p.flowTable.Start()
	defer p.flowTable.Stop()

	for atomic.LoadInt64(&p.state) == common.RunningState {
		conn, err := p.listener.Accept()
		if err != nil {
			if atomic.LoadInt64(&p.state) == common.RunningState {
				logging.GetLogger().Errorf("Error while accepting connection: %s", err.Error())
			}
			break
		}

		feeder, err := flow.NewPcapTableFeeder(conn, packetSeqChan, true, p.bpfFilter)
		if err != nil {
			logging.GetLogger().Errorf("Failed to create pcap table feeder: %s", err.Error())
			return
		}

		feeder.Start()
		defer feeder.Stop()
	}
}

// RegisterProbe registers a new probe in the graph
func (p *PcapSocketProbeHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {

	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}

	if _, ok := p.probes[tid]; ok {
		return fmt.Errorf("Already registered %s", tid)
	}

	port, err := p.portAllocator.Allocate()
	if err != nil {
		return err
	}

	var tcpAddr = *p.addr
	tcpAddr.Port = port

	listener, err := net.ListenTCP("tcp", &tcpAddr)
	if err != nil {
		logging.GetLogger().Errorf("Failed to listen on TDP socket %s: %s", tcpAddr.String(), err.Error())
		return err
	}

	opts := flow.TableOpts{
		RawPacketLimit: int64(capture.RawPacketLimit),
		TCPMetric:      capture.ExtraTCPMetric,
		SocketInfo:     capture.SocketInfo,
	}
	ft := p.fpta.Alloc(tid, opts)

	probe := &PcapSocketProbe{
		node:      n,
		state:     common.StoppedState,
		flowTable: ft,
		listener:  listener,
		port:      port,
		bpfFilter: capture.BPFFilter,
	}

	p.probesLock.Lock()
	p.probes[tid] = probe
	p.probesLock.Unlock()
	p.wg.Add(1)

	p.graph.AddMetadata(n, "Capture.PCAPSocket", tcpAddr.String())

	go func() {
		defer p.wg.Done()

		e.OnStarted()

		probe.run()

		e.OnStopped()
	}()

	return nil
}

// UnregisterProbe a probe
func (p *PcapSocketProbeHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}

	probe, ok := p.probes[tid]
	if !ok {
		return fmt.Errorf("No registered probe for %s", tid)
	}
	p.fpta.Release(probe.flowTable)
	delete(p.probes, tid)

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
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	for _, probe := range p.probes {
		p.UnregisterProbe(probe.node, nil)
	}
	p.wg.Wait()
}

// NewPcapSocketProbeHandler creates a new pcap socket probe
func NewPcapSocketProbeHandler(g *graph.Graph, fpta *FlowProbeTableAllocator) (*PcapSocketProbeHandler, error) {
	listen := config.GetString("agent.flow.pcapsocket.bind_address")
	minPort := config.GetInt("agent.flow.pcapsocket.min_port")
	maxPort := config.GetInt("agent.flow.pcapsocket.max_port")

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", listen, minPort))
	if err != nil {
		return nil, err
	}

	portAllocator, err := common.NewPortAllocator(minPort, maxPort)
	if err != nil {
		return nil, err
	}

	return &PcapSocketProbeHandler{
		graph:         g,
		fpta:          fpta,
		addr:          addr,
		probes:        make(map[string]*PcapSocketProbe),
		portAllocator: portAllocator,
	}, nil
}
