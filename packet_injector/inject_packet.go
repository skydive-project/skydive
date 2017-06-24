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
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

var (
	options = gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
)

// PacketParams describes the packet parameters to be injected
type PacketParams struct {
	SrcNodeID graph.Identifier `valid:"nonzero"`
	SrcIP     string           `valid:"nonzero"`
	SrcMAC    string           `valid:"nonzero"`
	DstIP     string           `valid:"nonzero"`
	DstMAC    string           `valid:"nonzero"`
	Type      string           `valid:"regexp=^(icmp4|icmp6)$"`
	Count     int64            `valid:"min=1"`
	ID        int64            `valid:"min=0"`
	Interval  int64            `valid:"min=0"`
	Payload   string
}

// InjectPacket inject some packets based on the graph
func InjectPacket(pp *PacketParams, g *graph.Graph) (string, error) {
	srcIP := getIP(pp.SrcIP)
	if srcIP == nil {
		return "", errors.New("Source Node doesn't have proper IP")
	}

	dstIP := getIP(pp.DstIP)
	if dstIP == nil {
		return "", errors.New("Destination Node doesn't have proper IP")
	}

	srcMAC, err := net.ParseMAC(pp.SrcMAC)
	if err != nil || srcMAC == nil {
		return "", errors.New("Source Node doesn't have proper MAC")
	}

	dstMAC, err := net.ParseMAC(pp.DstMAC)
	if err != nil || dstMAC == nil {
		return "", errors.New("Destination Node doesn't have proper MAC")
	}

	g.RLock()

	srcNode := g.GetNode(pp.SrcNodeID)
	if srcNode == nil {
		g.RUnlock()
		return "", errors.New("Unable to find source node")
	}

	tid, err := srcNode.GetFieldString("TID")
	if err != nil {
		g.RUnlock()
		return "", errors.New("Source node has no TID")
	}

	ifName, err := srcNode.GetFieldString("Name")
	if err != nil {
		g.RUnlock()
		return "", errors.New("Source node has no name")
	}

	_, nsPath, err := topology.NamespaceFromNode(g, srcNode)

	g.RUnlock()

	if err != nil {
		return "", err
	}

	rawSocket, err := common.NewRawSocketInNs(nsPath, ifName)
	if err != nil {
		return "", err
	}

	var l []gopacket.SerializableLayer
	ethLayer := &layers.Ethernet{SrcMAC: srcMAC, DstMAC: dstMAC}
	payload := gopacket.Payload([]byte(pp.Payload))

	switch pp.Type {
	case "icmp4":
		ethLayer.EthernetType = layers.EthernetTypeIPv4
		ipLayer := &layers.IPv4{Version: 4, SrcIP: srcIP, DstIP: dstIP, Protocol: layers.IPProtocolICMPv4}
		icmpLayer := &layers.ICMPv4{
			TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
			Id:       uint16(pp.ID),
		}
		l = append(l, ethLayer, ipLayer, icmpLayer, payload)
	case "icmp6":
		ethLayer.EthernetType = layers.EthernetTypeIPv6
		ipLayer := &layers.IPv6{Version: 6, SrcIP: srcIP, DstIP: dstIP, NextHeader: layers.IPProtocolICMPv6}
		icmpLayer := &layers.ICMPv6{
			TypeCode:  layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0),
			TypeBytes: []byte{byte(pp.ID & int64(0xFF00) >> 8), byte(pp.ID & int64(0xFF)), 0, 0},
		}
		icmpLayer.SetNetworkLayerForChecksum(ipLayer)
		l = append(l, ethLayer, ipLayer, icmpLayer, payload)
	default:
		rawSocket.Close()
		return "", fmt.Errorf("Unsupported traffic type '%s'", pp.Type)
	}

	buffer := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buffer, options, l...); err != nil {
		rawSocket.Close()
		return "", fmt.Errorf("Error while generating %s packet: %s", pp.Type, err.Error())
	}

	packetData := buffer.Bytes()
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	flowKey := flow.KeyFromGoPacket(&packet, "").String()
	flow := flow.NewFlow()
	flow.Init(flowKey, common.UnixMillis(time.Now()), &packet, int64(len(packetData)), tid, "", 0, 0)

	go func() {
		defer rawSocket.Close()

		for i := int64(0); i < pp.Count; i++ {
			logging.GetLogger().Debugf("Injecting packet on interface %s", ifName)

			if _, err := syscall.Write(rawSocket.GetFd(), packetData); err != nil {
				logging.GetLogger().Errorf("Write error: %s", err.Error())
			}

			if i != pp.Count-1 {
				time.Sleep(time.Millisecond * time.Duration(pp.Interval))
			}
		}
	}()

	return flow.TrackingID, nil
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
