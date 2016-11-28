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
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/logging"
)

var ErrFlowProtocol = errors.New("FlowProtocol invalid")
var ErrFieldNotFound = errors.New("Flow field not found")

type GetAttr interface {
	GetAttr(name string) interface{}
}

type FlowProbeNodeSetter interface {
	SetProbeNode(flow *Flow) bool
}

func (s *FlowLayer) MarshalJSON() ([]byte, error) {
	obj := &struct {
		Protocol string
		A        string
		B        string
	}{
		Protocol: s.Protocol.String(),
		A:        s.A,
		B:        s.B,
	}

	return json.Marshal(&obj)
}

func (s *FlowLayer) UnmarshalJSON(b []byte) error {
	m := struct {
		Protocol string
		A        string
		B        string
	}{}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	protocol, ok := FlowProtocol_value[m.Protocol]
	if !ok {
		return ErrFlowProtocol
	}
	s.Protocol = FlowProtocol(protocol)
	s.A = m.A
	s.B = m.B

	return nil
}

func layerFlow(l gopacket.Layer) gopacket.Flow {
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

type FlowKey string

func (f FlowKey) String() string {
	return string(f)
}

func FlowKeyFromGoPacket(p *gopacket.Packet, parentUUID string) FlowKey {
	network := layerFlow((*p).NetworkLayer()).FastHash()
	transport := layerFlow((*p).TransportLayer()).FastHash()

	return FlowKey(parentUUID + strconv.FormatUint(uint64(network^transport), 10))
}

func layerPathFromGoPacket(packet *gopacket.Packet) string {
	path := ""
	for i, layer := range (*packet).Layers() {
		if i > 0 {
			path += "/"
		}
		path += layer.LayerType().String()
	}
	return path
}

func (flow *Flow) UpdateUUIDs(key string, parentUUID string) {
	hasher := sha1.New()
	hasher.Write([]byte(flow.LayersPath))

	hasher.Write(flow.Link.Hash())
	hasher.Write(flow.Network.Hash())
	hasher.Write(flow.Transport.Hash())
	flow.TrackingID = hex.EncodeToString(hasher.Sum(nil))

	bfStart := make([]byte, 8)
	binary.BigEndian.PutUint64(bfStart, uint64(flow.Metric.Start))
	hasher.Write(bfStart)
	hasher.Write([]byte(flow.NodeUUID))

	// include key so that we are sure that two flows with different keys don't
	// give the same UUID due to different ways of hash the headers.
	hasher.Write([]byte(key))

	flow.UUID = hex.EncodeToString(hasher.Sum(nil))
	flow.ParentUUID = parentUUID
}

func (flow *Flow) initFromGoPacket(key string, now int64, packet *gopacket.Packet, length int64, setter FlowProbeNodeSetter, parentUUID string) {
	flow.Init(now, packet, length)
	if setter != nil {
		setter.SetProbeNode(flow)
	}
	flow.LayersPath = layerPathFromGoPacket(packet)
	appLayers := strings.Split(strings.TrimSuffix(flow.LayersPath, "/Payload"), "/")
	flow.Application = appLayers[len(appLayers)-1]

	flow.UpdateUUIDs(key, parentUUID)
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

func FlowsFromGoPacket(ft *Table, packet *gopacket.Packet, length int64, setter FlowProbeNodeSetter) []*Flow {
	if (*packet).Layer(gopacket.LayerTypeDecodeFailure) != nil {
		logging.GetLogger().Errorf("Decoding failure on layerpath %s", layerPathFromGoPacket(packet))
		logging.GetLogger().Debug((*packet).Dump())
		return nil
	}
	var packetsFlows []*gopacket.Packet

	packetData := (*packet).Data()
	packetLayers := (*packet).Layers()

	var start int
	var innerLen int
	var firstLayer = packetLayers[0]

	for i, layer := range packetLayers {
		innerLen += len(layer.LayerContents())
		layerType := layer.LayerType()

		switch layerType {
		case layers.LayerTypeGRE, layers.LayerTypeVXLAN, layers.LayerTypeMPLS:
			p := gopacket.NewPacket(packetData[start:start+innerLen], firstLayer.LayerType(), gopacket.NoCopy)
			packetsFlows = append(packetsFlows, &p)
			start = start + innerLen
			innerLen = 0
			if i+1 < len(packetLayers)-1 {
				firstLayer = packetLayers[i+1]
			}
		}
	}

	if len(packetsFlows) > 0 {
		p := gopacket.NewPacket(packetData[start:], firstLayer.LayerType(), gopacket.NoCopy)
		packetsFlows = append(packetsFlows, &p)
	} else {
		packetsFlows = append(packetsFlows, packet)
	}

	flows := make([]*Flow, len(packetsFlows))
	var parentUUID string
	for i, p := range packetsFlows {
		flows[i] = flowFromGoPacket(ft, p, length, setter, parentUUID)
		if flows[i] == nil {
			return nil
		}
		parentUUID = flows[i].UUID
	}

	return flows

}

func flowFromGoPacket(ft *Table, packet *gopacket.Packet, length int64, setter FlowProbeNodeSetter, parentUUID ...string) *Flow {
	pUUID := ""
	if len(parentUUID) > 0 {
		pUUID = parentUUID[0]
	}

	key := FlowKeyFromGoPacket(packet, pUUID).String()
	flow, new := ft.GetOrCreateFlow(key)
	if new {
		flow.initFromGoPacket(key, ft.GetTime(), packet, length, setter, pUUID)
	} else {
		flow.Update(ft.GetTime(), packet, length)
	}

	return flow
}

func FlowsFromSFlowSample(ft *Table, sample *layers.SFlowFlowSample, setter FlowProbeNodeSetter) []*Flow {
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

		goFlows := FlowsFromGoPacket(ft, &record.Header, int64(record.FrameLength), setter)
		if goFlows != nil {
			flows = append(flows, goFlows...)
		}
	}

	return flows
}

func (f *FlowLayer) GetField(fields []string) (string, error) {
	if f == nil || len(fields) != 2 {
		return "", ErrFieldNotFound
	}

	/* Protocol must be set on the Layer or the transport layer name like Link, Network, Transport */
	switch fields[0] {
	case "Link", "Network", "Transport":
	default:
		return "", ErrFieldNotFound
	}

	switch fields[1] {
	case "A":
		return f.A, nil
	case "B":
		return f.B, nil
	case "Protocol":
		return f.Protocol.String(), nil
	}
	return "", ErrFieldNotFound
}

func (f *FlowMetric) GetField(fields []string) (int64, error) {
	if len(fields) != 2 {
		return 0, ErrFieldNotFound
	}
	switch fields[1] {
	case "Start":
		return f.Start, nil
	case "Last":
		return f.Last, nil
	case "ABPackets":
		return f.ABPackets, nil
	case "ABBytes":
		return f.ABBytes, nil
	case "BAPackets":
		return f.BAPackets, nil
	case "BABytes":
		return f.BABytes, nil
	}
	return 0, ErrFieldNotFound
}

func (f *Flow) GetFieldString(field string) (string, error) {
	fields := strings.Split(field, ".")
	if len(fields) < 1 {
		return "", ErrFieldNotFound
	}
	name := fields[0]
	switch name {
	case "UUID":
		return f.UUID, nil
	case "LayersPath":
		return f.LayersPath, nil
	case "TrackingID":
		return f.TrackingID, nil
	case "ParentUUID":
		return f.ParentUUID, nil
	case "NodeUUID":
		return f.NodeUUID, nil
	case "ANodeUUID":
		return f.ANodeUUID, nil
	case "BNodeUUID":
		return f.BNodeUUID, nil
	case "Link":
		return f.Link.GetField(fields)
	case "Network":
		return f.Network.GetField(fields)
	case "Transport":
		return f.Transport.GetField(fields)
	case "UDPPORT", "TCPPORT", "SCTPPORT":
		return f.Transport.GetField(fields)
	case "IPV4", "IPV6":
		return f.Network.GetField(fields)
	case "ETHERNET":
		return f.Link.GetField(fields)
	case "Application":
		return f.Application, nil
	}
	return "", ErrFieldNotFound
}

func (f *Flow) GetFieldInt64(field string) (int64, error) {
	fields := strings.Split(field, ".")
	if len(fields) < 1 {
		return 0, ErrFieldNotFound
	}
	name := fields[0]
	switch name {
	case "Metric":
		return f.Metric.GetField(fields)
	case "LastUpdateMetric":
		return f.LastUpdateMetric.GetField(fields)
	}
	return 0, ErrFieldNotFound
}
