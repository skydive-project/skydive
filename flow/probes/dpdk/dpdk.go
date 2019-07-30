// +build dpdk

/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package dpdk

import (
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	dpdkcommon "github.com/intel-go/nff-go/common"
	dpdkflow "github.com/intel-go/nff-go/flow"
	"github.com/intel-go/nff-go/packet"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
)

var (
	dpdkNBWorkers uint
)

// ProbesHandler describes a flow probe handle in the graph
type ProbesHandler struct {
	Ctx probes.Context
}

// RegisterProbe registers a gopacket probe
func (p *ProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e probes.ProbeEventHandler) (probes.Probe, error) {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return nil, fmt.Errorf("No TID for node %v", n)
	}
	enablePort(tid, true)
	e.OnStarted(&probes.CaptureMetadata{})
	return nil, nil
}

// UnregisterProbe unregisters gopacket probe
func (p *ProbesHandler) UnregisterProbe(n *graph.Node, e probes.ProbeEventHandler, fp probes.Probe) error {
	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return fmt.Errorf("No TID for node %v", n)
	}
	enablePort(tid, false)
	e.OnStopped()
	return nil
}

// Start probe
func (p *ProbesHandler) Start() error {
	go dpdkflow.SystemStart()
	return nil
}

// Stop probe
func (p *ProbesHandler) Stop() {
}

func packetHandler(packet *packet.Packet, context dpdkflow.UserContext) {
	gopacket := gopacket.NewPacket(packet.GetRawPacketBytes(), layers.LayerTypeEthernet, gopacket.Default)
	ctxQ, _ := context.(ctxQueue)
	ctxQ.ft.FeedWithGoPacket(gopacket, nil)
}

func l3Splitter(currentPacket *packet.Packet, context dpdkflow.UserContext) uint {
	ipv4, ipv6, _ := currentPacket.ParseAllKnownL3()
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
	for _, ctxQ := range port.queues {
		ctxQ.enabled.Store(enable)
	}
}

var dpdkPorts = make(map[string]dpdkPort)

type dpdkPort struct {
	queues []*ctxQueue
}

type ctxQueue struct {
	enabled *atomic.Value
	ft      *flow.Table
}

func (c ctxQueue) Copy() interface{} {
	return c
}

func (c ctxQueue) Delete() {
}

func getDPDKMacAddress(port int) string {
	mac := dpdkflow.GetPortMACAddress(uint16(port))
	macAddr := ""
	for i, m := range mac {
		macAddr += fmt.Sprintf("%02x", m)
		if i < (len(mac) - 1) {
			macAddr += ":"
		}
	}
	return macAddr
}

// CaptureTypes supported
func (p *ProbesHandler) CaptureTypes() []string {
	return []string{"dpdk"}
}

// NewProbe returns a new DPDK probe
func NewProbe(ctx probes.Context, bundle *probe.Bundle) (probes.FlowProbeHandler, error) {
	ports := ctx.Config.GetStringSlice("dpdk.ports")
	nbWorkers := ctx.Config.GetInt("dpdk.workers")

	nbPorts := len(ports)
	if nbWorkers == 0 || nbPorts == 0 {
		return nil, fmt.Errorf("DPDK flow porbe is not configured")
	}
	dpdkNBWorkers = uint(nbWorkers)

	cfg := &dpdkflow.Config{
		LogType: dpdkcommon.Initialization,
	}
	debug := ctx.Config.GetInt("dpdk.debug")
	if debug > 0 {
		cfg.LogType = dpdkcommon.Debug
		cfg.DebugTime = uint(debug * 1000)
	}
	dpdkflow.SystemInit(cfg)

	opts := flow.TableOpts{
		RawPacketLimit: 0,
	}

	p := &ProbesHandler{Ctx: ctx}

	hostNode := ctx.Graph.LookupFirstNode(graph.Metadata{
		"Name": ctx.Graph.GetHost(),
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
		dpdkNode, err := ctx.Graph.NewNode(graph.GenID(), m)
		if err != nil {
			return nil, err
		}
		topology.AddOwnershipLink(ctx.Graph, hostNode, dpdkNode, nil)

		tid, _ := dpdkNode.GetFieldString("TID")
		uuids := flow.UUIDs{NodeTID: tid}

		port := dpdkPort{}

		inputFlow, _ := dpdkflow.SetReceiver(uint16(inport))
		outputFlows, _ := dpdkflow.SetSplitter(inputFlow, l3Splitter, uint(dpdkNBWorkers), nil)

		for i := 0; i < nbWorkers; i++ {
			ft := ctx.FTA.Alloc(uuids, opts)

			ft.Start(nil)
			ctxQ := ctxQueue{
				ft:      ft,
				enabled: &atomic.Value{},
			}
			ctxQ.enabled.Store(false)
			port.queues = append(port.queues, &ctxQ)

			dpdkflow.SetHandler(outputFlows[i], packetHandler, ctxQ)
		}
		dpdkPorts[tid] = port

		for i := 0; i < nbWorkers; i++ {
			dpdkflow.SetStopper(outputFlows[i])
		}

	}

	return p, nil
}
