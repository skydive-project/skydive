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

type InterfaceAttributes struct {
	TenantId   string
	VNI        string
	IfIndex    uint32
	IfName     string
	MTU        uint32
	BridgeName string
}

type Attributes struct {
	IntfAttrSrc InterfaceAttributes
	IntfAttrDst InterfaceAttributes
}

/* TODO(safchain) maybe use the proto object instead of this one */
type Flow struct {
	Uuid       string
	Host       string
	EtherSrc   string
	EtherDst   string
	EtherType  string
	Ipv4Src    string
	Ipv4Dst    string
	Protocol   string
	Path       string
	PortSrc    uint32
	PortDst    uint32
	Id         uint64
	Timestamp  uint64
	Attributes Attributes
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

	path := ""
	for i, layer := range (*packet).Layers() {
		if i > 0 {
			path += "."
		}
		path += layer.LayerType().String()
	}
	flow.Path = path
	hasher.Write([]byte(flow.Path))

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

/* TODO(safchain) should it be removed using the proto structures directly ? */
func FromData(data []byte) (*Flow, error) {
	p := new(FlowMessage)

	err := proto.Unmarshal(data, p)
	if err != nil {
		return nil, err
	}

	f := &Flow{
		Uuid:      p.GetUuid(),
		Host:      p.GetHost(),
		EtherSrc:  p.GetEtherSrc(),
		EtherDst:  p.GetEtherDst(),
		Ipv4Src:   p.GetIpv4Src(),
		Ipv4Dst:   p.GetIpv4Dst(),
		Path:      p.GetPath(),
		PortSrc:   p.GetPortSrc(),
		PortDst:   p.GetPortDst(),
		Id:        p.GetId(),
		Timestamp: p.GetTimestamp(),
		Attributes: Attributes{
			IntfAttrSrc: InterfaceAttributes{
				TenantId:   p.GetAttributes().GetIntfAttrSrc().GetTenantId(),
				VNI:        p.GetAttributes().GetIntfAttrSrc().GetVNI(),
				IfIndex:    p.GetAttributes().GetIntfAttrSrc().GetIfIndex(),
				IfName:     p.GetAttributes().GetIntfAttrSrc().GetIfName(),
				MTU:        p.GetAttributes().GetIntfAttrSrc().GetMTU(),
				BridgeName: p.GetAttributes().GetIntfAttrSrc().GetBridgeName(),
			},
			IntfAttrDst: InterfaceAttributes{
				TenantId:   p.GetAttributes().GetIntfAttrDst().GetTenantId(),
				VNI:        p.GetAttributes().GetIntfAttrDst().GetVNI(),
				IfIndex:    p.GetAttributes().GetIntfAttrDst().GetIfIndex(),
				IfName:     p.GetAttributes().GetIntfAttrDst().GetIfName(),
				MTU:        p.GetAttributes().GetIntfAttrDst().GetMTU(),
				BridgeName: p.GetAttributes().GetIntfAttrDst().GetBridgeName(),
			},
		},
	}

	return f, nil
}

func (flow *Flow) GetData() ([]byte, error) {
	m := &FlowMessage{
		Uuid:      proto.String(flow.Uuid),
		Host:      proto.String(flow.Host),
		EtherSrc:  proto.String(flow.EtherSrc),
		EtherDst:  proto.String(flow.EtherDst),
		EtherType: proto.String(flow.EtherType),
		Ipv4Src:   proto.String(flow.Ipv4Src),
		Ipv4Dst:   proto.String(flow.Ipv4Dst),
		Path:      proto.String(flow.Path),
		PortSrc:   proto.Uint32(flow.PortSrc),
		PortDst:   proto.Uint32(flow.PortDst),
		Id:        proto.Uint64(flow.Id),
		Timestamp: proto.Uint64(flow.Timestamp),

		Attributes: &FlowMessage_Attrs{
			IntfAttrSrc: &FlowMessage_InterfaceAttributes{
				TenantId:   proto.String(flow.Attributes.IntfAttrSrc.TenantId),
				VNI:        proto.String(flow.Attributes.IntfAttrSrc.VNI),
				IfIndex:    proto.Uint32(flow.Attributes.IntfAttrSrc.IfIndex),
				IfName:     proto.String(flow.Attributes.IntfAttrSrc.IfName),
				MTU:        proto.Uint32(flow.Attributes.IntfAttrSrc.MTU),
				BridgeName: proto.String(flow.Attributes.IntfAttrSrc.BridgeName),
			},
			IntfAttrDst: &FlowMessage_InterfaceAttributes{
				TenantId:   proto.String(flow.Attributes.IntfAttrDst.TenantId),
				VNI:        proto.String(flow.Attributes.IntfAttrDst.VNI),
				IfIndex:    proto.Uint32(flow.Attributes.IntfAttrDst.IfIndex),
				IfName:     proto.String(flow.Attributes.IntfAttrDst.IfName),
				MTU:        proto.Uint32(flow.Attributes.IntfAttrDst.MTU),
				BridgeName: proto.String(flow.Attributes.IntfAttrDst.BridgeName),
			},
		},
	}

	data, err := proto.Marshal(m)
	if err != nil {
		return []byte{}, err
	}

	return data, nil
}

func New(host string, in uint32, out uint32, packet *gopacket.Packet) *Flow {
	u, _ := uuid.NewV4()
	t := uint64(time.Now().Unix())

	flow := &Flow{Uuid: u.String(), Host: host, Timestamp: t}
	flow.Attributes.IntfAttrSrc.IfIndex = in
	flow.Attributes.IntfAttrDst.IfIndex = out

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
