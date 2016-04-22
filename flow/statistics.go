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

func (x FlowEndpointType) Value() int32 {
	return int32(x)
}

func NewFlowStatistics(packet *gopacket.Packet) *FlowStatistics {
	fs := &FlowStatistics{}
	fs.Endpoints = []*FlowEndpointsStatistics{}
	err := fs.newLinkLayerEndpointStatistics(packet)
	if err != nil {
		return fs
	}
	err = fs.newNetworkLayerEndpointStatistics(packet)
	if err != nil {
		return fs
	}
	err = fs.newTransportLayerEndpointStatistics(packet)
	if err != nil {
		return fs
	}
	return fs
}

func (fs *FlowStatistics) Update(packet *gopacket.Packet) {
	err := fs.updateLinkLayerStatistics(packet)
	if err != nil {
		return
	}
	err = fs.updateNetworkLayerStatistics(packet)
	if err != nil {
		return
	}
	err = fs.updateTransportLayerStatistics(packet)
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

func (fs *FlowStatistics) newLinkLayerEndpointStatistics(packet *gopacket.Packet) error {
	ep := &FlowEndpointsStatistics{}
	ep.AB = &FlowEndpointStatistics{}
	ep.BA = &FlowEndpointStatistics{}

	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return errors.New("Unable to decode the ethernet layer")
	}

	ep.Type = FlowEndpointType_ETHERNET
	ep.AB.Value = ethernetPacket.SrcMAC.String()
	ep.BA.Value = ethernetPacket.DstMAC.String()
	ep.hash(ethernetPacket.SrcMAC, ethernetPacket.DstMAC)
	fs.Endpoints = append(fs.Endpoints, ep)
	return nil
}

func (fs *FlowStatistics) updateLinkLayerStatistics(packet *gopacket.Packet) error {
	ep := fs.Endpoints[FlowEndpointLayer_LINK]
	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return errors.New("Unable to decode the ethernet layer")
	}

	var e *FlowEndpointStatistics
	if ep.AB.Value == ethernetPacket.SrcMAC.String() {
		e = ep.AB
	} else {
		e = ep.BA
	}
	e.Packets += uint64(1)
	if ethernetPacket.Length > 0 { // LLC
		e.Bytes += uint64(ethernetPacket.Length)
	} else {
		e.Bytes += uint64(len(ethernetPacket.Contents) + len(ethernetPacket.Payload))
	}
	return nil
}

func (fs *FlowStatistics) newNetworkLayerEndpointStatistics(packet *gopacket.Packet) error {
	ep := &FlowEndpointsStatistics{}
	ep.AB = &FlowEndpointStatistics{}
	ep.BA = &FlowEndpointStatistics{}

	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	ipv4Packet, ok := ipv4Layer.(*layers.IPv4)
	if !ok {
		return errors.New("Unable to decode the ipv4 layer")
	}

	ep.Type = FlowEndpointType_IPV4
	ep.AB.Value = ipv4Packet.SrcIP.String()
	ep.BA.Value = ipv4Packet.DstIP.String()
	ep.hash(ipv4Packet.SrcIP, ipv4Packet.DstIP)
	fs.Endpoints = append(fs.Endpoints, ep)
	return nil
}

func (fs *FlowStatistics) updateNetworkLayerStatistics(packet *gopacket.Packet) error {
	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	ipv4Packet, ok := ipv4Layer.(*layers.IPv4)
	if !ok {
		return errors.New("Unable to decode the ipv4 layer")
	}

	ep := fs.Endpoints[FlowEndpointLayer_NETWORK]

	var e *FlowEndpointStatistics
	if ep.AB.Value == ipv4Packet.SrcIP.String() {
		e = ep.AB
	} else {
		e = ep.BA
	}
	e.Packets += uint64(1)
	e.Bytes += uint64(ipv4Packet.Length)
	return nil
}

func (fs *FlowStatistics) newTransportLayerEndpointStatistics(packet *gopacket.Packet) error {
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
				return errors.New("Unable to decode the transport layer")
			}
		}
	}

	ep.Type = ptype
	switch ptype {
	case FlowEndpointType_TCPPORT:
		transportPacket, _ := transportLayer.(*layers.TCP)
		ep.AB.Value = strconv.Itoa(int(transportPacket.SrcPort))
		ep.BA.Value = strconv.Itoa(int(transportPacket.DstPort))
		ep.hash(transportPacket.SrcPort, transportPacket.DstPort)
	case FlowEndpointType_UDPPORT:
		transportPacket, _ := transportLayer.(*layers.UDP)
		ep.AB.Value = strconv.Itoa(int(transportPacket.SrcPort))
		ep.BA.Value = strconv.Itoa(int(transportPacket.DstPort))
		ep.hash(transportPacket.SrcPort, transportPacket.DstPort)
	case FlowEndpointType_SCTPPORT:
		transportPacket, _ := transportLayer.(*layers.SCTP)
		ep.AB.Value = strconv.Itoa(int(transportPacket.SrcPort))
		ep.BA.Value = strconv.Itoa(int(transportPacket.DstPort))
		ep.hash(transportPacket.SrcPort, transportPacket.DstPort)
	}
	fs.Endpoints = append(fs.Endpoints, ep)
	return nil
}

func (fs *FlowStatistics) updateTransportLayerStatistics(packet *gopacket.Packet) error {
	if len(fs.Endpoints) <= int(FlowEndpointLayer_TRANSPORT) {
		return errors.New("Unable to decode the transport layer")
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
	e.Bytes += uint64(len(transportLayer.LayerContents()) + len(transportLayer.LayerPayload()))
	return nil
}
