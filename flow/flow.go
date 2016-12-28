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
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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

type FlowPacket struct {
	gopacket *gopacket.Packet
	length   int64
}

// FlowPackets represents a suite of parent/child FlowPacket
type FlowPackets []FlowPacket

func (x FlowProtocol) Value() int32 {
	return int32(x)
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
	return strings.Replace(path, "Linux SLL/", "", 1)
}

func (flow *Flow) UpdateUUID(key string) {
	hasher := sha1.New()

	hasher.Write(flow.Transport.Hash())
	hasher.Write(flow.Network.Hash())
	hasher.Write([]byte(strings.TrimPrefix(flow.LayersPath, "Ethernet/")))
	flow.L3TrackingID = hex.EncodeToString(hasher.Sum(nil))

	hasher.Write(flow.Link.Hash())
	hasher.Write([]byte(flow.LayersPath))
	flow.TrackingID = hex.EncodeToString(hasher.Sum(nil))

	bfStart := make([]byte, 8)
	binary.BigEndian.PutUint64(bfStart, uint64(flow.Metric.Start))
	hasher.Write(bfStart)
	hasher.Write([]byte(flow.NodeTID))

	// include key so that we are sure that two flows with different keys don't
	// give the same UUID due to different ways of hash the headers.
	hasher.Write([]byte(key))

	flow.UUID = hex.EncodeToString(hasher.Sum(nil))
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

func (f *Flow) Init(key string, now int64, packet *gopacket.Packet, length int64, nodeTID string, parentUUID string) {
	f.Metric.Start = now
	f.Metric.Last = now

	f.newLinkLayer(packet, length)

	f.NodeTID = nodeTID
	f.ParentUUID = parentUUID

	f.LayersPath = layerPathFromGoPacket(packet)
	appLayers := strings.Split(strings.TrimSuffix(f.LayersPath, "/Payload"), "/")
	f.Application = appLayers[len(appLayers)-1]

	// no network layer then no transport layer
	if err := f.newNetworkLayer(packet); err == nil {
		f.newTransportLayer(packet)
	}

	// need to have as most variable filled as possible to get correct UUID
	f.UpdateUUID(key)
}

func (f *Flow) Update(now int64, packet *gopacket.Packet, length int64) {
	f.Metric.Last = now

	if updated := f.updateMetricsWithLinkLayer(packet, length); !updated {
		f.updateMetricsWithNetworkLayer(packet)
	}
}

func (fm *FlowMetric) Copy() *FlowMetric {
	return &FlowMetric{
		Start:     fm.Start,
		Last:      fm.Last,
		ABPackets: fm.ABPackets,
		ABBytes:   fm.ABBytes,
		BAPackets: fm.BAPackets,
		BABytes:   fm.BABytes,
	}
}

func (f *Flow) DumpInfo(layerSeparator ...string) string {
	fm := f.GetMetric()
	sep := " | "
	if len(layerSeparator) > 0 {
		sep = layerSeparator[0]
	}
	buf := bytes.NewBufferString("")
	buf.WriteString(fmt.Sprintf("%s\t", f.Link.Protocol))
	buf.WriteString(fmt.Sprintf("(%d %d)", fm.ABPackets, fm.ABBytes))
	buf.WriteString(fmt.Sprintf(" (%d %d)", fm.BAPackets, fm.BABytes))
	buf.WriteString(fmt.Sprintf("\t(%s -> %s)%s", f.Link.A, f.Link.B, sep))
	return buf.String()
}

func (f *Flow) newLinkLayer(packet *gopacket.Packet, length int64) {
	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		// bypass if a Link layer can't be decoded, i.e. Network layer is the first layer
		return
	}

	f.Link = &FlowLayer{
		Protocol: FlowProtocol_ETHERNET,
		A:        ethernetPacket.SrcMAC.String(),
		B:        ethernetPacket.DstMAC.String(),
	}

	f.updateMetricsWithLinkLayer(packet, length)
}

func getLinkLayerLength(packet *layers.Ethernet) int64 {
	if packet.Length > 0 { // LLC
		return 14 + int64(packet.Length)
	}

	return 14 + int64(len(packet.Payload))
}

func (f *Flow) updateMetricsWithLinkLayer(packet *gopacket.Packet, length int64) bool {
	ethernetLayer := (*packet).Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		// bypass if a Link layer can't be decoded, i.e. Network layer is the first layer
		return false
	}

	// if the length is given use it as the packet can be truncated like in SFlow
	if length == 0 {
		length = getLinkLayerLength(ethernetPacket)
	}

	if f.Link.A == ethernetPacket.SrcMAC.String() {
		f.Metric.ABPackets += int64(1)
		f.Metric.ABBytes += length
	} else {
		f.Metric.BAPackets += int64(1)
		f.Metric.BABytes += length
	}

	return true
}

func (f *Flow) newNetworkLayer(packet *gopacket.Packet) error {
	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		f.Network = &FlowLayer{
			Protocol: FlowProtocol_IPV4,
			A:        ipv4Packet.SrcIP.String(),
			B:        ipv4Packet.DstIP.String(),
		}
		return f.updateMetricsWithNetworkLayer(packet)
	}

	ipv6Layer := (*packet).Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		f.Network = &FlowLayer{
			Protocol: FlowProtocol_IPV6,
			A:        ipv6Packet.SrcIP.String(),
			B:        ipv6Packet.DstIP.String(),
		}
		return f.updateMetricsWithNetworkLayer(packet)
	}

	return errors.New("Unable to decode the IP layer")
}

func (f *Flow) updateMetricsWithNetworkLayer(packet *gopacket.Packet) error {
	// bypass if a Link layer already exist
	if f.Link != nil {
		return nil
	}

	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		if f.Network.A == ipv4Packet.SrcIP.String() {
			f.Metric.ABPackets += int64(1)
			f.Metric.ABBytes += int64(ipv4Packet.Length)
		} else {
			f.Metric.BAPackets += int64(1)
			f.Metric.BABytes += int64(ipv4Packet.Length)
		}
		return nil
	}
	ipv6Layer := (*packet).Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		if f.Network.A == ipv6Packet.SrcIP.String() {
			f.Metric.ABPackets += int64(1)
			f.Metric.ABBytes += int64(ipv6Packet.Length)
		} else {
			f.Metric.BAPackets += int64(1)
			f.Metric.BABytes += int64(ipv6Packet.Length)
		}
		return nil
	}
	return errors.New("Unable to decode the IP layer")
}

func (f *Flow) newTransportLayer(packet *gopacket.Packet) error {
	var transportLayer gopacket.Layer
	var ok bool
	transportLayer = (*packet).Layer(layers.LayerTypeTCP)
	_, ok = transportLayer.(*layers.TCP)
	ptype := FlowProtocol_TCPPORT
	if !ok {
		transportLayer = (*packet).Layer(layers.LayerTypeUDP)
		_, ok = transportLayer.(*layers.UDP)
		ptype = FlowProtocol_UDPPORT
		if !ok {
			transportLayer = (*packet).Layer(layers.LayerTypeSCTP)
			_, ok = transportLayer.(*layers.SCTP)
			ptype = FlowProtocol_SCTPPORT
			if !ok {
				return errors.New("Unable to decode the transport layer")
			}
		}
	}

	f.Transport = &FlowLayer{
		Protocol: ptype,
	}

	switch ptype {
	case FlowProtocol_TCPPORT:
		transportPacket, _ := transportLayer.(*layers.TCP)
		f.Transport.A = strconv.Itoa(int(transportPacket.SrcPort))
		f.Transport.B = strconv.Itoa(int(transportPacket.DstPort))
	case FlowProtocol_UDPPORT:
		transportPacket, _ := transportLayer.(*layers.UDP)
		f.Transport.A = strconv.Itoa(int(transportPacket.SrcPort))
		f.Transport.B = strconv.Itoa(int(transportPacket.DstPort))
	case FlowProtocol_SCTPPORT:
		transportPacket, _ := transportLayer.(*layers.SCTP)
		f.Transport.A = strconv.Itoa(int(transportPacket.SrcPort))
		f.Transport.B = strconv.Itoa(int(transportPacket.DstPort))
	}
	return nil
}

// FlowPacketsFromGoPacket split original packet into multiple packets in
// case of encapsulation like GRE, VXLAN, etc.
func FlowPacketsFromGoPacket(packet *gopacket.Packet, outerLength int64) FlowPackets {
	if (*packet).Layer(gopacket.LayerTypeDecodeFailure) != nil {
		logging.GetLogger().Errorf("Decoding failure on layerpath %s", layerPathFromGoPacket(packet))
		logging.GetLogger().Debug((*packet).Dump())
		return nil
	}

	var flowPackets FlowPackets

	packetData := (*packet).Data()
	packetLayers := (*packet).Layers()

	var topLayer = packetLayers[0]

	if outerLength == 0 {
		if ethernetPacket, ok := topLayer.(*layers.Ethernet); ok {
			outerLength = getLinkLayerLength(ethernetPacket)
		} else if ipv4Packet, ok := topLayer.(*layers.IPv4); ok {
			outerLength = int64(ipv4Packet.Length)
		} else if ipv6Packet, ok := topLayer.(*layers.IPv6); ok {
			outerLength = int64(ipv6Packet.Length)
		}
	}

	// length of the encapsulation header + the inner packet
	topLayerLength := outerLength

	var start int
	var innerLength int
	for i, layer := range packetLayers {
		innerLength += len(layer.LayerContents())

		switch layer.LayerType() {
		case layers.LayerTypeGRE:
			// If the next layer type is MPLS, we don't
			// create the tunneling packet at this level, but at the next one.
			if i < len(packetLayers)-2 && packetLayers[i+1].LayerType() == layers.LayerTypeMPLS {
				continue
			}
			fallthrough
		case layers.LayerTypeVXLAN, layers.LayerTypeMPLS, layers.LayerTypeGeneve:
			p := gopacket.NewPacket(packetData[start:start+innerLength], topLayer.LayerType(), gopacket.NoCopy)
			flowPackets = append(flowPackets, FlowPacket{gopacket: &p, length: topLayerLength})

			// subtract the current encapsulation header length as we are going to change the
			// encapsulation layer
			topLayerLength -= int64(innerLength)

			start += innerLength
			innerLength = 0

			// change topLayer in case of multiple encapsulation
			if i+1 < len(packetLayers)-1 {
				topLayer = packetLayers[i+1]
			}
		}
	}

	if len(flowPackets) > 0 {
		p := gopacket.NewPacket(packetData[start:], topLayer.LayerType(), gopacket.NoCopy)
		flowPackets = append(flowPackets, FlowPacket{gopacket: &p, length: 0})
	} else {
		flowPackets = append(flowPackets, FlowPacket{gopacket: packet, length: outerLength})
	}

	return flowPackets
}

// FlowPacketsFromSFlowSample returns an array of FlowPackets as a sample
// contains mutlple records which generate a FlowPackets each.
func FlowPacketsFromSFlowSample(sample *layers.SFlowFlowSample) []FlowPackets {
	var flowPacketsSet []FlowPackets

	for _, rec := range sample.Records {
		switch rec.(type) {
		case layers.SFlowRawPacketFlowRecord:
			/* We only support RawPacket from SFlow probe */
		default:
			continue
		}

		record := rec.(layers.SFlowRawPacketFlowRecord)

		// each record can generate multiple FlowPacket in case of encapsulation
		if flowPackets := FlowPacketsFromGoPacket(&record.Header, int64(record.FrameLength-record.PayloadRemoved)); len(flowPackets) > 0 {
			flowPacketsSet = append(flowPacketsSet, flowPackets)
		}
	}

	return flowPacketsSet
}

func (f *FlowLayer) GetField(field string) (string, error) {
	if f == nil {
		return "", ErrFieldNotFound
	}

	switch field {
	case "A":
		return f.A, nil
	case "B":
		return f.B, nil
	case "Protocol":
		return f.Protocol.String(), nil
	}
	return "", ErrFieldNotFound
}

func (f *FlowMetric) GetField(field string) (int64, error) {
	switch field {
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

	// root field
	name := fields[0]
	switch name {
	case "UUID":
		return f.UUID, nil
	case "LayersPath":
		return f.LayersPath, nil
	case "TrackingID":
		return f.TrackingID, nil
	case "L3TrackingID":
		return f.L3TrackingID, nil
	case "ParentUUID":
		return f.ParentUUID, nil
	case "NodeTID":
		return f.NodeTID, nil
	case "ANodeTID":
		return f.ANodeTID, nil
	case "BNodeTID":
		return f.BNodeTID, nil
	case "Application":
		return f.Application, nil
	}

	// sub field
	if len(fields) != 2 {
		return "", ErrFieldNotFound
	}

	switch name {
	case "Link":
		return f.Link.GetField(fields[1])
	case "Network":
		return f.Network.GetField(fields[1])
	case "Transport":
		return f.Transport.GetField(fields[1])
	case "UDPPORT", "TCPPORT", "SCTPPORT":
		return f.Transport.GetField(fields[1])
	case "IPV4", "IPV6":
		return f.Network.GetField(fields[1])
	case "ETHERNET":
		return f.Link.GetField(fields[1])
	}
	return "", ErrFieldNotFound
}

func (f *Flow) GetFieldInt64(field string) (int64, error) {
	fields := strings.Split(field, ".")
	if len(fields) != 2 {
		return 0, ErrFieldNotFound
	}
	name := fields[0]
	switch name {
	case "Metric":
		return f.Metric.GetField(fields[1])
	case "LastUpdateMetric":
		return f.LastUpdateMetric.GetField(fields[1])
	}
	return 0, ErrFieldNotFound
}

func (f *Flow) GetFields() []interface{} {
	return fields
}

var fields []interface{}

func introspectFields(t reflect.Type, prefix string) []interface{} {
	var fFields []interface{}

	for i := 0; i < t.NumField(); i++ {
		vField := t.Field(i)
		tField := vField.Type
		vName := prefix + vField.Name

		for tField.Kind() == reflect.Ptr {
			tField = tField.Elem()
		}

		if tField.Kind() == reflect.Struct {
			fFields = append(fFields, introspectFields(tField, vName+".")...)
		} else {
			fFields = append(fFields, vName)
		}
	}

	return fFields
}

func init() {
	fields = introspectFields(reflect.TypeOf(Flow{}), "")
}
