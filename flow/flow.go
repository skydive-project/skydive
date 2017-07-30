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

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
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
	// DefaultFlowProtobufSize : the default protobuf size without any raw packet for a flow
	DefaultProtobufFlowSize = 500
)

// Packet describes one packet
type Packet struct {
	gopacket *gopacket.Packet
	length   int64
}

// Packets represents a suite of parent/child Packet
type Packets struct {
	Packets   []Packet
	Timestamp int64
}

// RawPackets embeds flow RawPacket array with the associated link type
type RawPackets struct {
	LinkType   layers.LinkType
	RawPackets []*RawPacket
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

func layerPathFromGoPacket(packet *gopacket.Packet) string {
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
		Metric:           &ExtFlowMetric{},
		LastUpdateMetric: &FlowMetric{},
	}
}

// UpdateUUID updates the flow UUID based on protocotols layers path and layers IDs
func (f *Flow) UpdateUUID(key string, L2ID int64, L3ID int64) {
	layersPath := strings.Replace(f.LayersPath, "Dot1Q/", "", -1)

	hasher := sha1.New()

	hasher.Write(f.Transport.Hash())
	hasher.Write(f.Network.Hash())
	if f.Network != nil {
		netID := make([]byte, 8)
		binary.BigEndian.PutUint64(netID, uint64(f.Network.ID))
		hasher.Write(netID)
	}
	hasher.Write([]byte(strings.TrimPrefix(layersPath, "Ethernet/")))
	f.L3TrackingID = hex.EncodeToString(hasher.Sum(nil))

	hasher.Write(f.Link.Hash())
	if f.Link != nil {
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

func uint32Distance(a uint32, b uint32) uint32 {
	if a >= b {
		return a - b
	} else {
		return 0xffffffff - (b - a)
	}
}

func getExpectedSeq(packet *gopacket.Packet) uint32 {
	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	ipv4Packet, ok := ipv4Layer.(*layers.IPv4)
	if !ok {
		logging.GetLogger().Error("Can not retrieve IP header")
		return 0
	}

	ipHeaderLen := uint32(ipv4Packet.IHL * 4)

	transportLayer := (*packet).Layer(layers.LayerTypeTCP)
	transportPacket, ok := transportLayer.(*layers.TCP)
	if !ok {
		logging.GetLogger().Error("Can not retrieve TCP header")
		return 0
	}

	tcpHeaderLen := uint32(transportPacket.DataOffset * 4)
	var expectedSeq uint32 = transportPacket.Seq + uint32(ipv4Packet.Length) - ipHeaderLen - tcpHeaderLen

	return expectedSeq
}

func (f *Flow) Init(key string, now int64, packet *gopacket.Packet, length int64, nodeTID string, parentUUID string, L2ID int64, L3ID int64) {
	f.Start = now
	f.Last = now

	f.newLinkLayer(packet, length)

	f.NodeTID = nodeTID
	f.ParentUUID = parentUUID
	f.Metric.LenBySeq = false

	f.LayersPath = layerPathFromGoPacket(packet)
	appLayers := strings.Split(f.LayersPath, "/")
	f.Application = appLayers[len(appLayers)-1]

	// no network layer then no transport layer
	if err := f.newNetworkLayer(packet); err == nil {
		f.newTransportLayer(packet)

		if config.GetConfig().GetBool("agent.tcp_len_by_seq") {
			// Check if we can calculate length by TCP SEQ
			transportLayer := (*packet).Layer(layers.LayerTypeTCP)
			transportPacket, ok := transportLayer.(*layers.TCP)
			if ok && transportPacket.SYN {
				logging.GetLogger().Notice("Track length by sequantial number")

				ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
				if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
					if f.Network.A == ipv4Packet.SrcIP.String() {
						f.Metric.ABExpectedSeq = getExpectedSeq(packet)
						logging.GetLogger().Notice("SYN AB", f.Metric.ABExpectedSeq)
					} else {
						f.Metric.BAExpectedSeq = getExpectedSeq(packet)
						logging.GetLogger().Notice("SYN BA", f.Metric.BAExpectedSeq)
					}
					f.Metric.LenBySeq = true
				}
			}
		}
	}

	// need to have as most variable filled as possible to get correct UUID
	f.UpdateUUID(key, L2ID, L3ID)
}

// Update a flow metrics
func (f *Flow) Update(now int64, packet *gopacket.Packet, length int64) {
	f.Last = now

	if f.Metric.LenBySeq {
		f.updateMetricsBySeq(packet)
	} else if updated := f.updateMetricsWithLinkLayer(packet, length); !updated {
		logging.GetLogger().Error("Update metrics with network\n")
		f.updateMetricsWithNetworkLayer(packet)
	}

	f.updateMetricsSynCapture(packet)
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
		return nil
	}
	return errors.New("Unable to decode the IP layer")
}

func (f *Flow) updateMetricsSynCapture(packet *gopacket.Packet) error {
	// capture content of SYN packets

	if !f.Metric.CaptureSyn {
		return nil
	}
	//bypass if not TCP
	if f.Transport == nil || f.Transport.Protocol != FlowProtocol_TCPPORT {
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
	logging.GetLogger().Notice("Capture SYN/FIN/RST packets")
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

	var captureTime int64

	captureTime = f.Last
	if metadata := (*packet).Metadata(); metadata != nil {
		captureTime = metadata.CaptureInfo.Timestamp.UnixNano() /
			(int64(time.Millisecond) / int64(time.Nanosecond))
	} else {
		logging.GetLogger().Notice("No metadata in packet")
		return nil
	}

	switch {
	case tcpPacket.SYN:
		synData := ""
		if app := (*packet).ApplicationLayer(); app != nil {
			synData = string(app.Payload())
		} else {
			logging.GetLogger().Notice("Capture SYN failed to parse app data. leaving empty")
		}
		if f.Network.A == srcIP {
			if f.Metric.ABSynStart == 0 {
				f.Metric.ABSynStart = captureTime
				f.Metric.ABSynTTL = timeToLive
				f.Metric.ABSynData = ""
				f.Metric.ABSynData += synData
			} else {
				logging.GetLogger().Notice("Duplicate SYNCapture AB", f.Metric.ABSynData)
			}
		} else {
			if f.Metric.BASynStart == 0 {
				f.Metric.BASynStart = captureTime
				f.Metric.BASynTTL = timeToLive
				f.Metric.BASynData = ""
				f.Metric.BASynData += synData
			} else {
				logging.GetLogger().Notice("Duplicate SYNCapture BA", f.Metric.BASynData)
			}
		}
	case tcpPacket.FIN:
		if f.Network.A == srcIP {
			f.Metric.ABFinStart = captureTime
		} else {
			f.Metric.BAFinStart = captureTime
		}
	case tcpPacket.RST:
		if f.Network.A == srcIP {
			f.Metric.ABRstStart = captureTime
		} else {
			f.Metric.BARstStart = captureTime
		}
	}

	return nil
}

func (f *Flow) updateMetricsBySeq(packet *gopacket.Packet) error {
	transportLayer := (*packet).Layer(layers.LayerTypeTCP)
	transportPacket, _ := transportLayer.(*layers.TCP)

	var expectedSeq uint32

	ipv4Layer := (*packet).Layer(layers.LayerTypeIPv4)
	ipv4Packet, ok := ipv4Layer.(*layers.IPv4)
	if ok {
		if f.Network.A == ipv4Packet.SrcIP.String() {
			if transportPacket.SYN {
				f.Metric.ABExpectedSeq = getExpectedSeq(packet)
				logging.GetLogger().Notice("SYN AB", f.Metric.ABExpectedSeq)
				return nil
			}
			expectedSeq = f.Metric.ABExpectedSeq
		} else {
			if transportPacket.SYN {
				f.Metric.BAExpectedSeq = getExpectedSeq(packet)
				logging.GetLogger().Notice("SYN BA", f.Metric.BAExpectedSeq)
				return nil
			}
			expectedSeq = f.Metric.BAExpectedSeq
		}
	} else {
		logging.GetLogger().Error("Error retrieving IP layer")
	}

	length := uint32Distance(transportPacket.Seq, expectedSeq)

	if transportPacket.Seq < 0x10000 {
		logging.GetLogger().Notice("Overlap", transportPacket.Seq, expectedSeq)
	}

	if length > 0x80010000 {
		logging.GetLogger().Notice("Detected retransmission", transportPacket.Seq)
		return nil
	}

	newExpectedSeq := getExpectedSeq(packet)
	length = uint32Distance(newExpectedSeq, expectedSeq)

	if f.Network.A == ipv4Packet.SrcIP.String() {
		f.Metric.ABPackets += int64(1)
		f.Metric.ABBytes += int64(length)
		f.Metric.ABExpectedSeq = newExpectedSeq
	} else {
		f.Metric.BAPackets += int64(1)
		f.Metric.BABytes += int64(length)
		f.Metric.BAExpectedSeq = newExpectedSeq
	}

	if transportPacket.FIN {
		logging.GetLogger().Notice("FIN", f.Metric.ABExpectedSeq, f.Metric.BAExpectedSeq, f.Metric.ABBytes, f.Metric.BABytes)
	}

	return nil
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
		if config.GetConfig().GetBool("agent.capture_syn") {
			f.Metric.CaptureSyn = true
			f.Metric.ABSynStart = int64(0)
			f.Metric.BASynStart = int64(0)
			return f.updateMetricsSynCapture(packet)
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

// PacketsFromGoPacket split original packet into multiple packets in
// case of encapsulation like GRE, VXLAN, etc.
func PacketsFromGoPacket(packet *gopacket.Packet, outerLength int64, t int64, bpf *BPF) *Packets {
	flowPackets := &Packets{Timestamp: t}

	if (*packet).Layer(gopacket.LayerTypeDecodeFailure) != nil {
		logging.GetLogger().Errorf("Decoding failure on layerpath %s", layerPathFromGoPacket(packet))
		logging.GetLogger().Debug((*packet).Dump())
		return flowPackets
	}

	packetData := (*packet).Data()
	if bpf != nil && !bpf.Matches(packetData) {
		return flowPackets
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
			flowPackets.Packets = append(flowPackets.Packets, Packet{gopacket: &p, length: topLayerLength})

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

	if len(flowPackets.Packets) > 0 {
		p := gopacket.NewPacket(packetData[start:], topLayer.LayerType(), gopacket.NoCopy)
		flowPackets.Packets = append(flowPackets.Packets, Packet{gopacket: &p, length: 0})
	} else {
		flowPackets.Packets = append(flowPackets.Packets, Packet{gopacket: packet, length: outerLength})
	}

	return flowPackets
}

// PacketsFromSFlowSample returns an array of Packets as a sample
// contains mutlple records which generate a Packets each.
func PacketsFromSFlowSample(sample *layers.SFlowFlowSample, t int64, bpf *BPF) []*Packets {
	var flowPacketsSet []*Packets

	for _, rec := range sample.Records {
		switch rec.(type) {
		case layers.SFlowRawPacketFlowRecord:
			/* We only support RawPacket from SFlow probe */
		default:
			continue
		}

		record := rec.(layers.SFlowRawPacketFlowRecord)

		// each record can generate multiple Packet in case of encapsulation
		if flowPackets := PacketsFromGoPacket(&record.Header, int64(record.FrameLength-record.PayloadRemoved), t, bpf); len(flowPackets.Packets) > 0 {
			flowPacketsSet = append(flowPacketsSet, flowPackets)
		}
	}

	return flowPacketsSet
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
	default:
		return 0, common.ErrFieldNotFound
	}
}

// GetField returns the value of a field
func (f *Flow) GetField(field string) (interface{}, error) {
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
