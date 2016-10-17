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

func decodeInGRELayer(data []byte, p gopacket.PacketBuilder) error {
	switch (data[0] >> 4) & 0xf {
	case 4:
		ip4 := &layers.IPv4{}
		err := ip4.DecodeFromBytes(data, p)
		p.AddLayer(ip4)
		p.SetNetworkLayer(ip4)
		if err != nil {
			return err
		}
		return p.NextDecoder(ip4.NextLayerType())
	case 6:
		ip6 := &layers.IPv6{}
		err := ip6.DecodeFromBytes(data, p)
		p.AddLayer(ip6)
		p.SetNetworkLayer(ip6)
		if err != nil {
			return err
		}
		return p.NextDecoder(ip6.NextLayerType())
	}
	packet := gopacket.NewPacket(data, layers.LayerTypeARP, gopacket.Lazy)
	layer := packet.Layer(layers.LayerTypeARP)

	p.AddLayer(layer)

	return nil
}
