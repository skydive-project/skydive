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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/redhat-cip/skydive/logging"
)

func (s *FlowStatistics) MarshalJSON() ([]byte, error) {
	obj := &struct {
		Start     int64
		Last      int64
		Endpoints []*FlowEndpointsStatistics
	}{
		Start:     s.Start,
		Last:      s.Last,
		Endpoints: make([]*FlowEndpointsStatistics, len(s.Endpoints), len(s.Endpoints)),
	}

	i := 0
	for _, e := range s.Endpoints {
		obj.Endpoints[i] = e
		i++
	}

	return json.Marshal(&obj)
}

func (s *FlowEndpointStatistics) MarshalJSON() ([]byte, error) {
	obj := &struct {
		Value   string
		Packets uint64
		Bytes   uint64
	}{
		Value:   s.Value,
		Packets: s.Packets,
		Bytes:   s.Bytes,
	}

	return json.Marshal(&obj)
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

func (s *FlowEndpointStatistics) UnmarshalJSON(b []byte) error {
	m := struct {
		Value   string
		Packets uint64
		Bytes   uint64
	}{}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	s.Value = m.Value
	s.Packets = m.Packets
	s.Bytes = m.Bytes

	return nil
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

func (s *FlowStatistics) UnmarshalJSON(b []byte) error {
	m := struct {
		Start     int64
		Last      int64
		Endpoints []*FlowEndpointsStatistics
	}{}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	s.Start = m.Start
	s.Last = m.Last

	s.Endpoints = make(map[int32]*FlowEndpointsStatistics)
	for _, e := range m.Endpoints {
		s.Endpoints[e.Type.Value()] = e
	}

	return nil
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

func (flow *Flow) ID() string {
	id := flow.UUID

	// FIX(safchain) should not be empty, will be removed when having sync mapping
	if flow.ProbeGraphPath != "" {
		id += ":" + flow.ProbeGraphPath
	}

	return id
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
		fs = NewFlowStatistics()
		fs.Start = now
		fs.newEthernetEndpointStatistics(packet)
		fs.newIPV4EndpointStatistics(packet)
		fs.newTransportEndpointStatistics(packet)
		flow.Statistics = fs
	}
	fs.Last = now
	fs.updateEthernetFromGoPacket(packet)
	fs.updateIPV4FromGoPacket(packet)
	fs.updateTransportFromGoPacket(packet)

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
			hasher.Write([]byte(ep.AB.Value))
			hasher.Write([]byte(ep.BA.Value))
		}
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

func FlowFromGoPacket(ft *FlowTable, packet *gopacket.Packet, probePath string) *Flow {
	key := NewFlowKeyFromGoPacket(packet)
	flow, _ := ft.GetOrCreateFlow(key.String())
	flow.ProbeGraphPath = probePath
	flow.fillFromGoPacket(packet)
	return flow
}

func FlowsFromSFlowSample(ft *FlowTable, sample *layers.SFlowFlowSample, probePath string) []*Flow {
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

		flow := FlowFromGoPacket(ft, &record.Header, probePath)
		flows = append(flows, flow)
	}

	return flows
}
