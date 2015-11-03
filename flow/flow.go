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
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/nu7hatch/gouuid"
)

func (flow *Flow) fillFromGoPacket(packet *gopacket.Packet) error {
	hasher := sha1.New()

	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return errors.New("Unable to decode the ethernet layer")
	}
	flow.EtherSrc = proto.String(ethernetPacket.SrcMAC.String())
	flow.EtherDst = proto.String(ethernetPacket.DstMAC.String())

	hasher.Write([]byte(flow.GetEtherSrc()))
	hasher.Write([]byte(flow.GetEtherDst()))

	ipLayer := (*packet).Layer(layers.LayerTypeIPv4)
	if ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv4)

		flow.Ipv4Src = proto.String(ip.SrcIP.String())
		flow.Ipv4Dst = proto.String(ip.DstIP.String())

		hasher.Write([]byte(flow.GetIpv4Src()))
		hasher.Write([]byte(flow.GetIpv4Dst()))
	}

	path := ""
	for i, layer := range (*packet).Layers() {
		if i > 0 {
			path += "."
		}
		path += layer.LayerType().String()
	}
	flow.Path = proto.String(path)
	hasher.Write([]byte(flow.GetPath()))

	udpLayer := (*packet).Layer(layers.LayerTypeUDP)
	if udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		flow.PortSrc = proto.Uint32(uint32(udp.SrcPort))
		flow.PortDst = proto.Uint32(uint32(udp.DstPort))

		hasher.Write([]byte(udp.SrcPort.String()))
		hasher.Write([]byte(udp.DstPort.String()))
	}

	tcpLayer := (*packet).Layer(layers.LayerTypeTCP)
	if tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		flow.PortSrc = proto.Uint32(uint32(tcp.SrcPort))
		flow.PortDst = proto.Uint32(uint32(tcp.DstPort))

		hasher.Write([]byte(tcp.SrcPort.String()))
		hasher.Write([]byte(tcp.DstPort.String()))
	}

	icmpLayer := (*packet).Layer(layers.LayerTypeICMPv4)
	if icmpLayer != nil {
		icmp, _ := icmpLayer.(*layers.ICMPv4)
		flow.ID = proto.Uint64(uint64(icmp.Id))

		hasher.Write([]byte(strconv.Itoa(int(icmp.Id))))
	}

	/* update the temporary uuid */
	flow.UUID = proto.String(hex.EncodeToString(hasher.Sum(nil)))

	return nil
}

func FromData(data []byte) (*Flow, error) {
	flow := new(Flow)

	err := proto.Unmarshal(data, flow)
	if err != nil {
		return nil, err
	}

	return flow, nil
}

func (flow *Flow) GetData() ([]byte, error) {
	data, err := proto.Marshal(flow)
	if err != nil {
		return []byte{}, err
	}

	return data, nil
}

func New(host string, in uint32, out uint32, packet *gopacket.Packet) *Flow {
	u, _ := uuid.NewV4()
	t := uint64(time.Now().Unix())

	flow := &Flow{
		UUID: proto.String(u.String()),
		Host: proto.String(host),
		Timestamp: proto.Uint64(t),
		Attributes: &Flow_MappingAttributes{
			IntfAttrSrc: &Flow_InterfaceAttributes{},
			IntfAttrDst: &Flow_InterfaceAttributes{},
		},
	}
	flow.GetAttributes().GetIntfAttrSrc().IfIndex = proto.Uint32(in)
	flow.GetAttributes().GetIntfAttrDst().IfIndex = proto.Uint32(out)

	if packet != nil {
		flow.fillFromGoPacket(packet)
	}

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

		flow := New(host, sample.InputInterface, sample.OutputInterface, &record.Header)
		flows = append(flows, flow)
	}

	return flows
}
