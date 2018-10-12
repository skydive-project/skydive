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

package packetinjector

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
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

// PacketInjectionParams describes the packet parameters to be injected
type PacketInjectionParams struct {
	UUID      string
	SrcNodeID graph.Identifier `valid:"nonzero"`
	SrcIP     string           `valid:"nonzero"`
	SrcMAC    string           `valid:"nonzero"`
	SrcPort   int64            `valid:"min=0"`
	DstIP     string           `valid:"nonzero"`
	DstMAC    string           `valid:"nonzero"`
	DstPort   int64            `valid:"min=0"`
	Type      string           `valid:"regexp=^(icmp4|icmp6|tcp4|tcp6|udp4|udp6)$"`
	Count     int64            `valid:"min=1"`
	ID        int64            `valid:"min=0"`
	Interval  int64            `valid:"min=0"`
	Increment bool
	Payload   string
}

type channels struct {
	sync.Mutex
	Pipes map[string](chan bool)
}

func forgePacket(packetType string, layerType gopacket.LayerType, srcMAC, dstMAC net.HardwareAddr, srcIP, dstIP net.IP, srcPort, dstPort int64, ID int64, data string) ([]byte, gopacket.Packet, error) {
	var l []gopacket.SerializableLayer

	// use same size as ping when no payload specified
	if len(data) == 0 {
		data = common.RandString(56)
	}

	payload := gopacket.Payload([]byte(data))

	if layerType == layers.LayerTypeEthernet {
		ethLayer := &layers.Ethernet{SrcMAC: srcMAC, DstMAC: dstMAC}
		switch packetType {
		case "icmp4", "tcp4", "udp4":
			ethLayer.EthernetType = layers.EthernetTypeIPv4
		case "icmp6", "tcp6", "udp6":
			ethLayer.EthernetType = layers.EthernetTypeIPv6
		}
		l = append(l, ethLayer)
	}

	switch packetType {
	case "icmp4":
		ipLayer := &layers.IPv4{Version: 4, SrcIP: srcIP, DstIP: dstIP, Protocol: layers.IPProtocolICMPv4}
		icmpLayer := &layers.ICMPv4{
			TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
			Id:       uint16(ID),
		}
		l = append(l, ipLayer, icmpLayer)
	case "icmp6":
		ipLayer := &layers.IPv6{Version: 6, SrcIP: srcIP, DstIP: dstIP, NextHeader: layers.IPProtocolICMPv6}
		icmpLayer := &layers.ICMPv6{
			TypeCode:  layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0),
			TypeBytes: []byte{byte(ID & int64(0xFF00) >> 8), byte(ID & int64(0xFF)), 0, 0},
		}
		icmpLayer.SetNetworkLayerForChecksum(ipLayer)

		echoLayer := &layers.ICMPv6Echo{
			Identifier: uint16(ID),
		}
		l = append(l, ipLayer, icmpLayer, echoLayer)
	case "tcp4":
		ipLayer := &layers.IPv4{SrcIP: srcIP, DstIP: dstIP, Version: 4, Protocol: layers.IPProtocolTCP, TTL: 64}
		srcPort := layers.TCPPort(srcPort)
		dstPort := layers.TCPPort(dstPort)
		tcpLayer := &layers.TCP{SrcPort: srcPort, DstPort: dstPort, Seq: rand.Uint32(), SYN: true}
		tcpLayer.SetNetworkLayerForChecksum(ipLayer)
		l = append(l, ipLayer, tcpLayer)
	case "tcp6":
		ipLayer := &layers.IPv6{Version: 6, SrcIP: srcIP, DstIP: dstIP, NextHeader: layers.IPProtocolTCP}
		srcPort := layers.TCPPort(srcPort)
		dstPort := layers.TCPPort(dstPort)
		tcpLayer := &layers.TCP{SrcPort: srcPort, DstPort: dstPort, Seq: rand.Uint32(), SYN: true}
		tcpLayer.SetNetworkLayerForChecksum(ipLayer)
		l = append(l, ipLayer, tcpLayer)
	case "udp4":
		ipLayer := &layers.IPv4{SrcIP: srcIP, DstIP: dstIP, Version: 4, Protocol: layers.IPProtocolUDP, TTL: 64}
		srcPort := layers.UDPPort(srcPort)
		dstPort := layers.UDPPort(dstPort)
		udpLayer := &layers.UDP{SrcPort: srcPort, DstPort: dstPort}
		udpLayer.SetNetworkLayerForChecksum(ipLayer)
		l = append(l, ipLayer, udpLayer)
	case "udp6":
		ipLayer := &layers.IPv6{SrcIP: srcIP, DstIP: dstIP, Version: 6, NextHeader: layers.IPProtocolUDP}
		srcPort := layers.UDPPort(srcPort)
		dstPort := layers.UDPPort(dstPort)
		udpLayer := &layers.UDP{SrcPort: srcPort, DstPort: dstPort}
		udpLayer.SetNetworkLayerForChecksum(ipLayer)
		l = append(l, ipLayer, udpLayer)
	default:
		return nil, nil, fmt.Errorf("Unsupported traffic type '%s'", packetType)
	}
	l = append(l, payload)

	buffer := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buffer, options, l...); err != nil {
		return nil, nil, fmt.Errorf("Error while generating %s packet: %s", packetType, err)
	}

	packetData := buffer.Bytes()
	return packetData, gopacket.NewPacket(packetData, layerType, gopacket.Default), nil
}

// InjectPackets inject some packets based on the graph
func InjectPackets(pp *PacketInjectionParams, g *graph.Graph, chnl *channels) (string, error) {
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

	encapType, _ := srcNode.GetFieldString("EncapType")

	_, nsPath, err := topology.NamespaceFromNode(g, srcNode)
	g.RUnlock()
	if err != nil {
		return "", err
	}

	protocol := common.AllPackets
	layerType, _ := flow.GetFirstLayerType(encapType)
	switch layerType {
	case flow.LayerTypeRawIP, layers.LayerTypeIPv4, layers.LayerTypeIPv6:
		protocol = common.OnlyIPPackets
	}

	var rawSocket *common.RawSocket
	if nsPath != "" {
		rawSocket, err = common.NewRawSocketInNs(nsPath, ifName, protocol)
	} else {
		rawSocket, err = common.NewRawSocket(ifName, protocol)
	}
	if err != nil {
		return "", err
	}

	packetData, gpacket, err := forgePacket(pp.Type, layerType, srcMAC, dstMAC, srcIP, dstIP, pp.SrcPort, pp.DstPort, pp.ID, pp.Payload)
	if err != nil {
		rawSocket.Close()
		return "", err
	}

	gopacket.NewPacket(packetData, layerType, gopacket.Default)

	f := flow.NewFlowFromGoPacket(gpacket, tid, flow.UUIDs{}, flow.Opts{})

	p := make(chan bool)
	chnl.Lock()
	chnl.Pipes[pp.UUID] = p
	chnl.Unlock()

	go func(c chan bool) {
		defer rawSocket.Close()

	stopInjection:
		for i := int64(0); i < pp.Count; i++ {
			select {
			case <-c:
				logging.GetLogger().Debugf("Injection stopped on interface %s", ifName)
				break stopInjection
			default:
				logging.GetLogger().Debugf("Injecting packet on interface %s", ifName)

				if _, err := rawSocket.Write(packetData); err != nil {
					if err == syscall.ENXIO {
						logging.GetLogger().Warningf("Write error on interface %s: %s", ifName, err)
					} else {
						logging.GetLogger().Errorf("Write error on interface %s: %s", ifName, err)
					}
				}

				if i != pp.Count-1 {
					time.Sleep(time.Millisecond * time.Duration(pp.Interval))
				}

				if strings.HasPrefix(pp.Type, "icmp") && pp.Increment {
					if packetData, _, err = forgePacket(pp.Type, layerType, srcMAC, dstMAC, srcIP, dstIP, pp.SrcPort, pp.DstPort, pp.ID+i+1, pp.Payload); err != nil {
						logging.GetLogger().Error(err)
						return
					}
				}
			}
		}
		chnl.Lock()
		delete(chnl.Pipes, pp.UUID)
		chnl.Unlock()
	}(p)

	return f.TrackingID, nil
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
