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
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Create a layer type, should be unique and high, so it doesn't conflict,
// giving it a name and a decoder to use.
var LayerTypeInGRE = gopacket.RegisterLayerType(55555, gopacket.LayerTypeMetadata{Name: "LayerTypeInGRE", Decoder: gopacket.DecodeFunc(decodeInGRELayer)})

// Try to find if the next layer is IPv4, or IPv6. If it fails, it considers it is Ethernet.
var LayerTypeInMplsEthOrIp = gopacket.RegisterLayerType(55556, gopacket.LayerTypeMetadata{Name: "LayerTypeInMplsEthOrIp", Decoder: gopacket.DecodeFunc(decodeInMplsEthOrIpLayer)})

type InGRELayer struct {
	StrangeHeader []byte
	payload       []byte
}

func (m InGRELayer) LayerType() gopacket.LayerType {
	return LayerTypeInGRE
}

func (m InGRELayer) LayerContents() []byte {
	return m.StrangeHeader
}

func (m InGRELayer) LayerPayload() []byte {
	return m.payload
}

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
	} else {
		packet := gopacket.NewPacket(data, layers.LayerTypeARP, gopacket.Lazy)
		layer := packet.Layer(layers.LayerTypeARP)
		p.AddLayer(layer)
		return nil
	}
}

func decodeInMplsEthOrIpLayer(data []byte, p gopacket.PacketBuilder) error {
	if ipPrefix, err := ipDecoderFromRawData(data, p); ipPrefix && err == nil {
		return nil
	} else {
		// If IPv4 or IPv6 fails, we fallback to Ethernet
		eth := &layers.Ethernet{}
		err := eth.DecodeFromBytes(data, p)
		p.AddLayer(eth)
		if err != nil {
			return err
		}
		return p.NextDecoder(eth.NextLayerType())
	}
}

func init() {
	// By default, gopacket tries to decode IPv4 or IPv6 in the
	// MPLS next layer and fails otherwise. Instead, we also tries
	// to decode it as Ethernet.
	layers.MPLSPayloadDecoder = LayerTypeInMplsEthOrIp
}
