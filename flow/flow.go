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
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

// ErrFlowProtocol invalid protocol error
var ErrFlowProtocol = errors.New("FlowProtocol invalid")

const (
	// DefaultCaptureLength : default packet capture length
	DefaultCaptureLength uint32 = 256
	// MaxCaptureLength : maximum capture length accepted
	MaxCaptureLength uint32 = 4096
	// MaxRawPacketLimit : maximum raw packet captured, limitation could be removed once flow over tcp
	MaxRawPacketLimit uint32 = 10
	// DefaultProtobufFlowSize : the default protobuf size without any raw packet for a flow
	DefaultProtobufFlowSize = 500
)

// flowState is used internally to track states within the flow table.
// it is added to the generated Flow struct by Makefile
type flowState struct {
	lastMetric       *FlowMetric
	link1stPacket    int64
	network1stPacket int64
	skipSocketInfo   bool
	updateVersion    int64
}

// Packet describes one packet
type Packet struct {
	gopacket *gopacket.Packet
	length   int64
}

// PacketSequence represents a suite of parent/child Packet
type PacketSequence struct {
	Packets []Packet
}

// RawPackets embeds flow RawPacket array with the associated link type
type RawPackets struct {
	LinkType   layers.LinkType
	RawPackets []*RawPacket
}

// FlowOpts describes options that can be used to process flows
type FlowOpts struct {
	TCPMetric bool
}

// FlowUUIDs describes UUIDs that can be applied to flows
type FlowUUIDs struct {
	ParentUUID string
	L2ID       int64
	L3ID       int64
}

// SkipSocketInfo get or set the SocketInfo flow's state
func (f *Flow) SkipSocketInfo(v ...bool) bool {
	if len(v) > 0 {
		f.XXX_state.skipSocketInfo = v[0]
	}
	return f.XXX_state.skipSocketInfo
}

// Value returns int32 value of a FlowProtocol
func (x FlowProtocol) Value() int32 {
	return int32(x)
}

// MarshalJSON serialize a FlowLayer in JSON
func (f *FlowLayer) MarshalJSON() ([]byte, error) {
	obj := &struct {
		Protocol string
		A        string
		B        string
		ID       int64
	}{
		Protocol: f.Protocol.String(),
		A:        f.A,
		B:        f.B,
		ID:       f.ID,
	}

	return json.Marshal(&obj)
}

// UnmarshalJSON deserialize a JSON object in FlowLayer
func (f *FlowLayer) UnmarshalJSON(b []byte) error {
	m := struct {
		Protocol string
		A        string
		B        string
		ID       int64
	}{}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	protocol, ok := FlowProtocol_value[m.Protocol]
	if !ok {
		return ErrFlowProtocol
	}
	f.Protocol = FlowProtocol(protocol)
	f.A = m.A
	f.B = m.B
	f.ID = m.ID

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
	case gopacket.ApplicationLayer:
		switch l := l.(type) {
		case *ICMPv4:
			b := make([]byte, 4)
			binary.BigEndian.PutUint32(b, uint32(l.Type)<<24|uint32(l.TypeCode.Code())<<16|uint32(l.Id))
			return gopacket.NewFlow(0, b, nil)
		case *ICMPv6:
			b := make([]byte, 4)
			binary.BigEndian.PutUint32(b, uint32(l.Type)<<24|uint32(l.TypeCode.Code())<<16|uint32(l.Id))
			return gopacket.NewFlow(0, b, nil)
		}
	}
	return gopacket.Flow{}
}

// MarshalJSON serialize a ICMPLayer in JSON
func (i *ICMPLayer) MarshalJSON() ([]byte, error) {
	obj := &struct {
		Type string
		Code uint32
		ID   uint32
	}{
		Type: i.Type.String(),
		Code: i.Code,
		ID:   i.ID,
	}

	return json.Marshal(&obj)
}

// UnmarshalJSON deserialize a JSON object in ICMPLayer
func (i *ICMPLayer) UnmarshalJSON(b []byte) error {
	m := struct {
		Type string
		Code uint32
		ID   uint32
	}{}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	icmpType, ok := ICMPType_value[m.Type]
	if !ok {
		return ErrFlowProtocol
	}
	i.Type = ICMPType(icmpType)
	i.Code = m.Code
	i.ID = m.ID

	return nil
}

// Key describes a unique flow Key
type Key string

func (f Key) String() string {
	return string(f)
}

// KeyFromGoPacket returns the unique flow key
// The unique key is calculated based on parentUUID, network, transport and applicable layers
func KeyFromGoPacket(p *gopacket.Packet, parentUUID string) Key {
	network := layerFlow((*p).NetworkLayer()).FastHash()
	transport := layerFlow((*p).TransportLayer()).FastHash()
	application := layerFlow((*p).ApplicationLayer()).FastHash()
	return Key(parentUUID + strconv.FormatUint(uint64(network^transport^application), 10))
}

// LayerPathFromGoPacket returns path of all the layers separated by a slash.
func LayerPathFromGoPacket(packet *gopacket.Packet) string {
	path := ""
	for i, layer := range (*packet).Layers() {
		if layer.LayerType() == gopacket.LayerTypePayload {
			break
		}
		if i > 0 {
			path += "/"
		}
		path += layer.LayerType().String()
	}
	return strings.Replace(path, "Linux SLL/", "", 1)
}

func linkID(p *gopacket.Packet) int64 {
	id := int64(0)
	allLayers := (*p).Layers()
	for i := range allLayers {
		layer := allLayers[len(allLayers)-1-i]
		if layer.LayerType() == layers.LayerTypeDot1Q {
			id = (id << 12) | int64(layer.(*layers.Dot1Q).VLANIdentifier)
		}
	}
	return id
}

func networkID(p *gopacket.Packet) int64 {
	id := int64(0)
	allLayers := (*p).Layers()
	for i := range allLayers {
		layer := allLayers[len(allLayers)-1-i]
		if layer.LayerType() == layers.LayerTypeVXLAN {
			return int64(layer.(*layers.VXLAN).VNI)
		}
		if layer.LayerType() == layers.LayerTypeGRE {
			return int64(layer.(*layers.GRE).Key)
		}
		if layer.LayerType() == layers.LayerTypeGeneve {
			return int64(layer.(*layers.Geneve).VNI)
		}
	}
	return id
}

// NewFlow creates a new empty flow
func NewFlow() *Flow {
	return &Flow{
		Metric: &FlowMetric{},
	}
}

// UpdateUUID updates the flow UUID based on protocotols layers path and layers IDs
func (f *Flow) UpdateUUID(key string, L2ID int64, L3ID int64) {
	layersPath := strings.Replace(f.LayersPath, "Dot1Q/", "", -1)

	hasher := sha1.New()

	hasher.Write(f.Transport.Hash())
	if f.Network != nil {
		hasher.Write(f.Network.Hash())
		netID := make([]byte, 8)
		binary.BigEndian.PutUint64(netID, uint64(f.Network.ID))
		hasher.Write(netID)
	}
	hasher.Write([]byte(strings.TrimPrefix(layersPath, "Ethernet/")))
	f.L3TrackingID = hex.EncodeToString(hasher.Sum(nil))

	if f.Link != nil {
		hasher.Write(f.Link.Hash())
		linkID := make([]byte, 8)
		binary.BigEndian.PutUint64(linkID, uint64(f.Link.ID))
		hasher.Write(linkID)
	}

	if f.ICMP != nil {
		icmpID := make([]byte, 8*3)
		binary.BigEndian.PutUint64(icmpID, uint64(f.ICMP.Type))
		binary.BigEndian.PutUint64(icmpID, uint64(f.ICMP.Code))
		binary.BigEndian.PutUint64(icmpID, uint64(f.ICMP.ID))
		hasher.Write(icmpID)
	}

	hasher.Write([]byte(layersPath))
	f.TrackingID = hex.EncodeToString(hasher.Sum(nil))

	bfStart := make([]byte, 8)
	binary.BigEndian.PutUint64(bfStart, uint64(f.Start))
	hasher.Write(bfStart)
	hasher.Write([]byte(f.NodeTID))

	// include key so that we are sure that two flows with different keys don't
	// give the same UUID due to different ways of hash the headers.
	hasher.Write([]byte(key))
	bL2ID := make([]byte, 8)
	binary.BigEndian.PutUint64(bL2ID, uint64(L2ID))
	hasher.Write(bL2ID)
	bL3ID := make([]byte, 8)
	binary.BigEndian.PutUint64(bL3ID, uint64(L3ID))
	hasher.Write(bL3ID)

	f.UUID = hex.EncodeToString(hasher.Sum(nil))
}

// FromData deserialize a protobuf message to a Flow
func FromData(data []byte) (*Flow, error) {
	flow := new(Flow)

	err := proto.Unmarshal(data, flow)
	if err != nil {
		return nil, err
	}

	return flow, nil
}

// GetData serialize a Flow to a protobuf
func (f *Flow) GetData() ([]byte, error) {
	data, err := proto.Marshal(f)
	if err != nil {
		return []byte{}, err
	}

	return data, nil
}

// LinkType returns the Link type of the flow according the its first available layer.
func (f *Flow) LinkType() (layers.LinkType, error) {
	if f.Link != nil {
		return layers.LinkTypeEthernet, nil
	}

	if f.Network != nil {
		switch f.Network.Protocol {
		case FlowProtocol_IPV4:
			return layers.LinkTypeIPv4, nil
		case FlowProtocol_IPV6:
			return layers.LinkTypeIPv6, nil
		}
	}

	return 0, errors.New("LinkType unknown")
}

// Init initializes the flow with the given Timestamp, nodeTID and related UUIDs
func (f *Flow) Init(now int64, nodeTID string, uuids FlowUUIDs) {
	f.Start = now
	f.Last = now

	f.Metric.Start = now
	f.Metric.Last = now

	f.NodeTID = nodeTID
	f.ParentUUID = uuids.ParentUUID
}

// InitFromGoPacket initializes the flow based on packet data, flow key and ids
func (f *Flow) InitFromGoPacket(key string, packet *gopacket.Packet, length int64, nodeTID string, uuids FlowUUIDs, opts FlowOpts) {
	now := common.UnixMillis((*packet).Metadata().CaptureInfo.Timestamp)
	f.Init(now, nodeTID, uuids)

	f.newLinkLayer(packet, length)

	f.LayersPath = LayerPathFromGoPacket(packet)
	appLayers := strings.Split(f.LayersPath, "/")
	f.Application = appLayers[len(appLayers)-1]

	// no network layer then no transport layer
	if err := f.newNetworkLayer(packet); err == nil {
		f.newTransportLayer(packet, opts.TCPMetric)
	}

	// need to have as most variable filled as possible to get correct UUID
	f.UpdateUUID(key, uuids.L2ID, uuids.L3ID)
}

// Update a flow metrics and latency
func (f *Flow) Update(packet *gopacket.Packet, length int64) {
	now := common.UnixMillis((*packet).Metadata().CaptureInfo.Timestamp)
	f.Last = now
	f.Metric.Last = now

	if updated := f.updateMetricsWithLinkLayer(packet, length); !updated {
		f.updateMetricsWithNetworkLayer(packet)
	}
	if f.TCPFlowMetric != nil {
		f.updateTCPMetrics(packet)
	}
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
		ID:       linkID(packet),
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
	if !ok || f.Link == nil {
		// bypass if a Link layer can't be decoded, i.e. Network layer is the first layer
		return false
	}

	// if the length is given use it as the packet can be truncated like in SFlow
	if length == 0 {
		length = getLinkLayerLength(ethernetPacket)
	}

	if f.Link.A == ethernetPacket.SrcMAC.String() {
		f.Metric.ABPackets++
		f.Metric.ABBytes += length
	} else {
		f.Metric.BAPackets++
		f.Metric.BABytes += length
	}

	if f.XXX_state.link1stPacket == 0 {
		f.XXX_state.link1stPacket = (*packet).Metadata().Timestamp.UnixNano()
	} else {
		if (f.RTT == 0) && (f.Link.A == ethernetPacket.DstMAC.String()) {
			f.RTT = (*packet).Metadata().Timestamp.UnixNano() - f.XXX_state.link1stPacket
		}
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
			ID:       networkID(packet),
		}

		icmpLayer := (*packet).Layer(layers.LayerTypeICMPv4)
		if layer, ok := icmpLayer.(*ICMPv4); ok {
			f.ICMP = &ICMPLayer{
				Code: uint32(layer.TypeCode.Code()),
				Type: layer.Type,
				ID:   uint32(layer.Id),
			}
		}
		return f.updateMetricsWithNetworkLayer(packet)
	}

	ipv6Layer := (*packet).Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		f.Network = &FlowLayer{
			Protocol: FlowProtocol_IPV6,
			A:        ipv6Packet.SrcIP.String(),
			B:        ipv6Packet.DstIP.String(),
			ID:       networkID(packet),
		}

		icmpLayer := (*packet).Layer(layers.LayerTypeICMPv6)
		if layer, ok := icmpLayer.(*ICMPv6); ok {
			f.ICMP = &ICMPLayer{
				Code: uint32(layer.TypeCode.Code()),
				Type: layer.Type,
				ID:   uint32(layer.Id),
			}
		}
		return f.updateMetricsWithNetworkLayer(packet)
	}

	return errors.New("Unable to decode the network layer")
}

func (f *Flow) updateMetricsWithNetworkLayer(packet *gopacket.Packet) error {
	// bypass if a Link layer already exist
	if f.Link != nil {
		return nil
	}

	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		if f.Network.A == ipv4Packet.SrcIP.String() {
			f.Metric.ABPackets++
			f.Metric.ABBytes += int64(ipv4Packet.Length)
		} else {
			f.Metric.BAPackets++
			f.Metric.BABytes += int64(ipv4Packet.Length)
		}

		// update RTT
		if f.XXX_state.network1stPacket == 0 {
			f.XXX_state.network1stPacket = (*packet).Metadata().Timestamp.UnixNano()
		} else {
			if (f.RTT == 0) && (f.Network.A == ipv4Packet.DstIP.String()) {
				f.RTT = (*packet).Metadata().Timestamp.UnixNano() - f.XXX_state.network1stPacket
			}
		}
		return nil
	}
	ipv6Layer := (*packet).Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		if f.Network.A == ipv6Packet.SrcIP.String() {
			f.Metric.ABPackets++
			f.Metric.ABBytes += int64(ipv6Packet.Length)
		} else {
			f.Metric.BAPackets++
			f.Metric.BABytes += int64(ipv6Packet.Length)
		}

		// update RTT
		if f.XXX_state.network1stPacket == 0 {
			f.XXX_state.network1stPacket = (*packet).Metadata().Timestamp.UnixNano()
		} else {
			if (f.RTT == 0) && (f.Network.A == ipv6Packet.DstIP.String()) {
				f.RTT = (*packet).Metadata().Timestamp.UnixNano() - f.XXX_state.network1stPacket
			}
		}
		return nil
	}

	return errors.New("Unable to decode the IP layer")
}

func (f *Flow) updateTCPMetrics(packet *gopacket.Packet) error {
	// capture content of SYN packets
	//bypass if not TCP
	if f.Network == nil || f.Transport == nil || f.Transport.Protocol != FlowProtocol_TCPPORT {
		return nil
	}
	var metadata *gopacket.PacketMetadata
	if metadata = (*packet).Metadata(); metadata == nil {
		return nil
	}

	tcpLayer := (*packet).Layer(layers.LayerTypeTCP)
	tcpPacket, ok := tcpLayer.(*layers.TCP)
	if !ok {
		logging.GetLogger().Notice("Capture SYN unable to decode TCP layer. ignoring")
		return nil
	}

	// we capture SYN, FIN & RST
	if !(tcpPacket.SYN || tcpPacket.FIN || tcpPacket.RST) {
		return nil
	}
	var srcIP string
	var timeToLive uint32
	switch f.Network.Protocol {
	case FlowProtocol_IPV4:
		ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
		ipv4Packet, ok := ipv4Layer.(*layers.IPv4)
		if !ok {
			return errors.New("Unable to decode IPv4 Layer")
		}
		srcIP = ipv4Packet.SrcIP.String()
		timeToLive = uint32(ipv4Packet.TTL)
	case FlowProtocol_IPV6:
		ipv6Layer := (*packet).Layer(layers.LayerTypeIPv6)
		ipv6Packet, ok := ipv6Layer.(*layers.IPv6)
		if !ok {
			return errors.New("Unable to decode IPv4 Layer")
		}
		srcIP = ipv6Packet.SrcIP.String()
		timeToLive = uint32(ipv6Packet.HopLimit)
	default:
		logging.GetLogger().Notice("Capture SYN unknown IP version. ignoring")
		return nil
	}

	captureTime := common.UnixMillis(metadata.CaptureInfo.Timestamp)

	switch {
	case tcpPacket.SYN:
		if f.Network.A == srcIP {
			if f.TCPFlowMetric.ABSynStart == 0 {
				f.TCPFlowMetric.ABSynStart = captureTime
				f.TCPFlowMetric.ABSynTTL = timeToLive
			}
		} else {
			if f.TCPFlowMetric.BASynStart == 0 {
				f.TCPFlowMetric.BASynStart = captureTime
				f.TCPFlowMetric.BASynTTL = timeToLive
			}
		}
	case tcpPacket.FIN:
		if f.Network.A == srcIP {
			f.TCPFlowMetric.ABFinStart = captureTime
		} else {
			f.TCPFlowMetric.BAFinStart = captureTime
		}
	case tcpPacket.RST:
		if f.Network.A == srcIP {
			f.TCPFlowMetric.ABRstStart = captureTime
		} else {
			f.TCPFlowMetric.BARstStart = captureTime
		}
	}

	return nil
}

func (f *Flow) newTransportLayer(packet *gopacket.Packet, tcpMetric bool) error {
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
		if tcpMetric {
			f.TCPFlowMetric = &TCPMetric{}
			return f.updateTCPMetrics(packet)
		}
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

// PacketSeqFromGoPacket split original packet into multiple packets in
// case of encapsulation like GRE, VXLAN, etc.
func PacketSeqFromGoPacket(packet *gopacket.Packet, outerLength int64, bpf *BPF) *PacketSequence {
	ps := &PacketSequence{}
	if (*packet).Layer(gopacket.LayerTypeDecodeFailure) != nil {
		logging.GetLogger().Errorf("Decoding failure on layerpath %s", LayerPathFromGoPacket(packet))
		logging.GetLogger().Debug((*packet).Dump())
		return ps
	}

	packetData := (*packet).Data()
	if bpf != nil && !bpf.Matches(packetData) {
		return ps
	}

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
			// creates the tunneling packet at this level, but at the next one.
			if i < len(packetLayers)-2 && packetLayers[i+1].LayerType() == layers.LayerTypeMPLS {
				continue
			}
			fallthrough
			// We don't split on vlan layers.LayerTypeDot1Q
		case layers.LayerTypeVXLAN, layers.LayerTypeMPLS, layers.LayerTypeGeneve:
			p := gopacket.NewPacket(packetData[start:start+innerLength], topLayer.LayerType(), gopacket.NoCopy)
			ps.Packets = append(ps.Packets, Packet{gopacket: &p, length: topLayerLength})

			// subtract the current encapsulation header length as we are going to change the
			// encapsulation layer
			topLayerLength -= int64(innerLength)

			start += innerLength
			innerLength = 0

			// change topLayer in case of multiple encapsulation
			if i+1 <= len(packetLayers)-1 {
				topLayer = packetLayers[i+1]
			}
		}
	}

	if len(ps.Packets) > 0 {
		p := gopacket.NewPacket(packetData[start:], topLayer.LayerType(), gopacket.NoCopy)
		if metadata := (*packet).Metadata(); metadata != nil {
			p.Metadata().CaptureInfo = metadata.CaptureInfo
		}
		ps.Packets = append(ps.Packets, Packet{gopacket: &p, length: 0})
	} else {
		ps.Packets = append(ps.Packets, Packet{gopacket: packet, length: outerLength})
	}

	return ps
}

// PacketSeqFromSFlowSample returns an array of Packets as a sample
// contains mutlple records which generate a Packets each.
func PacketSeqFromSFlowSample(sample *layers.SFlowFlowSample, bpf *BPF) []*PacketSequence {
	var pss []*PacketSequence

	for _, rec := range sample.Records {
		switch rec.(type) {
		case layers.SFlowRawPacketFlowRecord:
			/* We only support RawPacket from SFlow probe */
		default:
			continue
		}

		record := rec.(layers.SFlowRawPacketFlowRecord)
		m := record.Header.Metadata()
		if m.CaptureInfo.Timestamp.IsZero() {
			m.CaptureInfo.Timestamp = time.Now()
		}
		// each record can generate multiple Packet in case of encapsulation
		if ps := PacketSeqFromGoPacket(&record.Header, int64(record.FrameLength-record.PayloadRemoved), bpf); len(ps.Packets) > 0 {
			pss = append(pss, ps)
		}
	}

	return pss
}

// GetStringField returns the value of a flow field
func (f *FlowLayer) GetStringField(field string) (string, error) {
	if f == nil {
		return "", common.ErrFieldNotFound
	}

	switch field {
	case "A":
		return f.A, nil
	case "B":
		return f.B, nil
	case "Protocol":
		return f.Protocol.String(), nil
	}
	return "", common.ErrFieldNotFound
}

// GetFieldInt64 returns the value of a flow field
func (f *FlowLayer) GetFieldInt64(field string) (int64, error) {
	if f == nil {
		return 0, common.ErrFieldNotFound
	}

	switch field {
	case "ID":
		return f.ID, nil
	}
	return 0, common.ErrFieldNotFound
}

// GetStringField returns the value of a ICMP field
func (i *ICMPLayer) GetStringField(field string) (string, error) {
	if i == nil {
		return "", common.ErrFieldNotFound
	}

	switch field {
	case "Type":
		return i.Type.String(), nil
	default:
		return "", common.ErrFieldNotFound
	}
}

// GetFieldInt64 returns the value of a ICMP field
func (i *ICMPLayer) GetFieldInt64(field string) (int64, error) {
	if i == nil {
		return 0, common.ErrFieldNotFound
	}

	switch field {
	case "ID":
		return int64(i.ID), nil
	default:
		return 0, common.ErrFieldNotFound
	}
}

// GetFieldInt64 returns the value of a ICMP field
func (i *TCPMetric) GetFieldInt64(field string) (int64, error) {
	if i == nil {
		return 0, common.ErrFieldNotFound
	}

	switch field {
	case "ABSynTTL":
		return int64(i.ABSynTTL), nil
	case "BASynTTL":
		return int64(i.BASynTTL), nil
	case "ABSynStart":
		return i.ABSynStart, nil
	case "BASynStart":
		return i.BASynStart, nil
	case "ABFinStart":
		return i.ABFinStart, nil
	case "BAFinStart":
		return i.BAFinStart, nil
	case "ABRstStart":
		return i.BARstStart, nil
	case "BARstStart":
		return i.BARstStart, nil
	default:
		return 0, common.ErrFieldNotFound
	}
}

// GetStringField returns the value of a SocketInfo field
func (si *SocketInfo) GetStringField(field string) (string, error) {
	if si == nil {
		return "", common.ErrFieldNotFound
	}

	switch field {
	case "Process":
		return si.Process, nil
	case "Name":
		return si.Name, nil
	default:
		return "", common.ErrFieldNotFound
	}
}

// GetFieldInt64 returns the value of a SocketInfo field
func (si *SocketInfo) GetFieldInt64(field string) (int64, error) {
	if si == nil {
		return 0, common.ErrFieldNotFound
	}

	switch field {
	case "Pid":
		return si.Pid, nil
	default:
		return 0, common.ErrFieldNotFound
	}
}

// GetFieldString returns the value of a Flow field
func (f *Flow) GetFieldString(field string) (string, error) {
	fields := strings.Split(field, ".")
	if len(fields) < 1 {
		return "", common.ErrFieldNotFound
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
		return "", common.ErrFieldNotFound
	}

	switch name {
	case "Link":
		return f.Link.GetStringField(fields[1])
	case "Network":
		return f.Network.GetStringField(fields[1])
	case "ICMP":
		return f.ICMP.GetStringField(fields[1])
	case "Transport":
		return f.Transport.GetStringField(fields[1])
	case "UDPPORT", "TCPPORT", "SCTPPORT":
		return f.Transport.GetStringField(fields[1])
	case "IPV4", "IPV6":
		return f.Network.GetStringField(fields[1])
	case "ETHERNET":
		return f.Link.GetStringField(fields[1])
	case "SocketA":
		return f.SocketA.GetStringField(fields[1])
	case "SocketB":
		return f.SocketB.GetStringField(fields[1])
	}
	return "", common.ErrFieldNotFound
}

// GetFieldInt64 returns the value of a Flow field
func (f *Flow) GetFieldInt64(field string) (_ int64, err error) {
	switch field {
	case "Last":
		return f.Last, nil
	case "Start":
		return f.Start, nil
	case "RTT":
		return f.RTT, nil
	}

	fields := strings.Split(field, ".")
	if len(fields) != 2 {
		return 0, common.ErrFieldNotFound
	}
	name := fields[0]
	switch name {
	case "Metric":
		return f.Metric.GetFieldInt64(fields[1])
	case "LastUpdateMetric":
		return f.LastUpdateMetric.GetFieldInt64(fields[1])
	case "TCPFlowMetric":
		return f.TCPFlowMetric.GetFieldInt64(fields[1])
	case "Link":
		return f.Link.GetFieldInt64(fields[1])
	case "Network":
		return f.Network.GetFieldInt64(fields[1])
	case "ICMP":
		return f.ICMP.GetFieldInt64(fields[1])
	case "Transport":
		return f.Transport.GetFieldInt64(fields[1])
	case "RawPacketsCaptured":
		return f.RawPacketsCaptured, nil
	case "SocketA":
		return f.SocketA.GetFieldInt64(fields[1])
	case "SocketB":
		return f.SocketB.GetFieldInt64(fields[1])
	default:
		return 0, common.ErrFieldNotFound
	}
}

// GetFieldInterface returns the value of a Flow field
func (f *Flow) GetFieldInterface(field string) (_ interface{}, err error) {
	switch field {
	case "Metric":
		return f.Metric, nil
	case "LastUpdateMetric":
		return f.LastUpdateMetric, nil
	case "TCPFlowMetric":
		return f.TCPFlowMetric, nil
	case "Link":
		return f.Link, nil
	case "Network":
		return f.Network, nil
	case "ICMP":
		return f.ICMP, nil
	case "Transport":
		return f.Transport, nil
	case "SocketA":
		return f.SocketA, nil
	case "SocketB":
		return f.SocketB, nil
	default:
		return 0, common.ErrFieldNotFound
	}
}

// GetField returns the value of a field
func (f *Flow) GetField(field string) (interface{}, error) {
	if i, err := f.GetFieldInterface(field); err == nil {
		return i, nil
	}

	if i, err := f.GetFieldInt64(field); err == nil {
		return i, nil
	}

	return f.GetFieldString(field)
}

// GetFields returns the list of valid field of a Flow
func (f *Flow) GetFields() []interface{} {
	return fields
}

var fields []interface{}

func introspectFields(t reflect.Type, prefix string) []interface{} {
	var fFields []interface{}

	for i := 0; i < t.NumField(); i++ {
		vField := t.Field(i)
		tField := vField.Type

		// ignore XXX fields as there are considered as private
		if strings.HasPrefix(vField.Name, "XXX_") {
			continue
		}

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
