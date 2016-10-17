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

// +build test

package flow

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/op/go-logging"
)

type ProtocolType int

const (
	ETH ProtocolType = 1 + iota
	IPv4
	IPv6
	TCP
	UDP
	GRE
)

/* protos must contain a UDP or TCP layer on top of IPv4 */
func forgeTestPacket(t *testing.T, seed int64, swap bool, protos ...ProtocolType) *gopacket.Packet {
	rnd := rand.New(rand.NewSource(seed))

	rawBytes := []byte{10, 20, 30}
	var protoStack []gopacket.SerializableLayer

	for i, proto := range protos {
		switch proto {
		case GRE:
			var ethernetType layers.EthernetType
			switch protos[i+1]{
			case IPv4: ethernetType = layers.EthernetTypeIPv4
			default:
				t.Error(fmt.Sprintf("Protocol %s can not be encapsulated in GRE", protos[i+1]))
			}
			greLayer := &layers.GRE{
				Protocol: ethernetType,
			}
			protoStack = append(protoStack, greLayer)
		case ETH:
			ethernetLayer := &layers.Ethernet{
				SrcMAC:       net.HardwareAddr{0x00, 0x0F, 0xAA, 0xFA, 0xAA, byte(rnd.Intn(0x100))},
				DstMAC:       net.HardwareAddr{0x00, 0x0D, 0xBD, 0xBD, byte(rnd.Intn(0x100)), 0xBD},
				EthernetType: layers.EthernetTypeIPv4,
			}
			if swap {
				ethernetLayer.SrcMAC, ethernetLayer.DstMAC = ethernetLayer.DstMAC, ethernetLayer.SrcMAC
			}
			protoStack = append(protoStack, ethernetLayer)
		case IPv4:
			ipv4Layer := &layers.IPv4{
				SrcIP: net.IP{127, 0, 0, byte(rnd.Intn(0x100))},
				DstIP: net.IP{byte(rnd.Intn(0x100)), 8, 8, 8},
			}
			switch protos[i+1] {
			case TCP:
				ipv4Layer.Protocol = layers.IPProtocolTCP
			case UDP:
				ipv4Layer.Protocol = layers.IPProtocolUDP
			case GRE:
				ipv4Layer.Protocol = layers.IPProtocolGRE
			}
			if swap {
				ipv4Layer.SrcIP, ipv4Layer.DstIP = ipv4Layer.DstIP, ipv4Layer.SrcIP
			}
			protoStack = append(protoStack, ipv4Layer)
		case TCP:
			tcpLayer := &layers.TCP{
				SrcPort: layers.TCPPort(byte(rnd.Intn(0x10000))),
				DstPort: layers.TCPPort(byte(rnd.Intn(0x10000))),
			}
			if swap {
				tcpLayer.SrcPort, tcpLayer.DstPort = tcpLayer.DstPort, tcpLayer.SrcPort
			}
			protoStack = append(protoStack, tcpLayer)
		case UDP:
			udpLayer := &layers.UDP{
				SrcPort: layers.UDPPort(byte(rnd.Intn(0x10000))),
				DstPort: layers.UDPPort(byte(rnd.Intn(0x10000))),
			}
			if swap {
				udpLayer.SrcPort, udpLayer.DstPort = udpLayer.DstPort, udpLayer.SrcPort
			}
			protoStack = append(protoStack, udpLayer)
		default:
			t.Log("forgeTestPacket : Unsupported protocol ", proto)
		}
	}
	protoStack = append(protoStack, gopacket.Payload(rawBytes))

	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{FixLengths: true}
	err := gopacket.SerializeLayers(buffer, options, protoStack...)

	if err != nil {
		t.Fail()
	}

	gpacket := gopacket.NewPacket(buffer.Bytes(), layers.LayerTypeEthernet, gopacket.Default)
	return &gpacket
}

type probeNodeSetter struct {
	uuid string
}

func (p *probeNodeSetter) SetProbeNode(f *Flow) bool {
	f.NodeUUID = p.uuid
	return true
}

func generateTestFlows(t *testing.T, ft *Table, baseSeed int64, swap bool, uuid string) []*Flow {
	flows := []*Flow{}
	for i := int64(0); i < 10; i++ {
		var packet *gopacket.Packet
		if i < 5 {
			packet = forgeTestPacket(t, i+baseSeed*10, swap, ETH, IPv4, TCP)
		} else {
			packet = forgeTestPacket(t, i+baseSeed*10, swap, ETH, IPv4, UDP)
		}
		flow := FlowFromGoPacket(ft, packet, 0, &probeNodeSetter{uuid})
		if flow == nil {
			t.Fail()
		}
		flows = append(flows, flow)
	}
	return flows
}

func GenerateTestFlows(t *testing.T, ft *Table, baseSeed int64, uuid string) []*Flow {
	return generateTestFlows(t, ft, baseSeed, false, uuid)
}
func GenerateTestFlowsSymmetric(t *testing.T, ft *Table, baseSeed int64, uuid string) []*Flow {
	return generateTestFlows(t, ft, baseSeed, true, uuid)
}

func randomizeLayerStats(t *testing.T, seed int64, now int64, f *Flow) {
	rnd := rand.New(rand.NewSource(seed))
	f.Metric.ABPackets = int64(rnd.Int63n(0x10000))
	f.Metric.ABBytes = f.Metric.ABPackets * int64(14+rnd.Intn(1501))
	f.Metric.BAPackets = int64(rnd.Int63n(0x10000))
	f.Metric.BABytes = f.Metric.BAPackets * int64(14+rnd.Intn(1501))

	f.Metric.Start = now - rnd.Int63n(100)
	f.Metric.Last = f.Metric.Start
	if (rnd.Int() % 2) == 0 {
		f.Metric.Last = f.Metric.Start + rnd.Int63n(100)
	}
	return
}

func NewTestFlowTableSimple(t *testing.T) *Table {
	ft := NewTable(nil, nil)
	var flows []*Flow
	f := &Flow{}
	f.UUID = "1234"
	flows = append(flows, f)
	ft.Update(flows)
	ft.Update(flows)
	if "1 flows" != ft.String() {
		t.Error("We should got only 1 flow")
	}
	f = &Flow{}
	f.UUID = "4567"
	flows = append(flows, f)
	ft.Update(flows)
	ft.Update(flows)
	if "2 flows" != ft.String() {
		t.Error("We should got only 2 flows")
	}
	return ft
}

func NewTestFlowTableComplex(t *testing.T, updateHandler *FlowHandler, expireHandler *FlowHandler) *Table {
	ft := NewTable(updateHandler, expireHandler)
	GenerateTestFlows(t, ft, 0xca55e77e, "probe-uuid")
	return ft
}

func graphFlows(now int64, flows []*Flow, tagsUUID ...string) string {
	minStart := int64(^uint64(0) >> 1)
	maxEnd := int64(0)
	for _, f := range flows {
		fstart := f.Metric.Start
		fend := f.Metric.Last
		if fstart < minStart {
			minStart = fstart
		}
		if fend > maxEnd {
			maxEnd = fend
		}
	}
	nbCol := 75
	scale := float64(maxEnd-minStart) / float64(nbCol)
	fmt.Printf("%d ----- %d (%d) scale %.2f\n", minStart-now, maxEnd-now, maxEnd-minStart, scale)
	for _, f := range flows {
		s := f.Metric
		fstart := s.Start
		fend := s.Last
		duration := fend - fstart
		hstr := hex.EncodeToString(f.Link.Hash())

		needTag := false
		for _, tag := range tagsUUID {
			if needTag = strings.Contains(hstr, tag); needTag {
				break
			}
		}
		if needTag {
			fmt.Print(logging.ColorSeq(logging.ColorRed))
		}
		for x := 0; x < nbCol; x++ {
			if (x < int(float64(fstart-minStart)/scale)) || (x > int(float64(fend-minStart)/scale)) {
				fmt.Print("-")
			} else {
				fmt.Print("x")
			}
		}
		e := f.Metric
		fdt := float64(duration)
		ppsAB := float64(e.ABPackets) / fdt
		bpsAB := float64(e.ABBytes) / fdt
		fmt.Printf(" %s %d (%d) %.0f %.0f\n", hstr[:6], s.Start-now, duration, ppsAB, bpsAB)
		if needTag {
			fmt.Print(logging.ColorSeq(0))
		}
	}
	return ""
}
