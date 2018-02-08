// +build dpdk

/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"strconv"
	"sync/atomic"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	dpdkcommon "github.com/intel-go/yanff/common"
	dpdkflow "github.com/intel-go/yanff/flow"
	"github.com/intel-go/yanff/packet"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

var (
	dpdkNBWorkers uint
)

// DPDKProbesHandler describes a flow probe handle in the graph
type DPDKProbesHandler struct {
	graph *graph.Graph
	fpta  *FlowProbeTableAllocator
}

// RegisterProbe registers a gopacket probe
func (p *DPDKProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}
	enablePort(tid, true)
	e.OnStarted()
	return nil
}

// UnregisterProbe unregisters gopacket probe
func (p *DPDKProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}
	enablePort(tid, false)
	e.OnStopped()
	return nil
}

// Start probe
func (p *DPDKProbesHandler) Start() {
	go dpdkflow.SystemStart()
}

// Stop probe
func (p *DPDKProbesHandler) Stop() {
}

func packetHandler(packets []*packet.Packet, next []bool, nbPackets uint, context dpdkflow.UserContext) {
	ctx, _ := context.(ctxQueue)
	if ctx.enabled.Load() == false {
		for i := uint(0); i < nbPackets; i++ {
			next[i] = false
		}
		return
	}

	for i := uint(0); i < nbPackets; i++ {
		packet := gopacket.NewPacket(packets[i].GetRawPacketBytes(), layers.LayerTypeEthernet, gopacket.Default)
		if ps := flow.PacketSeqFromGoPacket(&packet, 0, nil); len(ps.Packets) > 0 {
			ctx.packetSeqChan <- ps
		}
		next[i] = false
	}
}

func l3Splitter(currentPacket *packet.Packet, context dpdkflow.UserContext) uint {
	ipv4, ipv6 := currentPacket.ParseAllKnownL3()
	if ipv4 != nil {
		h := (ipv4.SrcAddr>>24)&0xff ^ (ipv4.DstAddr>>24)&0xff ^
			(ipv4.SrcAddr>>16)&0xff ^ (ipv4.DstAddr>>16)&0xff ^
			(ipv4.SrcAddr>>8)&0xff ^ (ipv4.DstAddr>>8)&0xff ^
			(ipv4.SrcAddr)&0xff ^ (ipv4.DstAddr)&0xff
		return uint(h) % dpdkNBWorkers
	}
	if ipv6 != nil {
		h := uint(0)
		for i := range ipv6.SrcAddr {
			h = h ^ uint(ipv6.SrcAddr[i]^ipv6.DstAddr[i])
		}
		return h % dpdkNBWorkers
	}
	return 0
}

func enablePort(tid string, enable bool) {
	port, ok := dpdkPorts[tid]
	if !ok {
		return
	}
	for _, q := range port.queues {
		q.enabled.Store(enable)
	}
}

var dpdkPorts = make(map[string]dpdkPort)

type dpdkPort struct {
	queues []*ctxQueue
}

type ctxQueue struct {
	enabled       *atomic.Value
	packetSeqChan chan *flow.PacketSequence
}

func (c ctxQueue) Copy() interface{} {
	return c
}

func getDPDKMacAddress(port int) string {
	mac := dpdkflow.GetPortMACAddress(uint8(port))
	macAddr := ""
	for i, m := range mac {
		macAddr += fmt.Sprintf("%02x", m)
		if i < (len(mac) - 1) {
			macAddr += ":"
		}
	}
	return macAddr
}

// NewDPDKProbesHandler creates a new gopacket probe in the graph
func NewDPDKProbesHandler(g *graph.Graph, fpta *FlowProbeTableAllocator) (*DPDKProbesHandler, error) {
	ports := config.GetStringSlice("dpdk.ports")
	nbWorkers := config.GetInt("dpdk.workers")

	nbPorts := len(ports)
	if nbWorkers == 0 || nbPorts == 0 {
		return nil, fmt.Errorf("DPDK flow porbe is not configured")
	}
	dpdkNBWorkers = uint(nbWorkers)

	cfg := &dpdkflow.Config{
		LogType: dpdkcommon.Initialization,
	}
	debug := config.GetInt("dpdk.debug")
	if debug > 0 {
		cfg.LogType = dpdkcommon.Debug
		cfg.DebugTime = uint(debug * 1000)
	}
	dpdkflow.SystemInit(cfg)

	opts := flow.TableOpts{
		RawPacketLimit: 0,
		TCPMetric:      false,
		SocketInfo:     false,
	}

	dph := &DPDKProbesHandler{
		graph: g,
		fpta:  fpta,
	}

	hostNode := g.LookupFirstNode(graph.Metadata{
		"Name": g.GetHost(),
		"Type": "host",
	})

	for _, p := range ports {
		inport, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("misconfiguration of DPDK port %v", p)
		}
		portMAC := getDPDKMacAddress(inport)
		m := graph.Metadata{
			"Name":      fmt.Sprintf("DPDK-port-%d", inport),
			"EncapType": "ether",
			"IfIndex":   inport,
			"MAC":       portMAC,
			"Driver":    "dpdk",
			"State":     "UP",
			"Type":      "dpdkport",
		}
		dpdkNode := g.NewNode(graph.GenID(), m)
		topology.AddOwnershipLink(g, hostNode, dpdkNode, nil)
		tid, _ := dpdkNode.GetFieldString("TID")
		port := dpdkPort{}

		inputFlow := dpdkflow.SetReceiver(uint8(inport))
		outputFlows := dpdkflow.SetSplitter(inputFlow, l3Splitter, uint(dpdkNBWorkers), nil)

		for i := 0; i < nbWorkers; i++ {
			ft := fpta.Alloc(tid, opts)

			ps, _ := ft.Start()
			ctx := ctxQueue{
				packetSeqChan: ps,
				enabled:       &atomic.Value{},
			}
			ctx.enabled.Store(false)
			port.queues = append(port.queues, &ctx)

			dpdkflow.SetHandler(outputFlows[i], packetHandler, ctx)
		}
		dpdkPorts[tid] = port

		for i := 0; i < nbWorkers; i++ {
			dpdkflow.SetStopper(outputFlows[i])
		}

	}
	return dph, nil
}
