/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package flow

import (
	"errors"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/nu7hatch/gouuid"
)

type Flow struct {
	Uuid            string
	InputInterface  uint32
	OutputInterface uint32
	FrameLength     uint32
	EtherSrc        string
	EtherDst        string
	EtherType       string
	Ipv4Src         string
	Ipv4Dst         string
	Protocol        string
	PortSrc         uint32
	PortDst         uint32
	Seq             uint64
}

func (flow *Flow) fillFromGoPacket(packet gopacket.Packet) error {
	ethernetLayer := packet.Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return errors.New("Unable to decode the ethernet layer")
	}
	flow.EtherSrc = ethernetPacket.SrcMAC.String()
	flow.EtherDst = ethernetPacket.DstMAC.String()
	flow.EtherType = ethernetPacket.EthernetType.String()

	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv4)
		flow.Ipv4Src = ip.SrcIP.String()
		flow.Ipv4Dst = ip.DstIP.String()
		flow.Protocol = ip.Protocol.String()
	}

	udpLayer := packet.Layer(layers.LayerTypeUDP)
	if udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		flow.PortSrc = uint32(udp.SrcPort)
		flow.PortDst = uint32(udp.DstPort)
	}

	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		flow.PortSrc = uint32(tcp.SrcPort)
		flow.PortDst = uint32(tcp.DstPort)
		flow.Seq = uint64(tcp.Seq)
	}

	return nil
}

func New(in uint32, out uint32, len uint32) Flow {
	u, _ := uuid.NewV4()
	flow := Flow{Uuid: u.String(), InputInterface: in, OutputInterface: out}

	return flow
}

func FLowsFromSFlowSample(sample layers.SFlowFlowSample) []Flow {
	flows := []Flow{}

	for _, rec := range sample.Records {

		/* FIX(safchain): just keeping the raw packet for now */
		record, ok := rec.(layers.SFlowRawPacketFlowRecord)
		if !ok {
			continue
		}

		flow := New(sample.InputInterface, sample.OutputInterface, record.FrameLength)
		flow.fillFromGoPacket(record.Header)

		flows = append(flows, flow)
	}

	return flows
}
