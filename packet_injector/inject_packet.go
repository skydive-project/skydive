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

package packet_injector

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

var (
	options = gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
)

type PacketParams struct {
	SrcNode *graph.Node
	DstNode *graph.Node
	Type    string
	Payload string
	Count   int
}

func InjectPacket(pp *PacketParams, g *graph.Graph) error {
	srcdata := pp.SrcNode.Metadata()
	dstdata := pp.DstNode.Metadata()

	if _, ok := srcdata["IPV4"]; !ok {
		return errors.New("Source Node doesn't have an IP")
	}

	srcIP := getIP(srcdata["IPV4"].(string))
	if srcIP == nil {
		return errors.New("Source Node doesn't have proper IP")
	}

	if _, ok := dstdata["IPV4"]; !ok {
		return errors.New("Destination Node doesn't have an IP")
	}

	dstIP := getIP(dstdata["IPV4"].(string))
	if dstIP == nil {
		return errors.New("Destination Node doesn't have proper IP")
	}

	srcMAC, err := net.ParseMAC(srcdata["MAC"].(string))
	if err != nil || srcMAC == nil {
		return errors.New("Source Node doesn't have proper MAC")
	}

	dstMAC, err := net.ParseMAC(dstdata["MAC"].(string))
	if err != nil || dstMAC == nil {
		return errors.New("Destination Node doesn't have proper MAC")
	}

	//create packet
	buffer := gopacket.NewSerializeBuffer()
	ipLayer := &layers.IPv4{Version: 4, SrcIP: srcIP, DstIP: dstIP}
	ethLayer := &layers.Ethernet{EthernetType: layers.EthernetTypeIPv4, SrcMAC: srcMAC, DstMAC: dstMAC}

	switch pp.Type {
	case "icmp":
		ipLayer.Protocol = layers.IPProtocolICMPv4
		gopacket.SerializeLayers(buffer, options,
			ethLayer,
			ipLayer,
			&layers.ICMPv4{
				TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
			},
			gopacket.Payload([]byte(pp.Payload)),
		)
	default:
		return fmt.Errorf("Unsupported traffic type '%s'", pp.Type)
	}

	g.RLock()

	srcNode := g.GetNode(pp.SrcNode.ID)
	if srcNode == nil {
		g.RUnlock()
		return errors.New("Unable to find source node")
	}

	nscontext, err := topology.NewNetNSContextByNode(g, srcNode)
	defer nscontext.Close()

	g.RUnlock()

	if err != nil {
		return err
	}

	handle, err := pcap.OpenLive(srcdata["Name"].(string), 1024, false, 2000)
	if err != nil {
		return fmt.Errorf("Unable to open the source node: %s", err.Error())
	}
	defer handle.Close()

	packet := buffer.Bytes()
	for i := 0; i < pp.Count; i++ {
		if err := handle.WritePacketData(packet); err != nil {
			return fmt.Errorf("Write error: %s", err.Error())
		}
	}

	return nil
}

func getIP(cidr string) net.IP {
	if len(cidr) <= 0 {
		return nil
	}
	ips := strings.Split(cidr, ",")
	//TODO(masco): currently taking first IP, need to implement to select a proper IP
	ip, _, err := net.ParseCIDR(ips[0])
	if err != nil {
		return nil
	}
	return ip
}
