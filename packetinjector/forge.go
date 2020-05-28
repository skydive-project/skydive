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
	"fmt"
	"math"
	"math/rand"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/logging"
)

var (
	options = gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
)

// ForgedPacketGenerator is used to forge packets. It creates
// a gopacket.Packet with the proper layers that can be directly
// inserted into a socket.
type ForgedPacketGenerator struct {
	*PacketInjectionRequest
	LayerType gopacket.LayerType
	close     chan bool
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// randString generates random string
func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func forgePacket(packetType string, layerType gopacket.LayerType, srcMAC, dstMAC net.HardwareAddr, TTL uint8, srcIP, dstIP net.IP, srcPort, dstPort uint16, ID uint16, data string) ([]byte, gopacket.Packet, error) {
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
			Id:       ID,
		}
		l = append(l, ipLayer, icmpLayer)
	case "icmp6":
		ipLayer := &layers.IPv6{Version: 6, SrcIP: srcIP, DstIP: dstIP, NextHeader: layers.IPProtocolICMPv6}
		icmpLayer := &layers.ICMPv6{
			TypeCode:  layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0),
			TypeBytes: []byte{byte(ID & uint16(0xFF00) >> 8), byte(ID & uint16(0xFF)), 0, 0},
		}
		icmpLayer.SetNetworkLayerForChecksum(ipLayer)

		echoLayer := &layers.ICMPv6Echo{
			Identifier: ID,
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

// Close the packet generator
func (f *ForgedPacketGenerator) Close() {
	f.close <- true
}

// PacketSource returns a channel when forged packets are pushed
func (f *ForgedPacketGenerator) PacketSource() chan *Packet {
	ch := make(chan *Packet)

	go func() {
		defer close(ch)

		payload := f.Payload
		// use same size as ping when no payload specified
		if len(payload) == 0 {
			payload = randString(56)
		}

		if f.Count == 0 {
			f.Count = 1
		}

		id := f.ICMPID
		srcPort := f.SrcPort

		for i := uint64(0); i < f.Count; i++ {
			packetData, packet, err := forgePacket(f.Type, f.LayerType, f.SrcMAC, f.DstMAC, f.TTL, f.SrcIP, f.DstIP, srcPort, f.DstPort, id, payload)
			if err != nil {
				logging.GetLogger().Error(err)
				return
			}

			if f.IncrementPayload > 0 {
				payload = payload + randString(int(f.IncrementPayload))
			}

			select {
			case <-f.close:
				return
			case ch <- &Packet{data: packetData, gopacket: packet}:
			}

			if i != f.Count-1 && f.Interval != 0 {
				select {
				case <-f.close:
					return
				case <-time.After(time.Millisecond * time.Duration(f.Interval)):
				}
			}

			if f.Mode == types.PIModeRandom {
				switch f.Type {
				case types.PITypeICMP4, types.PITypeICMP6:
					id = uint16(rand.Intn(math.MaxUint16-1) + 1)
				default:
					srcPort = uint16(rand.Intn(math.MaxUint16-1) + 1)
				}
			}
		}
		ch <- nil
	}()

	return ch
}

// NewForgedPacketGenerator returns a new generator of forged packets
func NewForgedPacketGenerator(pp *PacketInjectionRequest, encapType string) (*ForgedPacketGenerator, error) {
	layerType, _ := flow.GetFirstLayerType(encapType)

	return &ForgedPacketGenerator{
		PacketInjectionRequest: pp,
		LayerType:              layerType,
		close:                  make(chan bool, 1),
	}, nil
}
