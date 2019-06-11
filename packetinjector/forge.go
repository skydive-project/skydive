/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package packetinjector

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
)

// ForgedPacketGenerator is used to forge packets. It creates
// a gopacket.Packet with the proper layers that can be directly
// inserted into a socket.
type ForgedPacketGenerator struct {
	*PacketInjectionParams
	layerType      gopacket.LayerType
	srcMAC, dstMAC net.HardwareAddr
	srcIP, dstIP   net.IP
}

func forgePacket(packetType string, layerType gopacket.LayerType, srcMAC, dstMAC net.HardwareAddr, TTL uint8, srcIP, dstIP net.IP, srcPort, dstPort uint16, ID uint64, data string) ([]byte, gopacket.Packet, error) {
	var l []gopacket.SerializableLayer

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
		ipLayer := &layers.IPv4{Version: 4, SrcIP: srcIP, DstIP: dstIP, Protocol: layers.IPProtocolICMPv4, TTL: TTL}
		icmpLayer := &layers.ICMPv4{
			TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
			Id:       uint16(ID),
		}
		l = append(l, ipLayer, icmpLayer)
	case "icmp6":
		ipLayer := &layers.IPv6{Version: 6, SrcIP: srcIP, DstIP: dstIP, NextHeader: layers.IPProtocolICMPv6}
		icmpLayer := &layers.ICMPv6{
			TypeCode:  layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0),
			TypeBytes: []byte{byte(ID & uint64(0xFF00) >> 8), byte(ID & uint64(0xFF)), 0, 0},
		}
		icmpLayer.SetNetworkLayerForChecksum(ipLayer)

		echoLayer := &layers.ICMPv6Echo{
			Identifier: uint16(ID),
		}
		l = append(l, ipLayer, icmpLayer, echoLayer)
	case "tcp4":
		ipLayer := &layers.IPv4{SrcIP: srcIP, DstIP: dstIP, Version: 4, Protocol: layers.IPProtocolTCP, TTL: TTL}
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
		ipLayer := &layers.IPv4{SrcIP: srcIP, DstIP: dstIP, Version: 4, Protocol: layers.IPProtocolUDP, TTL: TTL}
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

// PacketSource returns a channel when forged packets are pushed
func (f *ForgedPacketGenerator) PacketSource() chan *Packet {
	ch := make(chan *Packet)

	go func() {
		payload := f.Payload
		// use same size as ping when no payload specified
		if len(payload) == 0 {
			payload = common.RandString(56)
		}

		for i := uint64(0); i < f.Count; i++ {
			id := uint64(f.ID)
			if strings.HasPrefix(f.Type, "icmp") && f.Increment {
				id += i
			}

			if f.IncrementPayload > 0 {
				payload = payload + common.RandString(int(f.IncrementPayload))
			}

			packetData, packet, err := forgePacket(f.Type, f.layerType, f.srcMAC, f.dstMAC, f.TTL, f.srcIP, f.dstIP, f.SrcPort, f.DstPort, id, payload)
			if err != nil {
				logging.GetLogger().Error(err)
				return
			}

			ch <- &Packet{data: packetData, gopacket: packet}

			if i != f.Count-1 {
				time.Sleep(time.Millisecond * time.Duration(f.Interval))
			}
		}
	}()

	return ch
}

// NewForgedPacketGenerator returns a new generator of forged packets
func NewForgedPacketGenerator(pp *PacketInjectionParams, srcNode *graph.Node) (*ForgedPacketGenerator, error) {
	encapType, _ := srcNode.GetFieldString("EncapType")
	layerType, _ := flow.GetFirstLayerType(encapType)

	srcIP := getIP(pp.SrcIP)
	if srcIP == nil {
		return nil, errors.New("Source Node doesn't have proper IP")
	}

	dstIP := getIP(pp.DstIP)
	if dstIP == nil {
		return nil, errors.New("Destination Node doesn't have proper IP")
	}

	srcMAC, err := net.ParseMAC(pp.SrcMAC)
	if err != nil || srcMAC == nil {
		return nil, errors.New("Source Node doesn't have proper MAC")
	}

	dstMAC, err := net.ParseMAC(pp.DstMAC)
	if err != nil || dstMAC == nil {
		return nil, errors.New("Destination Node doesn't have proper MAC")
	}

	return &ForgedPacketGenerator{
		PacketInjectionParams: pp,
		srcIP:     srcIP,
		dstIP:     dstIP,
		srcMAC:    srcMAC,
		dstMAC:    dstMAC,
		layerType: layerType,
	}, nil
}
