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
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/nu7hatch/gouuid"

	"github.com/redhat-cip/skydive/mappings"
)

type Flow struct {
	Uuid string
	/* TODO(safchain) how to get brige id ?, starting different agent per bridge ? */
	Host            string
	InputInterface  uint32
	OutputInterface uint32
	TenantIdSrc     string
	TenantIdDst     string
	VNISrc          string
	VNIDst          string
	EtherSrc        string
	EtherDst        string
	EtherType       string
	Ipv4Src         string
	Ipv4Dst         string
	Protocol        string
	PortSrc         uint32
	PortDst         uint32
	Id              uint64
}

func (flow *Flow) UpdateAttributes(mapper mappings.Mapper) {
	attrs := mapper.GetAttributes(flow.EtherSrc)

	flow.TenantIdSrc = attrs.TenantId
	flow.VNISrc = attrs.VNI

	attrs = mapper.GetAttributes(flow.EtherDst)
	flow.TenantIdDst = attrs.TenantId
	flow.VNIDst = attrs.VNI
}

func (flow *Flow) fillFromGoPacket(packet *gopacket.Packet) error {
	hasher := sha1.New()

	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return errors.New("Unable to decode the ethernet layer")
	}
	flow.EtherSrc = ethernetPacket.SrcMAC.String()
	flow.EtherDst = ethernetPacket.DstMAC.String()
	flow.EtherType = ethernetPacket.EthernetType.String()

	hasher.Write([]byte(flow.EtherSrc))
	hasher.Write([]byte(flow.EtherDst))
	hasher.Write([]byte(flow.EtherType))

	ipLayer := (*packet).Layer(layers.LayerTypeIPv4)
	if ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv4)
		flow.Ipv4Src = ip.SrcIP.String()
		flow.Ipv4Dst = ip.DstIP.String()
		flow.Protocol = ip.Protocol.String()

		hasher.Write([]byte(flow.Ipv4Src))
		hasher.Write([]byte(flow.Ipv4Dst))
		hasher.Write([]byte(flow.Protocol))
	}

	udpLayer := (*packet).Layer(layers.LayerTypeUDP)
	if udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		flow.PortSrc = uint32(udp.SrcPort)
		flow.PortDst = uint32(udp.DstPort)

		hasher.Write([]byte(udp.SrcPort.String()))
		hasher.Write([]byte(udp.DstPort.String()))
	}

	tcpLayer := (*packet).Layer(layers.LayerTypeTCP)
	if tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		flow.PortSrc = uint32(tcp.SrcPort)
		flow.PortDst = uint32(tcp.DstPort)

		hasher.Write([]byte(tcp.SrcPort.String()))
		hasher.Write([]byte(tcp.DstPort.String()))
	}

	icmpLayer := (*packet).Layer(layers.LayerTypeICMPv4)
	if icmpLayer != nil {
		icmp, _ := icmpLayer.(*layers.ICMPv4)
		flow.Id = uint64(icmp.Id)

		hasher.Write([]byte(strconv.Itoa(int(icmp.Id))))
	}

	/* update the temporary uuid */
	flow.Uuid = hex.EncodeToString(hasher.Sum(nil))

	return nil
}

func New(host string, in uint32, out uint32) Flow {
	u, _ := uuid.NewV4()
	flow := Flow{Uuid: u.String(), Host: host, InputInterface: in, OutputInterface: out}

	return flow
}

func FLowsFromSFlowSample(host string, sample *layers.SFlowFlowSample) []*Flow {
	flows := []*Flow{}

	for _, rec := range sample.Records {

		/* FIX(safchain): just keeping the raw packet for now */
		record, ok := rec.(layers.SFlowRawPacketFlowRecord)
		if !ok {
			continue
		}

		flow := New(host, sample.InputInterface, sample.OutputInterface)
		flow.fillFromGoPacket(&record.Header)

		flows = append(flows, &flow)
	}

	return flows
}
