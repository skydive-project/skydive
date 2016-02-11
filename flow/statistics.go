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
	"strconv"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func (x FlowEndpointType) Value() int32 {
	return int32(x)
}

func NewFlowStatistics() *FlowStatistics {
	fs := &FlowStatistics{}
	fs.Endpoints = make(map[int32]*FlowEndpointsStatistics)
	return fs
}

func (fs *FlowStatistics) newEthernetEndpointStatistics(packet *gopacket.Packet) error {
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
	fs.Endpoints[FlowEndpointType_ETHERNET.Value()] = ep
	return nil
}

func (fs *FlowStatistics) updateEthernetFromGoPacket(packet *gopacket.Packet) error {
	ep := fs.Endpoints[FlowEndpointType_ETHERNET.Value()]
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

func (fs *FlowStatistics) newIPV4EndpointStatistics(packet *gopacket.Packet) error {
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
	fs.Endpoints[FlowEndpointType_IPV4.Value()] = ep
	return nil
}

func (fs *FlowStatistics) updateIPV4FromGoPacket(packet *gopacket.Packet) error {
	ep := fs.Endpoints[FlowEndpointType_IPV4.Value()]

	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	ipv4Packet, ok := ipv4Layer.(*layers.IPv4)
	if !ok {
		return errors.New("Unable to decode the ipv4 layer")
	}

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

func (fs *FlowStatistics) newTransportEndpointStatistics(packet *gopacket.Packet) error {
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
	case FlowEndpointType_UDPPORT:
		transportPacket, _ := transportLayer.(*layers.UDP)
		ep.AB.Value = strconv.Itoa(int(transportPacket.SrcPort))
		ep.BA.Value = strconv.Itoa(int(transportPacket.DstPort))
	case FlowEndpointType_SCTPPORT:
		transportPacket, _ := transportLayer.(*layers.SCTP)
		ep.AB.Value = strconv.Itoa(int(transportPacket.SrcPort))
		ep.BA.Value = strconv.Itoa(int(transportPacket.DstPort))
	}
	fs.Endpoints[ptype.Value()] = ep
	return nil
}

func (fs *FlowStatistics) updateTransportFromGoPacket(packet *gopacket.Packet) error {
	var transportLayer gopacket.Layer
	transportLayer = (*packet).Layer(layers.LayerTypeTCP)
	_, ok := transportLayer.(*layers.TCP)
	ptype := FlowEndpointType_TCPPORT
	if !ok {
		transportLayer = (*packet).Layer(layers.LayerTypeUDP)
		_, ok := transportLayer.(*layers.UDP)
		ptype = FlowEndpointType_UDPPORT
		if !ok {
			transportLayer = (*packet).Layer(layers.LayerTypeSCTP)
			_, ok := transportLayer.(*layers.SCTP)
			ptype = FlowEndpointType_SCTPPORT
			if !ok {
				return errors.New("Unable to decode the transport layer")
			}
		}
	}

	ep := fs.Endpoints[ptype.Value()]
	var srcPort string

	switch ptype {
	case FlowEndpointType_TCPPORT:
		transportPacket, _ := transportLayer.(*layers.TCP)
		srcPort = transportPacket.SrcPort.String()
	case FlowEndpointType_UDPPORT:
		transportPacket, _ := transportLayer.(*layers.UDP)
		srcPort = transportPacket.SrcPort.String()
	case FlowEndpointType_SCTPPORT:
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
