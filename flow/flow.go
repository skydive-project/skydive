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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/spaolacci/murmur3"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

var (
	// ErrFlowProtocol invalid protocol error
	ErrFlowProtocol = errors.New("FlowProtocol invalid")
	// ErrLayerNotFound layer not present in the packet
	ErrLayerNotFound = errors.New("Layer not found")
)

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
	updateVersion    int64
}

// Packet describes one packet
type Packet struct {
	GoPacket gopacket.Packet // orignal gopacket

	Layers   []gopacket.Layer // layer of the sub packet
	Data     []byte           // byte of the sub packet
	Length   int64            // length of the original packet meaning layers + payload
	IPMetric *IPMetric

	linkLayer        gopacket.LinkLayer        // fast access to link layer
	networkLayer     gopacket.NetworkLayer     // fast access to network layer
	transportLayer   gopacket.TransportLayer   // fast access to transport layer
	applicationLayer gopacket.ApplicationLayer // fast access to application layer
}

// PacketSequence represents a suite of parent/child Packet
type PacketSequence struct {
	Packets []*Packet
}

// RawPackets embeds flow RawPacket array with the associated link type
type RawPackets struct {
	LinkType   layers.LinkType
	RawPackets []*RawPacket
}

// defines what are the layers used for the flow key calculation
type LayerKeyMode int

const (
	DefaultLayerKeyMode              = L2KeyMode
	L2KeyMode           LayerKeyMode = 0 // uses Layer2 and Layer3 for hash computation, default mode
	L3PreferedKeyMode   LayerKeyMode = 1 // uses Layer3 only and layer2 if no Layer3
)

// FlowOpts describes options that can be used to process flows
type FlowOpts struct {
	TCPMetric    bool
	IPDefrag     bool
	LayerKeyMode LayerKeyMode
	AppPortMap   *ApplicationPortMap
}

// FlowUUIDs describes UUIDs that can be applied to flows
type FlowUUIDs struct {
	ParentUUID string
	L2ID       int64
	L3ID       int64
}

func (l LayerKeyMode) String() string {
	if l == L2KeyMode {
		return "L2"
	}
	return "L3"
}

func LayerKeyModeByName(name string) (LayerKeyMode, error) {
	switch name {
	case "L2":
		return L2KeyMode, nil
	case "L3":
		return L3PreferedKeyMode, nil
	}
	return L2KeyMode, errors.New("LayerKeyMode unknown")
}

func DefaultLayerKeyModeName() string {
	mode := config.GetString("flow.default_layer_key_mode")
	if mode == "" {
		mode = DefaultLayerKeyMode.String()
	}
	return mode
}

// Layer returns the given layer type
func (p *Packet) Layer(t gopacket.LayerType) gopacket.Layer {
	for _, l := range p.Layers {
		if l.LayerType() == t {
			return l
		}
	}
	return nil
}

// LinkLayer returns first link layer
func (p *Packet) LinkLayer() gopacket.LinkLayer {
	if p.linkLayer != nil {
		return p.linkLayer
	}

	if layer := p.Layer(layers.LayerTypeEthernet); layer != nil {
		p.linkLayer = layer.(*layers.Ethernet)
	}
	return p.linkLayer
}

// NetworkLayer return first network layer
func (p *Packet) NetworkLayer() gopacket.NetworkLayer {
	if p.networkLayer != nil {
		return p.networkLayer
	}

	if layer := p.Layer(layers.LayerTypeIPv4); layer != nil {
		p.networkLayer = layer.(*layers.IPv4)
	} else if layer := p.Layer(layers.LayerTypeIPv6); layer != nil {
		p.networkLayer = layer.(*layers.IPv6)
	}
	return p.networkLayer
}

// TransportLayer returns first transport layer
func (p *Packet) TransportLayer() gopacket.TransportLayer {
	if p.transportLayer != nil {
		return p.transportLayer
	}

	if layer := p.Layer(layers.LayerTypeUDP); layer != nil {
		p.transportLayer = layer.(*layers.UDP)
	} else if layer := p.Layer(layers.LayerTypeTCP); layer != nil {
		p.transportLayer = layer.(*layers.TCP)
	} else if layer := p.Layer(layers.LayerTypeSCTP); layer != nil {
		p.transportLayer = layer.(*layers.SCTP)
	}
	return p.transportLayer
}

// ApplicationFlow returns first application flow
func (p *Packet) ApplicationFlow() (gopacket.Flow, error) {
	if layer := p.Layer(layers.LayerTypeICMPv4); layer != nil {
		l := layer.(*ICMPv4)
		value32 := make([]byte, 4)
		binary.BigEndian.PutUint32(value32, uint32(l.Type)<<24|uint32(l.TypeCode.Code())<<16|uint32(l.Id))
		return gopacket.NewFlow(0, value32, nil), nil
	} else if layer := p.Layer(layers.LayerTypeICMPv6); layer != nil {
		l := layer.(*ICMPv6)
		value32 := make([]byte, 4)
		binary.BigEndian.PutUint32(value32, uint32(l.Type)<<24|uint32(l.TypeCode.Code())<<16|uint32(l.Id))
		return gopacket.NewFlow(0, value32, nil), nil
	}

	return gopacket.Flow{}, ErrLayerNotFound
}

// TransportFlow returns first transport flow
func (p *Packet) TransportFlow() (gopacket.Flow, error) {
	layer := p.TransportLayer()
	if layer == nil {
		return gopacket.Flow{}, ErrLayerNotFound
	}

	// check vxlan in order to ignore source port from hash calculation
	if layer.LayerType() == layers.LayerTypeUDP {
		encap := p.Layers[len(p.Layers)-1]

		if encap.LayerType() == layers.LayerTypeVXLAN || encap.LayerType() == layers.LayerTypeGeneve {
			value16 := make([]byte, 2)
			binary.BigEndian.PutUint16(value16, uint16(layer.(*layers.UDP).DstPort))

			// use the vni and the dest port to distinguish flows
			value32 := make([]byte, 4)
			if encap.LayerType() == layers.LayerTypeVXLAN {
				binary.BigEndian.PutUint32(value32, encap.(*layers.VXLAN).VNI)
			} else {
				binary.BigEndian.PutUint32(value32, encap.(*layers.Geneve).VNI)
			}

			return gopacket.NewFlow(0, value32, value16), nil
		}
	}

	return layer.TransportFlow(), nil
}

// Key returns the unique flow key
// The unique key is calculated based on parentUUID, network, transport and applicable layers
func (p *Packet) Key(parentUUID string, opts FlowOpts) string {
	var uuid uint64

	// uses L2 is requested or if there is no network layer
	if opts.LayerKeyMode == L2KeyMode || p.NetworkLayer() == nil {
		if layer := p.LinkLayer(); layer != nil {
			uuid ^= layer.LinkFlow().FastHash()
		}
	}
	if layer := p.NetworkLayer(); layer != nil {
		uuid ^= layer.NetworkFlow().FastHash()
	}
	if tf, err := p.TransportFlow(); err == nil {
		uuid ^= tf.FastHash()
	}
	if af, err := p.ApplicationFlow(); err == nil {
		uuid ^= af.FastHash()
	}

	return parentUUID + strconv.FormatUint(uuid, 10)
}

// Value returns int32 value of a FlowProtocol
func (p FlowProtocol) Value() int32 {
	return int32(p)
}

// MarshalJSON serialize a FlowProtocol in JSON
func (p FlowProtocol) MarshalJSON() ([]byte, error) {
	return []byte("\"" + p.String() + "\""), nil
}

// UnmarshalJSON serialize a FlowProtocol in JSON
func (p *FlowProtocol) UnmarshalJSON(b []byte) error {
	protocol, ok := FlowProtocol_value[string(b[1:len(b)-1])]
	if !ok {
		return ErrFlowProtocol
	}
	*p = FlowProtocol(protocol)

	return nil
}

// MarshalJSON serialize a FlowProtocol in JSON
func (i ICMPType) MarshalJSON() ([]byte, error) {
	return []byte("\"" + i.String() + "\""), nil
}

// UnmarshalJSON serialize a ICMPType in JSON
func (i *ICMPType) UnmarshalJSON(b []byte) error {
	icmpType, ok := ICMPType_value[string(b[1:len(b)-1])]
	if !ok {
		return ErrFlowProtocol
	}
	*i = ICMPType(icmpType)

	return nil
}

// GetFirstLayerType returns layer type and link type according to the given encapsulation
func GetFirstLayerType(encapType string) (gopacket.LayerType, layers.LinkType) {
	switch encapType {
	case "ether":
		return layers.LayerTypeEthernet, layers.LinkTypeEthernet
	case "none", "gre":
		return LayerTypeRawIP, layers.LinkTypeIPv4
	case "sit", "ipip":
		return layers.LayerTypeIPv4, layers.LinkTypeIPv4
	case "tunnel6", "gre6":
		return layers.LayerTypeIPv6, layers.LinkTypeIPv6
	default:
		logging.GetLogger().Warningf("Encapsulation unknown %s, defaulting to Ethernet", encapType)
		return layers.LayerTypeEthernet, layers.LinkTypeEthernet
	}
}

// LayersPath returns path and the application of all the layers separated by a slash.
func LayersPath(ls []gopacket.Layer) (string, string) {
	var app, path string
	for i, layer := range ls {
		tp := layer.LayerType()
		if tp == layers.LayerTypeLinuxSLL {
			continue
		}
		if tp == gopacket.LayerTypePayload || tp == gopacket.LayerTypeDecodeFailure {
			break
		}
		if i > 0 {
			path += "/"
		}
		app = layer.LayerType().String()
		path += app
	}
	return path, app
}

func linkID(p *Packet) int64 {
	id := int64(0)

	length := len(p.Layers) - 1
	for i := range p.Layers {
		layer := p.Layers[length-i]
		if layer.LayerType() == layers.LayerTypeDot1Q {
			id = (id << 12) | int64(layer.(*layers.Dot1Q).VLANIdentifier)
		}
	}
	return id
}

func networkID(p *Packet) int64 {
	id := int64(0)

	length := len(p.Layers) - 1
	for i := range p.Layers {
		layer := p.Layers[length-i]
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

// NewFlowFromGoPacket creates a new flow from the given gopacket
func NewFlowFromGoPacket(p gopacket.Packet, nodeTID string, uuids FlowUUIDs, opts FlowOpts) *Flow {
	f := NewFlow()

	var length int64
	if p.Metadata() != nil {
		length = int64(p.Metadata().CaptureLength)
	}

	packet := &Packet{
		GoPacket: p,
		Layers:   p.Layers(),
		Data:     p.Data(),
		Length:   length,
	}

	f.initFromPacket(packet.Key(uuids.ParentUUID, opts), packet, nodeTID, uuids, opts)

	return f
}

// UpdateUUID updates the flow UUID based on protocotols layers path and layers IDs
func (f *Flow) UpdateUUID(key string, opts FlowOpts) {
	layersPath := strings.Replace(f.LayersPath, "Dot1Q/", "", -1)

	hasher := murmur3.New64()
	f.Network.Hash(hasher)
	f.ICMP.Hash(hasher)
	f.Transport.Hash(hasher)

	// only need network and transport to compute l3trackingID
	hasher.Write([]byte(strings.TrimPrefix(layersPath, "Ethernet/")))
	f.L3TrackingID = hex.EncodeToString(hasher.Sum(nil))

	if opts.LayerKeyMode == L2KeyMode || f.Network == nil {
		f.Link.Hash(hasher)
	}

	hasher.Write([]byte(layersPath))
	f.TrackingID = hex.EncodeToString(hasher.Sum(nil))

	value64 := make([]byte, 8)
	binary.BigEndian.PutUint64(value64, uint64(f.Start))
	hasher.Write(value64)
	hasher.Write([]byte(f.NodeTID))

	// include key so that we are sure that two flows with different keys don't
	// give the same UUID due to different ways of hash the headers.
	hasher.Write([]byte(key))

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

// initFromPacket initializes the flow based on packet data, flow key and ids
func (f *Flow) initFromPacket(key string, packet *Packet, nodeTID string, uuids FlowUUIDs, opts FlowOpts) {
	now := common.UnixMillis(packet.GoPacket.Metadata().CaptureInfo.Timestamp)
	f.Init(now, nodeTID, uuids)

	f.newLinkLayer(packet)

	f.LayersPath, f.Application = LayersPath(packet.Layers)

	// no network layer then no transport layer
	if err := f.newNetworkLayer(packet); err == nil {
		f.newTransportLayer(packet, opts)
	}

	// need to have as most variable filled as possible to get correct UUID
	f.UpdateUUID(key, opts)

	// update metrics
	f.Update(packet, opts)
}

// Update a flow metrics and latency
func (f *Flow) Update(packet *Packet, opts FlowOpts) {
	now := common.UnixMillis(packet.GoPacket.Metadata().CaptureInfo.Timestamp)
	f.Last = now
	f.Metric.Last = now

	if opts.LayerKeyMode == L3PreferedKeyMode {
		// use the ethernet length as we want to get the full size and we want to
		// rely on the l3 address order.
		length := packet.Length
		if length == 0 {
			if ethernetPacket := getLinkLayer(packet); ethernetPacket != nil {
				length = getLinkLayerLength(ethernetPacket)
			}
		}

		f.updateMetricsWithNetworkLayer(packet, length)
	} else {
		if updated := f.updateMetricsWithLinkLayer(packet); !updated {
			f.updateMetricsWithNetworkLayer(packet, 0)
		}
	}
	if f.TCPMetric != nil {
		f.updateTCPMetrics(packet)
	}
}

func (f *Flow) newLinkLayer(packet *Packet) error {
	ethernetLayer := packet.Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		// bypass if a Link layer can't be decoded, i.e. Network layer is the first layer
		return ErrLayerNotFound
	}

	f.Link = &FlowLayer{
		Protocol: FlowProtocol_ETHERNET,
		A:        ethernetPacket.SrcMAC.String(),
		B:        ethernetPacket.DstMAC.String(),
		ID:       linkID(packet),
	}

	return nil
}

func getLinkLayer(packet *Packet) *layers.Ethernet {
	ethernetLayer := packet.Layer(layers.LayerTypeEthernet)
	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		return nil
	}
	return ethernetPacket
}

func getLinkLayerLength(packet *layers.Ethernet) int64 {
	if packet.Length > 0 { // LLC
		return 14 + int64(packet.Length)
	}

	// Metric in that case will not be correct if payload > captureLength
	return 14 + int64(len(packet.Payload))
}

func (f *Flow) updateMetricsWithLinkLayer(packet *Packet) bool {
	ethernetPacket := getLinkLayer(packet)
	if ethernetPacket == nil || f.Link == nil {
		return false
	}

	length := packet.Length
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
		f.XXX_state.link1stPacket = packet.GoPacket.Metadata().Timestamp.UnixNano()
	} else {
		if (f.RTT == 0) && (f.Link.A == ethernetPacket.DstMAC.String()) {
			f.RTT = packet.GoPacket.Metadata().Timestamp.UnixNano() - f.XXX_state.link1stPacket
		}
	}

	return true
}

func (f *Flow) newNetworkLayer(packet *Packet) error {
	ipv4Layer := packet.Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		f.Network = &FlowLayer{
			Protocol: FlowProtocol_IPV4,
			A:        ipv4Packet.SrcIP.String(),
			B:        ipv4Packet.DstIP.String(),
			ID:       networkID(packet),
		}
		f.IPMetric = packet.IPMetric

		icmpLayer := packet.Layer(layers.LayerTypeICMPv4)
		if layer, ok := icmpLayer.(*ICMPv4); ok {
			f.ICMP = &ICMPLayer{
				Code: uint32(layer.TypeCode.Code()),
				Type: layer.Type,
				ID:   uint32(layer.Id),
			}
		}
		return nil
	}

	ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		f.Network = &FlowLayer{
			Protocol: FlowProtocol_IPV6,
			A:        ipv6Packet.SrcIP.String(),
			B:        ipv6Packet.DstIP.String(),
			ID:       networkID(packet),
		}

		icmpLayer := packet.Layer(layers.LayerTypeICMPv6)
		if layer, ok := icmpLayer.(*ICMPv6); ok {
			f.ICMP = &ICMPLayer{
				Code: uint32(layer.TypeCode.Code()),
				Type: layer.Type,
				ID:   uint32(layer.Id),
			}
		}
		return nil
	}

	return ErrLayerNotFound
}

func (f *Flow) updateMetricsWithNetworkLayer(packet *Packet, length int64) error {
	ipv4Layer := packet.Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		if length == 0 {
			length = int64(ipv4Packet.Length)
		}
		if f.Network.A == ipv4Packet.SrcIP.String() {
			f.Metric.ABPackets++
			f.Metric.ABBytes += length
		} else {
			f.Metric.BAPackets++
			f.Metric.BABytes += length
		}

		// update RTT
		if f.XXX_state.network1stPacket == 0 {
			f.XXX_state.network1stPacket = packet.GoPacket.Metadata().Timestamp.UnixNano()
		} else {
			if (f.RTT == 0) && (f.Network.A == ipv4Packet.DstIP.String()) {
				f.RTT = packet.GoPacket.Metadata().Timestamp.UnixNano() - f.XXX_state.network1stPacket
			}
		}
		return nil
	}
	ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		if length == 0 {
			length = int64(ipv6Packet.Length)
		}
		if f.Network.A == ipv6Packet.SrcIP.String() {
			f.Metric.ABPackets++
			f.Metric.ABBytes += length
		} else {
			f.Metric.BAPackets++
			f.Metric.BABytes += length
		}

		// update RTT
		if f.XXX_state.network1stPacket == 0 {
			f.XXX_state.network1stPacket = packet.GoPacket.Metadata().Timestamp.UnixNano()
		} else {
			if (f.RTT == 0) && (f.Network.A == ipv6Packet.DstIP.String()) {
				f.RTT = packet.GoPacket.Metadata().Timestamp.UnixNano() - f.XXX_state.network1stPacket
			}
		}
		return nil
	}

	return ErrLayerNotFound
}

func (f *Flow) updateTCPMetrics(packet *Packet) error {
	// capture content of SYN packets
	// bypass if not TCP
	if f.Network == nil || f.Transport == nil || f.Transport.Protocol != FlowProtocol_TCP {
		return nil
	}
	var metadata *gopacket.PacketMetadata
	if metadata = packet.GoPacket.Metadata(); metadata == nil {
		return nil
	}

	tcpLayer := packet.Layer(layers.LayerTypeTCP)
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
		ipv4Layer := packet.Layer(layers.LayerTypeIPv4)
		ipv4Packet, ok := ipv4Layer.(*layers.IPv4)
		if !ok {
			return ErrLayerNotFound
		}
		srcIP = ipv4Packet.SrcIP.String()
		timeToLive = uint32(ipv4Packet.TTL)
	case FlowProtocol_IPV6:
		ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
		ipv6Packet, ok := ipv6Layer.(*layers.IPv6)
		if !ok {
			return ErrLayerNotFound
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
			if f.TCPMetric.ABSynStart == 0 {
				f.TCPMetric.ABSynStart = captureTime
				f.TCPMetric.ABSynTTL = timeToLive
			}
		} else {
			if f.TCPMetric.BASynStart == 0 {
				f.TCPMetric.BASynStart = captureTime
				f.TCPMetric.BASynTTL = timeToLive
			}
		}
	case tcpPacket.FIN:
		if f.Network.A == srcIP {
			f.TCPMetric.ABFinStart = captureTime
		} else {
			f.TCPMetric.BAFinStart = captureTime
		}
	case tcpPacket.RST:
		if f.Network.A == srcIP {
			f.TCPMetric.ABRstStart = captureTime
		} else {
			f.TCPMetric.BARstStart = captureTime
		}
	}

	return nil
}

func (f *Flow) newTransportLayer(packet *Packet, opts FlowOpts) error {
	if layer := packet.Layer(layers.LayerTypeTCP); layer != nil {
		f.Transport = &TransportLayer{Protocol: FlowProtocol_TCP}

		transportPacket := layer.(*layers.TCP)
		srcPort, dstPort := int(transportPacket.SrcPort), int(transportPacket.DstPort)
		f.Transport.A, f.Transport.B = int64(srcPort), int64(dstPort)

		if app, ok := opts.AppPortMap.TCPApplication(srcPort, dstPort); ok {
			f.Application = app
		}

		if opts.TCPMetric {
			f.TCPMetric = &TCPMetric{}
		}
	} else if layer := packet.Layer(layers.LayerTypeUDP); layer != nil {
		f.Transport = &TransportLayer{Protocol: FlowProtocol_UDP}

		transportPacket := layer.(*layers.UDP)
		srcPort, dstPort := int(transportPacket.SrcPort), int(transportPacket.DstPort)
		f.Transport.A, f.Transport.B = int64(srcPort), int64(dstPort)

		if app, ok := opts.AppPortMap.UDPApplication(srcPort, dstPort); ok {
			f.Application = app
		}
	} else if layer := packet.Layer(layers.LayerTypeSCTP); layer != nil {
		f.Transport = &TransportLayer{Protocol: FlowProtocol_SCTP}

		transportPacket := layer.(*layers.SCTP)
		f.Transport.A = int64(transportPacket.SrcPort)
		f.Transport.B = int64(transportPacket.DstPort)
	} else {
		return ErrLayerNotFound
	}

	return nil
}

// PacketSeqFromGoPacket split original packet into multiple packets in
// case of encapsulation like GRE, VXLAN, etc.
func PacketSeqFromGoPacket(packet gopacket.Packet, outerLength int64, bpf *BPF, defragger *IPDefragger) *PacketSequence {
	ps := &PacketSequence{}

	// defragment and set ip metric if requested
	var ipMetric *IPMetric
	if defragger != nil {
		m, ok := defragger.Defrag(packet)
		if !ok {
			return ps
		}
		ipMetric = m
	}

	if packet.ErrorLayer() != nil {
		logging.GetLogger().Debugf("Decoding or partial decoding error : %s\n", packet.Dump())
	}

	if packet.LinkLayer() == nil && packet.NetworkLayer() == nil {
		logging.GetLogger().Debugf("Unknown packet : %s\n", packet.Dump())
		return ps
	}

	packetData := packet.Data()
	if bpf != nil && !bpf.Matches(packetData) {
		return ps
	}

	packetLayers := packet.Layers()
	metadata := packet.Metadata()

	var topLayer = packetLayers[0]

	if outerLength == 0 {
		if ethernetPacket, ok := topLayer.(*layers.Ethernet); ok {
			if metadata != nil && metadata.Length > 0 {
				outerLength = int64(metadata.Length)
			} else {
				outerLength = getLinkLayerLength(ethernetPacket)
			}
		} else if ipv4Packet, ok := topLayer.(*layers.IPv4); ok {
			outerLength = int64(ipv4Packet.Length)
		} else if ipv6Packet, ok := topLayer.(*layers.IPv6); ok {
			outerLength = int64(ipv6Packet.Length)
		}
	}

	// length of the encapsulation header + the inner packet
	topLayerIndex, topLayerOffset, topLayerLength := 0, 0, int(outerLength)

	offset, length := topLayerOffset, topLayerLength
	for i, layer := range packetLayers {
		length -= len(layer.LayerContents())
		offset += len(layer.LayerContents())

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
			p := &Packet{
				GoPacket: packet,
				Layers:   packetLayers[topLayerIndex : i+1],
				Data:     packetData[topLayerOffset:],
				Length:   int64(topLayerLength),
			}
			// As this is the top flow, we can use the layer pointer from GoPacket
			// This avoid to parse them later.
			if len(ps.Packets) == 0 {
				p.networkLayer = packet.NetworkLayer()
				p.transportLayer = packet.TransportLayer()
			}

			ps.Packets = append(ps.Packets, p)

			topLayerIndex = i + 1
			topLayerLength = length
			topLayerOffset = offset
		}
	}

	p := &Packet{
		GoPacket: packet,
		Layers:   packetLayers[topLayerIndex:],
		Data:     packetData[topLayerOffset:],
		Length:   int64(topLayerLength),
		IPMetric: ipMetric,
	}
	if len(ps.Packets) == 0 {
		// As this is the top flow, we can use the layer pointer from GoPacket
		// This avoid to parse them later.
		p.networkLayer = packet.NetworkLayer()
		p.transportLayer = packet.TransportLayer()
	}

	ps.Packets = append(ps.Packets, p)

	return ps
}

// PacketSeqFromSFlowSample returns an array of Packets as a sample
// contains mutlple records which generate a Packets each.
func PacketSeqFromSFlowSample(sample *layers.SFlowFlowSample, bpf *BPF, defragger *IPDefragger) []*PacketSequence {
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
		if ps := PacketSeqFromGoPacket(record.Header, int64(record.FrameLength-record.PayloadRemoved), bpf, defragger); len(ps.Packets) > 0 {
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

//GetStringField returns the value of a transport layer field
func (tl *TransportLayer) GetStringField(field string) (string, error) {
	if tl == nil {
		return "", common.ErrFieldNotFound
	}

	switch field {
	case "Protocol":
		return tl.Protocol.String(), nil
	}
	return "", common.ErrFieldNotFound
}

//GetFieldInt64 returns the value of a transport layer firld
func (tl *TransportLayer) GetFieldInt64(field string) (int64, error) {
	if tl == nil {
		return 0, common.ErrFieldNotFound
	}

	switch field {
	case "ID":
		return tl.ID, nil
	case "A":
		return tl.A, nil
	case "B":
		return tl.B, nil
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

// GetFieldInt64 returns the value of a IPMetric field
func (i *IPMetric) GetFieldInt64(field string) (int64, error) {
	if i == nil {
		return 0, common.ErrFieldNotFound
	}
	switch field {
	case "Fragments":
		return i.Fragments, nil
	case "FragmentErrors":
		return i.FragmentErrors, nil
	default:
		return 0, common.ErrFieldNotFound
	}
}

// GetFieldInt64 returns the value of a TCPMetric field
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
	case "ABSegmentOutOfOrder":
		return i.ABSegmentOutOfOrder, nil
	case "ABSegmentSkipped":
		return i.ABSegmentSkipped, nil
	case "ABSegmentSkippedBytes":
		return i.ABSegmentSkippedBytes, nil
	case "ABPackets":
		return i.ABPackets, nil
	case "ABBytes":
		return i.ABBytes, nil
	case "ABSawStart":
		return i.ABSawStart, nil
	case "ABSawEnd":
		return i.ABSawEnd, nil
	case "BASegmentOutOfOrder":
		return i.BASegmentOutOfOrder, nil
	case "BASegmentSkipped":
		return i.BASegmentSkipped, nil
	case "BASegmentSkippedBytes":
		return i.BASegmentSkippedBytes, nil
	case "BAPackets":
		return i.BAPackets, nil
	case "BABytes":
		return i.BABytes, nil
	case "BASawStart":
		return i.BASawStart, nil
	case "BASawEnd":
		return i.BASawEnd, nil
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
	case "UDP", "TCP", "SCTP":
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
	case "TCPMetric":
		return f.TCPMetric.GetFieldInt64(fields[1])
	case "IPMetric":
		return f.IPMetric.GetFieldInt64(fields[1])
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

// GetFieldInterface returns the value of a Flow field
func (f *Flow) GetFieldInterface(field string) (_ interface{}, err error) {
	switch field {
	case "Metric":
		return f.Metric, nil
	case "LastUpdateMetric":
		return f.LastUpdateMetric, nil
	case "TCPMetric":
		return f.TCPMetric, nil
	case "Link":
		return f.Link, nil
	case "Network":
		return f.Network, nil
	case "ICMP":
		return f.ICMP, nil
	case "Transport":
		return f.Transport, nil
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
