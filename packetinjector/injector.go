//go:generate go run github.com/skydive-project/skydive/scripts/gendecoder
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"encoding/json"
	"fmt"
	"net"

	"github.com/google/gopacket"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/api/rest"
)

// PacketInjectionRequest describes the packet parameters to be injected
// easyjson:json
type PacketInjectionRequest struct {
	rest.BasicResource
	SrcIP            net.IP           `valid:"isIP"`
	SrcMAC           net.HardwareAddr `valid:"isMAC"`
	SrcPort          uint16
	DstIP            net.IP           `valid:"isIP"`
	DstMAC           net.HardwareAddr `valid:"isMAC"`
	DstPort          uint16
	Type             string `valid:"regexp=^(icmp4|icmp6|tcp4|tcp6|udp4|udp6)$"`
	Count            uint64 `valid:"min=1"`
	ICMPID           uint16
	Interval         uint64
	Mode             string
	IncrementPayload int64
	Payload          string
	Pcap             []byte
	TTL              uint8
}

// GetName returns the name of the resource
func (r *PacketInjectionRequest) GetName() string {
	return "PacketInjection"
}

// Injections holds the injections metadata
// easyjson:json
// gendecoder
type Injections []*InjectionMetadata

// InjectionMetadata holds attributes and statistics about an injection
// easyjson:json
// gendecoder
type InjectionMetadata struct {
	PacketInjectionRequest
	ID          string
	State       string
	PacketCount int64
}

// InjectionsMetadataDecoder implements a json message raw decoder
func InjectionsMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var injections Injections
	if err := json.Unmarshal(raw, &injections); err != nil {
		return nil, fmt.Errorf("unable to unmarshal injections metadata %s: %s", string(raw), err)
	}

	return &injections, nil
}

// Packet is defined as a gopacket and it byte representation
type Packet struct {
	data     []byte
	gopacket gopacket.Packet
}

// Data returns the packet payload as an array of byte
func (p *Packet) Data() []byte {
	return p.data
}

// PacketForger describes an objects that feeds a channel with packets
type PacketForger interface {
	PacketSource() chan *Packet
	Close()
}
