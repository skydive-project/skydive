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
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func (x FlowEndpointType) Value() int32 {
	return int32(x)
}

func Var8bin(v []byte) []byte {
	r := make([]byte, 8)
	skip := 8 - len(v)
	for i, b := range v {
		r[i+skip] = b
	}
	return r
}

func HashFromValues(ab interface{}, ba interface{}) []byte {
	var vab, vba uint64
	var binab, binba []byte

	hasher := sha1.New()
	switch ab.(type) {
	case net.HardwareAddr:
		binab = ab.(net.HardwareAddr)
		binba = ba.(net.HardwareAddr)
		vab = binary.BigEndian.Uint64(Var8bin(binab))
		vba = binary.BigEndian.Uint64(Var8bin(binba))
	case net.IP:
		// IP can be IPV4 or IPV6
		binab = ab.(net.IP).To16()
		binba = ba.(net.IP).To16()

		vab = binary.BigEndian.Uint64(Var8bin(binab[:8]))
		vba = binary.BigEndian.Uint64(Var8bin(binba[:8]))
		if vab == vba {
			vab = binary.BigEndian.Uint64(Var8bin(binab[8:]))
			vba = binary.BigEndian.Uint64(Var8bin(binba[8:]))
		}
	case layers.TCPPort:
		binab = make([]byte, 2)
		binba = make([]byte, 2)
		binary.BigEndian.PutUint16(binab, uint16(ab.(layers.TCPPort)))
		binary.BigEndian.PutUint16(binba, uint16(ba.(layers.TCPPort)))
		vab = uint64(ab.(layers.TCPPort))
		vba = uint64(ba.(layers.TCPPort))
	case layers.UDPPort:
		binab = make([]byte, 2)
		binba = make([]byte, 2)
		binary.BigEndian.PutUint16(binab, uint16(ab.(layers.UDPPort)))
		binary.BigEndian.PutUint16(binba, uint16(ba.(layers.UDPPort)))
		vab = uint64(ab.(layers.UDPPort))
		vba = uint64(ba.(layers.UDPPort))
	case layers.SCTPPort:
		binab = make([]byte, 2)
		binba = make([]byte, 2)
		binary.BigEndian.PutUint16(binab, uint16(ab.(layers.SCTPPort)))
		binary.BigEndian.PutUint16(binba, uint16(ba.(layers.SCTPPort)))
		vab = uint64(ab.(layers.SCTPPort))
		vba = uint64(ba.(layers.SCTPPort))
	}

	if vab < vba {
		hasher.Write(binab)
		hasher.Write(binba)
	} else {
		hasher.Write(binba)
		hasher.Write(binab)
	}
	return hasher.Sum(nil)
}

func (fs *FlowStatistics) Init(now int64, packet *gopacket.Packet, length uint64) {
	fs.Start = now
	fs.Last = now

	fs.Endpoints = []*FlowEndpointsStatistics{}
	payloadLen, err := fs.newLinkLayerEndpointStatistics(packet, length)
	if err != nil {
		return
	}
	payloadLen, err = fs.newNetworkLayerEndpointStatistics(packet, payloadLen)
	if err != nil {
		return
	}
	_, err = fs.newTransportLayerEndpointStatistics(packet, payloadLen)
	if err != nil {
		return
	}
}

func (fs *FlowStatistics) Update(now int64, packet *gopacket.Packet, length uint64) {
	fs.Last = now

	payloadLen, err := fs.updateLinkLayerStatistics(packet, length)
	if err != nil {
		return
	}
	// TODO MPLS ?
	payloadLen, err = fs.updateNetworkLayerStatistics(packet, payloadLen)
	if err != nil {
		return
	}
	_, err = fs.updateTransportLayerStatistics(packet, payloadLen)
	if err != nil {
		return
	}
}

func (fs *FlowStatistics) DumpInfo(layerSeparator ...string) string {
	sep := " | "
	if len(layerSeparator) > 0 {
		sep = layerSeparator[0]
	}
	buf := bytes.NewBufferString("")
	for _, ep := range fs.Endpoints {
		buf.WriteString(fmt.Sprintf("%s\t", ep.Type))
		buf.WriteString(fmt.Sprintf("(%d %d)", ep.AB.Packets, ep.AB.Bytes))
		buf.WriteString(fmt.Sprintf(" (%d %d)", ep.BA.Packets, ep.BA.Bytes))
		buf.WriteString(fmt.Sprintf("\t(%s -> %s)%s", ep.AB.Value, ep.BA.Value, sep))
	}
	return buf.String()
}

func (fs *FlowStatistics) GetEndpointsType(eptype FlowEndpointType) *FlowEndpointsStatistics {
	for _, ep := range fs.Endpoints {
		if ep.Type == eptype {
			return ep
		}
	}
	return nil
}

func (fs *FlowStatistics) GetLayerHash(ltype FlowEndpointType) (hash []byte) {
	e := fs.GetEndpointsType(ltype)
	if e == nil {
		return
	}
	return e.Hash
}

func (fs *FlowStatistics) newLinkLayerEndpointStatistics(packet *gopacket.Packet, length uint64) (uint64, error) {
	ep := &FlowEndpointsStatistics{}
	ep.AB = &FlowEndpointStatistics{}
	ep.BA = &FlowEndpointStatistics{}

	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return 0, errors.New("Unable to decode the ethernet layer")
	}

	ep.Type = FlowEndpointType_ETHERNET
	ep.AB.Value = ethernetPacket.SrcMAC.String()
	ep.BA.Value = ethernetPacket.DstMAC.String()
	ep.Hash = HashFromValues(ethernetPacket.SrcMAC, ethernetPacket.DstMAC)
	fs.Endpoints = append(fs.Endpoints, ep)
	return fs.updateLinkLayerStatistics(packet, length)
}

func (fs *FlowStatistics) updateLinkLayerStatistics(packet *gopacket.Packet, length uint64) (uint64, error) {
	ep := fs.Endpoints[FlowEndpointLayer_LINK]
	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return 0, errors.New("Unable to decode the ethernet layer")
	}

	var e *FlowEndpointStatistics
	if ep.AB.Value == ethernetPacket.SrcMAC.String() {
		e = ep.AB
	} else {
		e = ep.BA
	}

	e.Packets += uint64(1)

	// if the length is given use it as the packet can be truncated like in SFlow
	if length == 0 {
		if ethernetPacket.Length > 0 { // LLC
			length = 14 + uint64(ethernetPacket.Length)
		} else {
			length = 14 + uint64(len(ethernetPacket.Payload))
		}
	}
	e.Bytes += length

	return length - 14, nil
}

func (fs *FlowStatistics) newNetworkLayerEndpointStatistics(packet *gopacket.Packet, length uint64) (uint64, error) {
	ep := &FlowEndpointsStatistics{}
	ep.AB = &FlowEndpointStatistics{}
	ep.BA = &FlowEndpointStatistics{}

	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		ep.Type = FlowEndpointType_IPV4
		ep.AB.Value = ipv4Packet.SrcIP.String()
		ep.BA.Value = ipv4Packet.DstIP.String()
		ep.Hash = HashFromValues(ipv4Packet.SrcIP, ipv4Packet.DstIP)
		fs.Endpoints = append(fs.Endpoints, ep)
		return fs.updateNetworkLayerStatistics(packet, length)
	}

	ipv6Layer := (*packet).Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		ep.Type = FlowEndpointType_IPV6
		ep.AB.Value = ipv6Packet.SrcIP.String()
		ep.BA.Value = ipv6Packet.DstIP.String()
		ep.Hash = HashFromValues(ipv6Packet.SrcIP, ipv6Packet.DstIP)
		fs.Endpoints = append(fs.Endpoints, ep)
		return fs.updateNetworkLayerStatistics(packet, length)
	}

	return 0, errors.New("Unable to decode the IP layer")
}

func (fs *FlowStatistics) updateNetworkLayerStatistics(packet *gopacket.Packet, length uint64) (uint64, error) {
	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		ep := fs.Endpoints[FlowEndpointLayer_NETWORK]

		var e *FlowEndpointStatistics
		if ep.AB.Value == ipv4Packet.SrcIP.String() {
			e = ep.AB
		} else {
			e = ep.BA
		}
		e.Packets += uint64(1)
		e.Bytes += length
		return length - uint64(len(ipv4Packet.Contents)), nil
	}

	ipv6Layer := (*packet).Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		ep := fs.Endpoints[FlowEndpointLayer_NETWORK]

		var e *FlowEndpointStatistics
		if ep.AB.Value == ipv6Packet.SrcIP.String() {
			e = ep.AB
		} else {
			e = ep.BA
		}
		e.Packets += uint64(1)
		e.Bytes += length
		return length - uint64(len(ipv6Packet.Contents)), nil
	}

	return 0, errors.New("Unable to decode the IP layer")
}

func (fs *FlowStatistics) newTransportLayerEndpointStatistics(packet *gopacket.Packet, length uint64) (uint64, error) {
	ep := &FlowEndpointsStatistics{}
	ep.AB = &FlowEndpointStatistics{}
	ep.BA = &FlowEndpointStatistics{}

	var transportLayer gopacket.Layer
	var ok bool
	transportLayer = (*packet).Layer(layers.LayerTypeTCP)
	_, ok = transportLayer.(*layers.TCP)
	ptype := FlowEndpointType_TCPPORT
	if !ok {
		transportLayer = (*packet).Layer(layers.LayerTypeUDP)
		_, ok = transportLayer.(*layers.UDP)
		ptype = FlowEndpointType_UDPPORT
		if !ok {
			transportLayer = (*packet).Layer(layers.LayerTypeSCTP)
			_, ok = transportLayer.(*layers.SCTP)
			ptype = FlowEndpointType_SCTPPORT
			if !ok {
				return 0, errors.New("Unable to decode the transport layer")
			}
		}
	}

	ep.Type = ptype
	switch ptype {
	case FlowEndpointType_TCPPORT:
		transportPacket, _ := transportLayer.(*layers.TCP)
		ep.AB.Value = strconv.Itoa(int(transportPacket.SrcPort))
		ep.BA.Value = strconv.Itoa(int(transportPacket.DstPort))
		ep.Hash = HashFromValues(transportPacket.SrcPort, transportPacket.DstPort)
	case FlowEndpointType_UDPPORT:
		transportPacket, _ := transportLayer.(*layers.UDP)
		ep.AB.Value = strconv.Itoa(int(transportPacket.SrcPort))
		ep.BA.Value = strconv.Itoa(int(transportPacket.DstPort))
		ep.Hash = HashFromValues(transportPacket.SrcPort, transportPacket.DstPort)
	case FlowEndpointType_SCTPPORT:
		transportPacket, _ := transportLayer.(*layers.SCTP)
		ep.AB.Value = strconv.Itoa(int(transportPacket.SrcPort))
		ep.BA.Value = strconv.Itoa(int(transportPacket.DstPort))
		ep.Hash = HashFromValues(transportPacket.SrcPort, transportPacket.DstPort)
	}
	fs.Endpoints = append(fs.Endpoints, ep)
	return fs.updateTransportLayerStatistics(packet, length)
}

func (fs *FlowStatistics) updateTransportLayerStatistics(packet *gopacket.Packet, length uint64) (uint64, error) {
	if len(fs.Endpoints) <= int(FlowEndpointLayer_TRANSPORT) {
		return 0, errors.New("Unable to decode the transport layer")
	}
	ep := fs.Endpoints[FlowEndpointLayer_TRANSPORT]

	var transportLayer gopacket.Layer
	var srcPort string
	switch ep.Type {
	case FlowEndpointType_TCPPORT:
		transportLayer = (*packet).Layer(layers.LayerTypeTCP)
		transportPacket, _ := transportLayer.(*layers.TCP)
		srcPort = transportPacket.SrcPort.String()
	case FlowEndpointType_UDPPORT:
		transportLayer = (*packet).Layer(layers.LayerTypeUDP)
		transportPacket, _ := transportLayer.(*layers.UDP)
		srcPort = transportPacket.SrcPort.String()
	case FlowEndpointType_SCTPPORT:
		transportLayer = (*packet).Layer(layers.LayerTypeSCTP)
		transportPacket, _ := transportLayer.(*layers.SCTP)
		srcPort = transportPacket.SrcPort.String()
	}
	var e *FlowEndpointStatistics
	if ep.AB.Value == srcPort {
		e = ep.AB
	} else {
		e = ep.BA
	}
	e.Packets += uint64(1)
	e.Bytes += length
	return length - uint64(len(transportLayer.LayerContents())), nil
}

type FlowSetBandwidth struct {
	ABpackets uint64
	ABbytes   uint64
	BApackets uint64
	BAbytes   uint64
	Duration  int64
	NBFlow    uint64
}

func (fsbw FlowSetBandwidth) String() string {
	return fmt.Sprintf("dt : %d seconds nbFlow %d\n\t\tAB -> BA\nPackets : %8d %8d\nBytes   : %8d %8d\n", fsbw.Duration, fsbw.NBFlow, fsbw.ABpackets, fsbw.BApackets, fsbw.ABbytes, fsbw.BAbytes)
}
