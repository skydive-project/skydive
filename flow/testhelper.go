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
	"math/rand"
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type ProtocolType int

const (
	ETH ProtocolType = 1 + iota
	IPv4
	IPv6
	TCP
	UDP
)

/* protos must contain a UDP or TCP layer on top of IPv4 */
func forgeTestPacket(t *testing.T, seed int64, protos ...ProtocolType) *gopacket.Packet {
	rnd := rand.New(rand.NewSource(seed))

	rawBytes := []byte{10, 20, 30}
	var protoStack []gopacket.SerializableLayer

	for i, proto := range protos {
		switch proto {
		case ETH:
			ethernetLayer := &layers.Ethernet{
				SrcMAC:       net.HardwareAddr{0x00, 0x0F, 0xAA, 0xFA, 0xAA, byte(rnd.Intn(0x100))},
				DstMAC:       net.HardwareAddr{0x00, 0x0D, 0xBD, 0xBD, byte(rnd.Intn(0x100)), 0xBD},
				EthernetType: layers.EthernetTypeIPv4,
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
			}
			protoStack = append(protoStack, ipv4Layer)
		case TCP:
			tcpLayer := &layers.TCP{
				SrcPort: layers.TCPPort(byte(rnd.Intn(0x10000))),
				DstPort: layers.TCPPort(byte(rnd.Intn(0x10000))),
			}
			protoStack = append(protoStack, tcpLayer)
		case UDP:
			udpLayer := &layers.UDP{
				SrcPort: layers.UDPPort(byte(rnd.Intn(0x10000))),
				DstPort: layers.UDPPort(byte(rnd.Intn(0x10000))),
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

func GenerateTestFlows(t *testing.T, ft *FlowTable, baseSeed int64, probePath string) []*Flow {
	flows := []*Flow{}
	for i := int64(0); i < 10; i++ {
		var packet *gopacket.Packet
		if i < 5 {
			packet = forgeTestPacket(t, i+baseSeed*10, ETH, IPv4, TCP)
		} else {
			packet = forgeTestPacket(t, i+baseSeed*10, ETH, IPv4, UDP)
		}
		flow := FlowFromGoPacket(ft, packet, probePath)
		if flow == nil {
			t.Fail()
		}
		flows = append(flows, flow)
	}
	return flows
}

func NewTestFlowTableSimple(t *testing.T) *FlowTable {
	ft := NewFlowTable()
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

func NewFlowTableComplex(t *testing.T) *FlowTable {
	ft := NewFlowTable()
	GenerateTestFlows(t, ft, 0, "probe")
	return ft
}
