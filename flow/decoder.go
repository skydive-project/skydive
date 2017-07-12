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

package flow

import (
	"encoding/binary"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// LayerTypeInGRE creates a layer type, should be unique and high, so it doesn't conflict,
// giving it a name and a decoder to use.
var LayerTypeInGRE = gopacket.RegisterLayerType(55555, gopacket.LayerTypeMetadata{Name: "LayerTypeInGRE", Decoder: gopacket.DecodeFunc(decodeInGRELayer)})

// Try to find if the next layer is IPv4, or IPv6. If it fails, it considers it is Ethernet.
var layerTypeInMplsEthOrIP = gopacket.RegisterLayerType(55556, gopacket.LayerTypeMetadata{Name: "LayerTypeInMplsEthOrIp", Decoder: gopacket.DecodeFunc(decodeInMplsEthOrIPLayer)})

var layerTypeICMPv4 = gopacket.OverrideLayerType(19, gopacket.LayerTypeMetadata{Name: "ICMPv4", Decoder: gopacket.DecodeFunc(decodeICMPv4)})
var layerTypeICMPv6 = gopacket.OverrideLayerType(57, gopacket.LayerTypeMetadata{Name: "ICMPv6", Decoder: gopacket.DecodeFunc(decodeICMPv6)})

type inGRELayer struct {
	StrangeHeader []byte
	payload       []byte
}

func (m inGRELayer) LayerType() gopacket.LayerType {
	return LayerTypeInGRE
}

func (m inGRELayer) LayerContents() []byte {
	return m.StrangeHeader
}

func (m inGRELayer) LayerPayload() []byte {
	return m.payload
}

// ICMPv4 aims to store ICMP metadata and aims to be used for the flow hash key
type ICMPv4 struct {
	layers.ICMPv4
	Type ICMPType
}

// Payload returns the ICMP payload
func (i *ICMPv4) Payload() []byte { return i.LayerPayload() }

// ICMPv6 aims to store ICMP metadata and aims to be used for the flow hash key
type ICMPv6 struct {
	layers.ICMPv6
	Type ICMPType
	Id   uint16
}

// Payload returns the ICMP payload
func (i *ICMPv6) Payload() []byte { return i.LayerPayload() }

// Try to decode data as IP4 or IP6. If data starts by 4 or 6,
// ipPrefix is set to true to indicate it seems to be an IP header,
// and a decoding failure would be reported in error.
func ipDecoderFromRawData(data []byte, p gopacket.PacketBuilder) (ipPrefix bool, e error) {
	switch (data[0] >> 4) & 0xf {
	case 4:
		ip4 := &layers.IPv4{}
		err := ip4.DecodeFromBytes(data, p)
		p.AddLayer(ip4)

		// Only the first call to this function is kept by
		// gopacket. So, this works even if this layer is not
		// the network layer (in case of encapsulation).
		p.SetNetworkLayer(ip4)
		if err != nil {
			return true, err
		}
		return true, p.NextDecoder(ip4.NextLayerType())
	case 6:
		ip6 := &layers.IPv6{}
		err := ip6.DecodeFromBytes(data, p)
		p.AddLayer(ip6)
		p.SetNetworkLayer(ip6)
		if err != nil {
			return true, err
		}
		return true, p.NextDecoder(ip6.NextLayerType())
	default:
		return false, nil
	}
}

func decodeInGRELayer(data []byte, p gopacket.PacketBuilder) error {
	if ipPrefix, err := ipDecoderFromRawData(data, p); ipPrefix {
		return err
	}
	packet := gopacket.NewPacket(data, layers.LayerTypeARP, gopacket.Lazy)
	layer := packet.Layer(layers.LayerTypeARP)
	p.AddLayer(layer)
	return nil
}

func decodeInMplsEthOrIPLayer(data []byte, p gopacket.PacketBuilder) error {
	if ipPrefix, err := ipDecoderFromRawData(data, p); ipPrefix && err == nil {
		return nil
	}
	// If IPv4 or IPv6 fails, we fallback to Ethernet
	eth := &layers.Ethernet{}
	err := eth.DecodeFromBytes(data, p)
	p.AddLayer(eth)
	if err != nil {
		return err
	}
	return p.NextDecoder(eth.NextLayerType())
}

func decodeICMPv4(data []byte, p gopacket.PacketBuilder) error {
	icmpv4 := &ICMPv4{}
	err := icmpv4.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}

	switch icmpv4.TypeCode.Type() {
	case layers.ICMPv4TypeEchoRequest, layers.ICMPv4TypeEchoReply:
		icmpv4.Type = ICMPType_ECHO
	case layers.ICMPv4TypeAddressMaskRequest, layers.ICMPv4TypeAddressMaskReply:
		icmpv4.Type = ICMPType_ADDRESS_MASK
	case layers.ICMPv4TypeDestinationUnreachable:
		icmpv4.Type = ICMPType_DESTINATION_UNREACHABLE
	case layers.ICMPv4TypeInfoRequest, layers.ICMPv4TypeInfoReply:
		icmpv4.Type = ICMPType_INFO
	case layers.ICMPv4TypeParameterProblem:
		icmpv4.Type = ICMPType_PARAMETER_PROBLEM
	case layers.ICMPv4TypeRedirect:
		icmpv4.Type = ICMPType_REDIRECT
	case layers.ICMPv4TypeRouterSolicitation, layers.ICMPv4TypeRouterAdvertisement:
		icmpv4.Type = ICMPType_ROUTER
	case layers.ICMPv4TypeSourceQuench:
		icmpv4.Type = ICMPType_SOURCE_QUENCH
	case layers.ICMPv4TypeTimeExceeded:
		icmpv4.Type = ICMPType_TIME_EXCEEDED
	case layers.ICMPv4TypeTimestampRequest, layers.ICMPv4TypeTimestampReply:
		icmpv4.Type = ICMPType_TIMESTAMP
	}

	p.AddLayer(icmpv4)
	p.SetApplicationLayer(icmpv4)
	return p.NextDecoder(icmpv4.NextLayerType())
}

func decodeICMPv6(data []byte, p gopacket.PacketBuilder) error {
	icmpv6 := &ICMPv6{}
	err := icmpv6.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}

	switch icmpv6.TypeCode.Type() {
	case layers.ICMPv6TypeEchoRequest, layers.ICMPv6TypeEchoReply:
		icmpv6.Type = ICMPType_ECHO
		icmpv6.Id = binary.BigEndian.Uint16(icmpv6.TypeBytes[0:2])
	case layers.ICMPv6TypeNeighborSolicitation, layers.ICMPv6TypeNeighborAdvertisement:
		icmpv6.Type = ICMPType_NEIGHBOR
	case layers.ICMPv6TypeDestinationUnreachable:
		icmpv6.Type = ICMPType_DESTINATION_UNREACHABLE
	case layers.ICMPv6TypePacketTooBig:
		icmpv6.Type = ICMPType_PACKET_TOO_BIG
	case layers.ICMPv6TypeParameterProblem:
		icmpv6.Type = ICMPType_PARAMETER_PROBLEM
	case layers.ICMPv6TypeRedirect:
		icmpv6.Type = ICMPType_REDIRECT
	case layers.ICMPv6TypeRouterSolicitation, layers.ICMPv6TypeRouterAdvertisement:
		icmpv6.Type = ICMPType_ROUTER
	case layers.ICMPv6TypeTimeExceeded:
		icmpv6.Type = ICMPType_TIME_EXCEEDED
	}

	p.AddLayer(icmpv6)
	p.SetApplicationLayer(icmpv6)
	return p.NextDecoder(icmpv6.NextLayerType())
}

func init() {
	// By default, gopacket tries to decode IPv4 or IPv6 in the
	// MPLS next layer and fails otherwise. Instead, we also tries
	// to decode it as Ethernet.
	layers.MPLSPayloadDecoder = layerTypeInMplsEthOrIP
}
