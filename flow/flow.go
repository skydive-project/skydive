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
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/redhat-cip/skydive/logging"
)

type FlowProbePathSetter interface {
	SetProbePath(flow *Flow) bool
}

func (s *FlowEndpointsStatistics) MarshalJSON() ([]byte, error) {
	obj := &struct {
		Type string
		AB   *FlowEndpointStatistics
		BA   *FlowEndpointStatistics
	}{
		Type: s.Type.String(),
		AB:   s.AB,
		BA:   s.BA,
	}

	return json.Marshal(&obj)
}

func (s *FlowEndpointsStatistics) UnmarshalJSON(b []byte) error {
	m := struct {
		Type string
		AB   *FlowEndpointStatistics
		BA   *FlowEndpointStatistics
	}{}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	s.Type = FlowEndpointType(FlowEndpointType_value[m.Type])
	s.AB = m.AB
	s.BA = m.BA

	return nil
}

func Var8bin(v []byte) []byte {
	r := make([]byte, 8)
	skip := 8 - len(v)
	for i, b := range v {
		r[i+skip] = b
	}
	return r
}

func (eps *FlowEndpointsStatistics) hash(ab interface{}, ba interface{}) {
	var vab, vba uint64
	var binab, binba []byte

	hasher := sha1.New()
	switch ab.(type) {
	case net.HardwareAddr:
		binab = ab.(net.HardwareAddr)
		binba = ba.(net.HardwareAddr)
		vab = binary.BigEndian.Uint64(Var8bin(binab))
		vba = binary.BigEndian.Uint64(Var8bin(binba))
	case net.IP:
		binab = ab.(net.IP)
		binba = ba.(net.IP)
		vab = binary.BigEndian.Uint64(Var8bin(binab))
		vba = binary.BigEndian.Uint64(Var8bin(binba))
	case layers.TCPPort:
		binab = make([]byte, 2)
		binba = make([]byte, 2)
		binary.BigEndian.PutUint16(binab, uint16(ab.(layers.TCPPort)))
		binary.BigEndian.PutUint16(binba, uint16(ba.(layers.TCPPort)))
		vab = uint64(ab.(layers.TCPPort))
		vba = uint64(ba.(layers.TCPPort))
	case layers.UDPPort:
		binab = make([]byte, 2)
		binba = make([]byte, 2)
		binary.BigEndian.PutUint16(binab, uint16(ab.(layers.UDPPort)))
		binary.BigEndian.PutUint16(binba, uint16(ba.(layers.UDPPort)))
		vab = uint64(ab.(layers.UDPPort))
		vba = uint64(ba.(layers.UDPPort))
	case layers.SCTPPort:
		binab = make([]byte, 2)
		binba = make([]byte, 2)
		binary.BigEndian.PutUint16(binab, uint16(ab.(layers.SCTPPort)))
		binary.BigEndian.PutUint16(binba, uint16(ba.(layers.SCTPPort)))
		vab = uint64(ab.(layers.SCTPPort))
		vba = uint64(ba.(layers.SCTPPort))
	}
	if vab < vba {
		hasher.Write(binab)
		hasher.Write(binba)
	} else {
		hasher.Write(binba)
		hasher.Write(binab)
	}
	eps.Hash = hasher.Sum(nil)
}

func LayerFlow(l gopacket.Layer) gopacket.Flow {
	switch l.(type) {
	case gopacket.LinkLayer:
		return l.(gopacket.LinkLayer).LinkFlow()
	case gopacket.NetworkLayer:
		return l.(gopacket.NetworkLayer).NetworkFlow()
	case gopacket.TransportLayer:
		return l.(gopacket.TransportLayer).TransportFlow()
	}
	return gopacket.Flow{}
}

type FlowKey struct {
	net, transport uint64
}

func NewFlowKeyFromGoPacket(p *gopacket.Packet) *FlowKey {
	return &FlowKey{
		net:       LayerFlow((*p).NetworkLayer()).FastHash(),
		transport: LayerFlow((*p).TransportLayer()).FastHash(),
	}
}

func (key FlowKey) String() string {
	return fmt.Sprintf("%x-%x", key.net, key.transport)
}

func (flow *Flow) fillFromGoPacket(packet *gopacket.Packet) error {
	/* Continue if no ethernet layer */
	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	_, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return errors.New("Unable to decode the ethernet layer")
	}

	newFlow := false
	fs := flow.GetStatistics()
	now := time.Now().Unix() //(*packet).Metadata().Timestamp.Unix()
	if fs == nil {
		newFlow = true
		fs = NewFlowStatistics(packet)
		fs.Start = now
		flow.Statistics = fs
	}
	fs.Last = now
	fs.Update(packet)

	if newFlow {
		hasher := sha1.New()
		path := ""
		for i, layer := range (*packet).Layers() {
			if i > 0 {
				path += "/"
			}
			path += layer.LayerType().String()
		}
		flow.LayersPath = path
		hasher.Write([]byte(flow.LayersPath))

		/* Generate an flow UUID */
		for _, ep := range fs.GetEndpoints() {
			hasher.Write(ep.Hash)
		}
		flow.TrackingID = hex.EncodeToString(hasher.Sum(nil))

		bfStart := make([]byte, 8)
		binary.BigEndian.PutUint64(bfStart, uint64(fs.Start))
		hasher.Write(bfStart)
		hasher.Write([]byte(flow.ProbeGraphPath))
		flow.UUID = hex.EncodeToString(hasher.Sum(nil))
	}
	return nil
}

func FromData(data []byte) (*Flow, error) {
	flow := new(Flow)

	err := proto.Unmarshal(data, flow)
	if err != nil {
		return nil, err
	}

	return flow, nil
}

func (flow *Flow) GetData() ([]byte, error) {
	data, err := proto.Marshal(flow)
	if err != nil {
		return []byte{}, err
	}

	return data, nil
}

func FlowFromGoPacket(ft *Table, packet *gopacket.Packet, setter FlowProbePathSetter) *Flow {
	key := NewFlowKeyFromGoPacket(packet)
	flow, _ := ft.GetOrCreateFlow(key.String())
	if setter != nil {
		setter.SetProbePath(flow)
	}

	err := flow.fillFromGoPacket(packet)
	if err != nil {
		logging.GetLogger().Error(err.Error())
		return nil
	}

	return flow
}

func FlowsFromSFlowSample(ft *Table, sample *layers.SFlowFlowSample, setter FlowProbePathSetter) []*Flow {
	flows := []*Flow{}

	for _, rec := range sample.Records {

		/* FIX(safchain): just keeping the raw packet for now */
		switch rec.(type) {
		case layers.SFlowRawPacketFlowRecord:
			/* We only support RawPacket from SFlow probe */
		case layers.SFlowExtendedSwitchFlowRecord:
			logging.GetLogger().Debug("1st layer is not SFlowRawPacketFlowRecord type")
			continue
		default:
			logging.GetLogger().Critical("1st layer is not a SFlow supported type")
			continue
		}

		record := rec.(layers.SFlowRawPacketFlowRecord)

		flow := FlowFromGoPacket(ft, &record.Header, setter)
		if flow != nil {
			flows = append(flows, flow)
		}
	}

	return flows
}
