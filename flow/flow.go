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
	"math"
	"reflect"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

type FlowProbeNodeSetter interface {
	SetProbeNode(flow *Flow) bool
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

func FlowKeyFromGoPacket(p *gopacket.Packet) FlowKey {
	network := layerFlow((*p).NetworkLayer()).FastHash()
	transport := layerFlow((*p).TransportLayer()).FastHash()

	return FlowKey(strconv.FormatUint(uint64(network^transport), 10))
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

func (flow *Flow) UpdateUUIDs(key string) {
	fs := flow.GetStatistics()

	hasher := sha1.New()
	hasher.Write([]byte(flow.LayersPath))

	for _, ep := range fs.GetEndpoints() {
		hasher.Write(ep.Hash)
	}
	flow.TrackingID = hex.EncodeToString(hasher.Sum(nil))

	bfStart := make([]byte, 8)
	binary.BigEndian.PutUint64(bfStart, uint64(fs.Start))
	hasher.Write(bfStart)
	hasher.Write([]byte(flow.ProbeNodeUUID))

	// include key so that we are sure that two flows with different keys don't
	// give the same UUID due to different ways of hash the headers.
	hasher.Write([]byte(key))

	flow.UUID = hex.EncodeToString(hasher.Sum(nil))
}

func (flow *Flow) initFromGoPacket(key string, now int64, packet *gopacket.Packet, length uint64, setter FlowProbeNodeSetter) {
	flow.Statistics.Init(now, packet, length)

	if setter != nil {
		setter.SetProbeNode(flow)
	}

	flow.LayersPath = layerPathFromGoPacket(packet)

	flow.UpdateUUIDs(key)
}

func FromData(data []byte) (*Flow, error) {
	flow := new(Flow)

	err := proto.Unmarshal(data, flow)
	if err != nil {
		return nil, err
	}

	return flow, nil
}

func GetAttribute(intf interface{}, name string) interface{} {
	if getter, ok := intf.(GetAttr); ok {
		return getter.GetAttr(name)
	}
	value := reflect.Indirect(reflect.ValueOf(intf))
	field := value.FieldByName(name)
	if !field.IsValid() {
		return nil
	}
	return field.Interface()
}

func GetFields(intf interface{}, fields []string) interface{} {
componentLoop:
	for _, component := range fields {
		value := reflect.Indirect(reflect.ValueOf(intf))
		if value.Kind() == reflect.Slice {
			for i := 0; i < value.Len(); i++ {
				if intf = value.Index(i).Interface(); GetAttribute(intf, component) != nil {
					continue componentLoop
				}
			}
			return nil
		} else {
			intf = GetAttribute(intf, component)
		}

		if intf == nil {
			return nil
		}

		if component == "Last" && common.CrossTypeEqual(intf, 0) {
			return math.MaxUint32
		}
	}
	return intf
}

func (flow *Flow) GetData() ([]byte, error) {
	data, err := proto.Marshal(flow)
	if err != nil {
		return []byte{}, err
	}

	return data, nil
}

func (flow *Flow) GetLayerHash(ltype FlowEndpointType) string {
	s := flow.GetStatistics()
	if s == nil {
		return ""
	}
	return hex.EncodeToString(s.GetLayerHash(ltype))
}

func FlowFromGoPacket(ft *Table, packet *gopacket.Packet, length uint64, setter FlowProbeNodeSetter) *Flow {
	if el := (*packet).Layer(layers.LayerTypeEthernet); el == nil {
		logging.GetLogger().Error("Unable to decode the ethernet layer")
		return nil
	}

	key := FlowKeyFromGoPacket(packet).String()
	flow, new := ft.GetOrCreateFlow(key)
	if new {
		flow.initFromGoPacket(key, ft.GetTime(), packet, length, setter)
	} else {
		flow.GetStatistics().Update(ft.GetTime(), packet, length)
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

		flow := FlowFromGoPacket(ft, &record.Header, uint64(record.FrameLength), setter)
		if flow != nil {
			flows = append(flows, flow)
		}
	}

	return flows
}

func (fes *FlowEndpointsStatistics) GetAttr(name string) interface{} {
	flowType, ok := FlowEndpointType_value[name]
	if ok && fes.Type == FlowEndpointType(flowType) {
		return fes.Type
	}

	value := reflect.Indirect(reflect.ValueOf(fes))
	field := value.FieldByName(name)
	if !field.IsValid() {
		return nil
	}
	return field.Interface()
}
