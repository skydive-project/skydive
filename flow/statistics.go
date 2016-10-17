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
	"bytes"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func (x FlowProtocol) Value() int32 {
	return int32(x)
}

func (f *Flow) Init(now int64, packet *gopacket.Packet, length int64) {

	f.Metric.Start = now
	f.Metric.Last = now

	payloadLen, err := f.newLinkLayer(packet, length)
	if err != nil {
		return
	}
	payloadLen, err = f.newNetworkLayer(packet, payloadLen)
	if err != nil {
		return
	}
	_, err = f.newTransportLayer(packet, payloadLen)
	if err != nil {
		return
	}
}

func (f *Flow) Update(now int64, packet *gopacket.Packet, length int64) {
	f.Metric.Last = now

	payloadLen, err := f.updateLinkLayer(packet, length)
	if err != nil {
		return
	}
	_, err = f.updateNetworkLayer(packet, payloadLen)
	if err != nil {
		return
	}
}

func (fm *FlowMetric) Copy() *FlowMetric {
	return &FlowMetric{
		Start:     fm.Start,
		Last:      fm.Last,
		ABPackets: fm.ABPackets,
		ABBytes:   fm.ABBytes,
		BAPackets: fm.BAPackets,
		BABytes:   fm.BABytes,
	}
}

func (f *Flow) DumpInfo(layerSeparator ...string) string {
	fm := f.GetMetric()
	sep := " | "
	if len(layerSeparator) > 0 {
		sep = layerSeparator[0]
	}
	buf := bytes.NewBufferString("")
	buf.WriteString(fmt.Sprintf("%s\t", f.Link.Protocol))
	buf.WriteString(fmt.Sprintf("(%d %d)", fm.ABPackets, fm.ABBytes))
	buf.WriteString(fmt.Sprintf(" (%d %d)", fm.BAPackets, fm.BABytes))
	buf.WriteString(fmt.Sprintf("\t(%s -> %s)%s", f.Link.A, f.Link.B, sep))
	return buf.String()
}

func (f *Flow) newLinkLayer(packet *gopacket.Packet, length int64) (int64, error) {
	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		// bypass if a Link layer can't be decoded, i.e. Network layer is the first layer
		return length, nil
	}

	f.Link = &FlowLayer{
		Protocol: FlowProtocol_ETHERNET,
		A:        ethernetPacket.SrcMAC.String(),
		B:        ethernetPacket.DstMAC.String(),
	}
	return f.updateLinkLayer(packet, length)
}

func (f *Flow) updateLinkLayer(packet *gopacket.Packet, length int64) (int64, error) {
	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		// bypass if a Link layer can't be decoded, i.e. Network layer is the first layer
		return length, nil
	}

	// if the length is given use it as the packet can be truncated like in SFlow
	if length == 0 {
		if ethernetPacket.Length > 0 { // LLC
			length = 14 + int64(ethernetPacket.Length)
		} else {
			length = 14 + int64(len(ethernetPacket.Payload))
		}
	}

	if f.Link.A == ethernetPacket.SrcMAC.String() {
		f.Metric.ABPackets += int64(1)
		f.Metric.ABBytes += length
	} else {
		f.Metric.BAPackets += int64(1)
		f.Metric.BABytes += length
	}

	return length - 14, nil
}

func (f *Flow) newNetworkLayer(packet *gopacket.Packet, length int64) (int64, error) {
	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		f.Network = &FlowLayer{
			Protocol: FlowProtocol_IPV4,
			A:        ipv4Packet.SrcIP.String(),
			B:        ipv4Packet.DstIP.String(),
		}
		return f.updateNetworkLayer(packet, length)
	}

	ipv6Layer := (*packet).Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		f.Network = &FlowLayer{
			Protocol: FlowProtocol_IPV6,
			A:        ipv6Packet.SrcIP.String(),
			B:        ipv6Packet.DstIP.String(),
		}
		return f.updateNetworkLayer(packet, length)
	}

	return 0, errors.New("Unable to decode the IP layer")
}

func (f *Flow) updateNetworkLayer(packet *gopacket.Packet, length int64) (int64, error) {
	// bypass if a Link layer already exist
	if f.Link != nil {
		return length, nil
	}

	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		if f.Network.A == ipv4Packet.SrcIP.String() {
			f.Metric.ABPackets += int64(1)
			f.Metric.ABBytes += length
		} else {
			f.Metric.BAPackets += int64(1)
			f.Metric.BABytes += length
		}
		return length - int64(len(ipv4Packet.Contents)), nil
	}
	ipv6Layer := (*packet).Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		if f.Network.A == ipv6Packet.SrcIP.String() {
			f.Metric.ABPackets += int64(1)
			f.Metric.ABBytes += length
		} else {
			f.Metric.BAPackets += int64(1)
			f.Metric.BABytes += length
		}
		return length - int64(len(ipv6Packet.Contents)), nil
	}
	return 0, errors.New("Unable to decode the IP layer")
}

func (f *Flow) newTransportLayer(packet *gopacket.Packet, length int64) (int64, error) {
	var transportLayer gopacket.Layer
	var ok bool
	transportLayer = (*packet).Layer(layers.LayerTypeTCP)
	_, ok = transportLayer.(*layers.TCP)
	ptype := FlowProtocol_TCPPORT
	if !ok {
		transportLayer = (*packet).Layer(layers.LayerTypeUDP)
		_, ok = transportLayer.(*layers.UDP)
		ptype = FlowProtocol_UDPPORT
		if !ok {
			transportLayer = (*packet).Layer(layers.LayerTypeSCTP)
			_, ok = transportLayer.(*layers.SCTP)
			ptype = FlowProtocol_SCTPPORT
			if !ok {
				return 0, errors.New("Unable to decode the transport layer")
			}
		}
	}

	f.Transport = &FlowLayer{
		Protocol: ptype,
	}

	switch ptype {
	case FlowProtocol_TCPPORT:
		transportPacket, _ := transportLayer.(*layers.TCP)
		f.Transport.A = strconv.Itoa(int(transportPacket.SrcPort))
		f.Transport.B = strconv.Itoa(int(transportPacket.DstPort))
	case FlowProtocol_UDPPORT:
		transportPacket, _ := transportLayer.(*layers.UDP)
		f.Transport.A = strconv.Itoa(int(transportPacket.SrcPort))
		f.Transport.B = strconv.Itoa(int(transportPacket.DstPort))
	case FlowProtocol_SCTPPORT:
		transportPacket, _ := transportLayer.(*layers.SCTP)
		f.Transport.A = strconv.Itoa(int(transportPacket.SrcPort))
		f.Transport.B = strconv.Itoa(int(transportPacket.DstPort))
	}
	return length - int64(len(transportLayer.LayerContents())), nil
}
