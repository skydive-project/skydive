/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package flow

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	fmt "fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/pierrec/xxHash/xxHash64"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	fl "github.com/skydive-project/skydive/flow/layers"
	"github.com/skydive-project/skydive/graffiti/logging"
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
	extKey        interface{} // used to make a link between internal flow to external flow, ex: eBPF
	lastMetric    FlowMetric
	rtt1stPacket  int64
	updateVersion int64
	ipv4          *layers.IPv4
	ipv6          *layers.IPv6
}

// Packet describes one packet
type Packet struct {
	GoPacket gopacket.Packet // orignal gopacket

	Layers   []gopacket.Layer // layer of the sub packet
	Data     []byte           // byte of the sub packet
	Length   int64            // length of the original packet meaning layers + payload
	IPMetric *IPMetric

	linkLayer      gopacket.LinkLayer      // fast access to link layer
	networkLayer   gopacket.NetworkLayer   // fast access to network layer
	transportLayer gopacket.TransportLayer // fast access to transport layer
}

// PacketSequence represents a suite of parent/child Packet
type PacketSequence struct {
	Packets []*Packet
}

// LayerKeyMode defines what are the layers used for the flow key calculation
type LayerKeyMode int

// Flow key calculation modes
const (
	DefaultLayerKeyMode              = L2KeyMode // default mode
	L2KeyMode           LayerKeyMode = 0         // uses Layer2 and Layer3 for hash computation, default mode
	L3PreferredKeyMode  LayerKeyMode = 1         // uses Layer3 only and layer2 if no Layer3
)

// ExtraLayers defines extra layer to be pushed in flow
type ExtraLayers int

const (
	// VRRPLayer extra layer
	VRRPLayer ExtraLayers = 1
	// DNSLayer extra layer
	DNSLayer ExtraLayers = 2
	// DHCPv4Layer extra layer
	DHCPv4Layer ExtraLayers = 4
	// ALLLayer all extra layers
	ALLLayer ExtraLayers = 255
)

var extraLayersMap = map[string]ExtraLayers{
	"VRRP":   VRRPLayer,
	"DNS":    DNSLayer,
	"DHCPv4": DHCPv4Layer,
}

// Parse set the ExtraLayers struct with the given list of protocol strings
func (e *ExtraLayers) Parse(s ...string) error {
	*e = 0
	for _, v := range s {
		if i, ok := extraLayersMap[v]; ok {
			*e |= i
		} else {
			return fmt.Errorf("Extra layer not supported: %s", v)
		}
	}
	return nil
}

// Extract returns a string list of the ExtraLayers protocol
func (e ExtraLayers) Extract() []string {
	var l []string
	for k, v := range extraLayersMap {
		if (e & v) > 0 {
			l = append(l, k)
		}
	}
	return l
}

// String returns a string of the list of the protocols
func (e ExtraLayers) String() string {
	return strings.Join(e.Extract(), "|")
}

// MarshalJSON serializes the ExtraLayers structure
func (e ExtraLayers) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.Extract())
}

// UnmarshalJSON deserializes json input to ExtraLayers
func (e *ExtraLayers) UnmarshalJSON(data []byte) error {
	var a []string
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	return e.Parse(a...)
}

// Opts describes options that can be used to process flows
type Opts struct {
	TCPMetric    bool
	IPDefrag     bool
	LayerKeyMode LayerKeyMode
	AppPortMap   *ApplicationPortMap
	ExtraLayers  ExtraLayers
}

func (l LayerKeyMode) String() string {
	if l == L2KeyMode {
		return "L2"
	}
	return "L3"
}

// LayerKeyModeByName converts a string to a layer key mode
func LayerKeyModeByName(name string) (LayerKeyMode, error) {
	switch name {
	case "L2":
		return L2KeyMode, nil
	case "L3":
		return L3PreferredKeyMode, nil
	}
	return L2KeyMode, errors.New("LayerKeyMode unknown")
}

// DefaultLayerKeyModeName returns the default layer key mode
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
	value32 := make([]byte, 4)
	if layer := p.Layer(layers.LayerTypeICMPv4); layer != nil {
		l := layer.(*layers.ICMPv4)
		t := ICMPv4TypeToFlowICMPType(l.TypeCode.Type())
		binary.BigEndian.PutUint32(value32, uint32(t)<<24|uint32(l.TypeCode.Code())<<16|uint32(l.Id))
		return gopacket.NewFlow(0, value32, nil), nil
	} else if layer := p.Layer(layers.LayerTypeICMPv6Echo); layer != nil {
		l := layer.(*layers.ICMPv6Echo)
		binary.BigEndian.PutUint32(value32, uint32(ICMPType_ECHO)<<24|uint32(l.Identifier))
		return gopacket.NewFlow(0, value32, nil), nil
	} else if layer := p.Layer(layers.LayerTypeICMPv6); layer != nil {
		l := layer.(*layers.ICMPv6)
		t := ICMPv6TypeToFlowICMPType(l.TypeCode.Type())
		binary.BigEndian.PutUint32(value32, uint32(t)<<24|uint32(l.TypeCode.Code())<<16)
		return gopacket.NewFlow(0, value32, nil), nil
	}

	return gopacket.Flow{}, ErrLayerNotFound
}

// TransportFlow returns first transport flow
func (p *Packet) TransportFlow(swap bool) (gopacket.Flow, error) {
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

			if swap {
				return gopacket.NewFlow(0, value16, value32), nil
			}
			return gopacket.NewFlow(0, value32, value16), nil
		}
	}

	return layer.TransportFlow(), nil
}

// Keys returns keys of the packet
func (p *Packet) Keys(parentUUID string, uuids *UUIDs, opts *Opts) (uint64, uint64, uint64) {
	hasher := xxHash64.New(0)
	swap := false

	l2Layer := p.LinkLayer()
	l3Layer := p.NetworkLayer()
	swapFromNetwork := true

	if (opts.LayerKeyMode == L2KeyMode && l2Layer != nil) ||
		l3Layer == nil {
		swapFromNetwork = false
		src, dst := l2Layer.LinkFlow().Endpoints()
		cmp := bytes.Compare(src.Raw(), dst.Raw())
		swap = cmp > 0
		if cmp == 0 && l3Layer != nil {
			swapFromNetwork = true
		}
	}
	if swapFromNetwork {
		src, dst := l3Layer.NetworkFlow().Endpoints()
		cmp := bytes.Compare(src.Raw(), dst.Raw())
		swap = cmp > 0
		if cmp == 0 {
			if tf, err := p.TransportFlow(false); err == nil {
				src, dst := tf.Endpoints()
				swap = bytes.Compare(src.Raw(), dst.Raw()) > 0
			}
		}
	}

	if l3Layer != nil {
		hashFlow(l3Layer.NetworkFlow(), hasher, swap)
	}
	if tf, err := p.TransportFlow(swap); err == nil {
		hashFlow(tf, hasher, swap)
	}
	if af, err := p.ApplicationFlow(); err == nil {
		src, dst := af.Endpoints()
		hashFlow(af, hasher, bytes.Compare(src.Raw(), dst.Raw()) > 0)
	}
	l3Key := hasher.Sum64()
	l2Key := l3Key

	// uses L2 is requested or if there is no network layer
	if (opts.LayerKeyMode == L2KeyMode && p.LinkLayer() != nil) || p.NetworkLayer() == nil {
		if layer := p.LinkLayer(); layer != nil {
			hashFlow(layer.LinkFlow(), hasher, swap)
			l2Key = hasher.Sum64()
		}
	}

	hasher.Write([]byte(parentUUID))

	return hasher.Sum64(), l2Key, l3Key
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
	var app string
	var path strings.Builder

	for i, layer := range ls {
		tp := layer.LayerType()
		if tp == layers.LayerTypeLinuxSLL {
			continue
		} else if tp == gopacket.LayerTypePayload || tp == gopacket.LayerTypeDecodeFailure {
			break
		}
		if i > 0 {
			path.WriteString("/")
		}
		app = layer.LayerType().String()
		path.WriteString(app)
	}
	return path.String(), app
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
func NewFlowFromGoPacket(p gopacket.Packet, parentUUID string, uuids *UUIDs, opts *Opts) *Flow {
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

	key, l2Key, l3Key := packet.Keys(parentUUID, uuids, opts)

	f.initFromPacket(key, l2Key, l3Key, packet, parentUUID, uuids, opts)

	return f
}

// setUUIDs set the flow UUIDs based on keys generated from gopacket flow
// l2Key stands for layer2 and beyond
// l3Key stands for layer3 and beyond
func (f *Flow) setUUIDs(key, l2Key, l3Key uint64) {
	f.TrackingID = strconv.FormatUint(l2Key, 16)
	f.L3TrackingID = strconv.FormatUint(l3Key, 16)

	value64 := make([]byte, 8)
	binary.BigEndian.PutUint64(value64, uint64(f.Start))

	hasher := xxHash64.New(key)
	hasher.Write(value64)
	hasher.Write([]byte(f.NodeTID))

	f.UUID = strconv.FormatUint(hasher.Sum64(), 16)
}

// SetUUIDs updates the UUIDs using the flow layers and returns l2/l3 keys
func (f *Flow) SetUUIDs(key uint64, opts Opts) (uint64, uint64) {
	hasher := xxHash64.New(0)
	swap := false
	if f.Link != nil && ((opts.LayerKeyMode == L2KeyMode &&
		strings.Compare(f.Link.A, f.Link.B) != 0) ||
		f.Network == nil) {

		swap = strings.Compare(f.Link.A, f.Link.B) > 0
	} else {
		if cmp := strings.Compare(f.Network.A, f.Network.B); cmp == 0 && f.Transport != nil {
			swap = f.Transport.A > f.Transport.B
		} else {
			swap = cmp > 0
		}
	}
	f.Network.Hash(hasher, swap)
	f.Transport.Hash(hasher, swap)
	f.ICMP.Hash(hasher)

	l3Key := hasher.Sum64()
	l2Key := l3Key

	if (opts.LayerKeyMode == L2KeyMode && f.Link != nil) || f.Network == nil {
		f.Link.Hash(hasher, swap)
		l2Key = hasher.Sum64()
	}

	f.setUUIDs(key, l2Key, l3Key)

	return l2Key, l3Key
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

// Init initializes the flow with the given Timestamp, parentUUID and table uuids
func (f *Flow) Init(now int64, parentUUID string, uuids *UUIDs) {
	f.Start = now
	f.Last = now

	f.Metric.Start = now
	f.Metric.Last = now

	f.NodeTID = uuids.NodeTID
	f.CaptureID = uuids.CaptureID
	f.ParentUUID = parentUUID

	f.FinishType = FlowFinishType_NOT_FINISHED
}

// initFromPacket initializes the flow based on packet data, flow key and ids
func (f *Flow) initFromPacket(key, l2Key, l3Key uint64, packet *Packet, parentUUID string, uuids *UUIDs, opts *Opts) {
	now := common.UnixMillis(packet.GoPacket.Metadata().CaptureInfo.Timestamp)
	f.Init(now, parentUUID, uuids)

	f.newLinkLayer(packet)

	f.LayersPath, f.Application = LayersPath(packet.Layers)

	// no network layer then no transport layer
	if err := f.newNetworkLayer(packet); err == nil {
		f.newTransportLayer(packet, opts)
	}

	// add optional application layer
	f.newApplicationLayer(packet, opts)

	// need to have as most variable filled as possible to get correct UUID
	f.setUUIDs(key, l2Key, l3Key)

	// update metrics
	f.Update(packet, opts)
}

// Update a flow metrics and latency
func (f *Flow) Update(packet *Packet, opts *Opts) {
	now := common.UnixMillis(packet.GoPacket.Metadata().CaptureInfo.Timestamp)
	f.Last = now
	f.Metric.Last = now

	if opts.LayerKeyMode == L3PreferredKeyMode {
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

	f.updateRTT(packet)

	// depends on options
	if f.TCPMetric != nil {
		f.updateTCPMetrics(packet)
	}
	if (opts.ExtraLayers & DNSLayer) != 0 {
		if layer := packet.Layer(layers.LayerTypeDNS); layer != nil {
			f.updateDNSLayer(layer, packet.GoPacket.Metadata().CaptureInfo.Timestamp)
		}
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

func (f *Flow) updateRTT(packet *Packet) {
	icmpLayer := packet.Layer(layers.LayerTypeICMPv4)
	if icmp, ok := icmpLayer.(*layers.ICMPv4); ok {
		switch icmp.TypeCode.Type() {
		case layers.ICMPv4TypeEchoRequest:
			if f.XXX_state.rtt1stPacket == 0 {
				f.XXX_state.rtt1stPacket = packet.GoPacket.Metadata().Timestamp.UnixNano()
			}
		case layers.ICMPv4TypeEchoReply:
			if f.XXX_state.rtt1stPacket != 0 {
				f.Metric.RTT = packet.GoPacket.Metadata().Timestamp.UnixNano() - f.XXX_state.rtt1stPacket
				f.XXX_state.rtt1stPacket = 0
			}
		}

		return
	}

	icmpLayer = packet.Layer(layers.LayerTypeICMPv6)
	if icmp, ok := icmpLayer.(*layers.ICMPv6); ok {
		switch icmp.TypeCode.Type() {
		case layers.ICMPv6TypeEchoRequest:
			if f.XXX_state.rtt1stPacket == 0 {
				f.XXX_state.rtt1stPacket = packet.GoPacket.Metadata().Timestamp.UnixNano()
			}
		case layers.ICMPv6TypeEchoReply:
			if f.XXX_state.rtt1stPacket != 0 {
				f.Metric.RTT = packet.GoPacket.Metadata().Timestamp.UnixNano() - f.XXX_state.rtt1stPacket
				f.XXX_state.rtt1stPacket = 0
			}
		}

		return
	}

	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpPacket, ok := tcpLayer.(*layers.TCP); ok {
		if tcpPacket.SYN && f.XXX_state.rtt1stPacket == 0 {
			f.XXX_state.rtt1stPacket = packet.GoPacket.Metadata().Timestamp.UnixNano()
		} else if f.XXX_state.rtt1stPacket != 0 && ((tcpPacket.SYN && tcpPacket.ACK) || tcpPacket.RST) {
			f.Metric.RTT = packet.GoPacket.Metadata().Timestamp.UnixNano() - f.XXX_state.rtt1stPacket
			f.XXX_state.rtt1stPacket = 0
		}
	}
}

func (f *Flow) getNetworkLayer(packet *Packet) (*layers.IPv4, *layers.IPv6) {
	if f.XXX_state.ipv4 != nil {
		return f.XXX_state.ipv4, nil
	}
	if f.XXX_state.ipv6 != nil {
		return nil, f.XXX_state.ipv6
	}

	ipv4Layer := packet.Layer(layers.LayerTypeIPv4)
	if ipv4Packet, ok := ipv4Layer.(*layers.IPv4); ok {
		f.XXX_state.ipv4 = ipv4Packet
		return f.XXX_state.ipv4, nil
	}

	ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
	if ipv6Packet, ok := ipv6Layer.(*layers.IPv6); ok {
		f.XXX_state.ipv6 = ipv6Packet
		return nil, f.XXX_state.ipv6
	}

	return nil, nil
}

func (f *Flow) isABPacket(packet *Packet) bool {
	if f.Network == nil {
		return false
	}
	cmp := false

	ipv4Packet, ipv6Packet := f.getNetworkLayer(packet)
	if ipv4Packet != nil {
		cmp = f.Network.A == ipv4Packet.SrcIP.String()
		if bytes.Compare(ipv4Packet.SrcIP, ipv4Packet.DstIP) == 0 && f.Transport != nil {
			tcpLayer := packet.Layer(layers.LayerTypeTCP)
			if tcpPacket, ok := tcpLayer.(*layers.TCP); ok {
				return f.Transport.A > int64(tcpPacket.SrcPort)
			}
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			if udpPacket, ok := udpLayer.(*layers.UDP); ok {
				return f.Transport.A > int64(udpPacket.SrcPort)
			}
			sctpLayer := packet.Layer(layers.LayerTypeSCTP)
			if sctpPacket, ok := sctpLayer.(*layers.SCTP); ok {
				return f.Transport.A > int64(sctpPacket.SrcPort)
			}
			icmpLayer := packet.Layer(layers.LayerTypeICMPv4)
			if icmpPacket, ok := icmpLayer.(*layers.ICMPv4); ok {
				return f.ICMP.Type > ICMPv4TypeToFlowICMPType(icmpPacket.TypeCode.Type())
			}
		}
	}
	if ipv6Packet != nil {
		cmp = f.Network.A == ipv6Packet.SrcIP.String()
		if bytes.Compare(ipv6Packet.SrcIP, ipv6Packet.DstIP) == 0 && f.Transport != nil {
			tcpLayer := packet.Layer(layers.LayerTypeTCP)
			if tcpPacket, ok := tcpLayer.(*layers.TCP); ok {
				return f.Transport.A > int64(tcpPacket.SrcPort)
			}
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			if udpPacket, ok := udpLayer.(*layers.UDP); ok {
				return f.Transport.A > int64(udpPacket.SrcPort)
			}
			sctpLayer := packet.Layer(layers.LayerTypeSCTP)
			if sctpPacket, ok := sctpLayer.(*layers.SCTP); ok {
				return f.Transport.A > int64(sctpPacket.SrcPort)
			}
			icmpLayer := packet.Layer(layers.LayerTypeICMPv6)
			if icmpPacket, ok := icmpLayer.(*layers.ICMPv6); ok {
				return f.ICMP.Type > ICMPv6TypeToFlowICMPType(icmpPacket.TypeCode.Type())
			}
		}
	}

	return cmp
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

	/* found a layer that can identify the connection way */
	cmp := f.Link.A == ethernetPacket.SrcMAC.String()
	if bytes.Compare(ethernetPacket.SrcMAC, ethernetPacket.DstMAC) == 0 {
		cmp = f.isABPacket(packet)
	}
	if cmp {
		f.Metric.ABPackets++
		f.Metric.ABBytes += length
	} else {
		f.Metric.BAPackets++
		f.Metric.BABytes += length
	}

	return true
}

func (f *Flow) newNetworkLayer(packet *Packet) error {
	ipv4Packet, ipv6Packet := f.getNetworkLayer(packet)
	if ipv4Packet != nil {
		f.Network = &FlowLayer{
			Protocol: FlowProtocol_IPV4,
			A:        ipv4Packet.SrcIP.String(),
			B:        ipv4Packet.DstIP.String(),
			ID:       networkID(packet),
		}
		f.IPMetric = packet.IPMetric

		icmpLayer := packet.Layer(layers.LayerTypeICMPv4)
		if icmp, ok := icmpLayer.(*layers.ICMPv4); ok {
			f.ICMP = &ICMPLayer{
				Type: ICMPv4TypeToFlowICMPType(icmp.TypeCode.Type()),
				Code: uint32(icmp.TypeCode.Code()),
				ID:   uint32(icmp.Id),
			}
		}
		return nil
	}

	if ipv6Packet != nil {
		f.Network = &FlowLayer{
			Protocol: FlowProtocol_IPV6,
			A:        ipv6Packet.SrcIP.String(),
			B:        ipv6Packet.DstIP.String(),
			ID:       networkID(packet),
		}

		icmpLayer := packet.Layer(layers.LayerTypeICMPv6)
		if icmp, ok := icmpLayer.(*layers.ICMPv6); ok {
			t := ICMPv6TypeToFlowICMPType(icmp.TypeCode.Type())
			f.ICMP = &ICMPLayer{
				Type: t,
				Code: uint32(icmp.TypeCode.Code()),
			}

			if t == ICMPType_ECHO {
				echoLayer := packet.Layer(layers.LayerTypeICMPv6Echo)
				if echo, ok := echoLayer.(*layers.ICMPv6Echo); ok {
					f.ICMP.ID = uint32(echo.Identifier)
				}
			}
		}
		return nil
	}

	return ErrLayerNotFound
}

func (f *Flow) updateMetricsWithNetworkLayer(packet *Packet, length int64) error {
	ipv4Packet, ipv6Packet := f.getNetworkLayer(packet)
	if ipv4Packet != nil {
		if length == 0 {
			length = int64(ipv4Packet.Length)
		}
		if f.isABPacket(packet) {
			f.Metric.ABPackets++
			f.Metric.ABBytes += length
		} else {
			f.Metric.BAPackets++
			f.Metric.BABytes += length
		}

		return nil
	}
	if ipv6Packet != nil {
		if length == 0 {
			length = int64(ipv6Packet.Length)
		}
		if f.isABPacket(packet) {
			f.Metric.ABPackets++
			f.Metric.ABBytes += length
		} else {
			f.Metric.BAPackets++
			f.Metric.BABytes += length
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
	ipv4Packet, ipv6Packet := f.getNetworkLayer(packet)
	switch f.Network.Protocol {
	case FlowProtocol_IPV4:
		if ipv4Packet == nil {
			return ErrLayerNotFound
		}
		srcIP = ipv4Packet.SrcIP.String()
		timeToLive = uint32(ipv4Packet.TTL)
	case FlowProtocol_IPV6:
		if ipv6Packet == nil {
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
		if f.Network.A == srcIP && f.TCPMetric.ABSynStart == 0 {
			f.TCPMetric.ABSynStart = captureTime
			f.TCPMetric.ABSynTTL = timeToLive
		} else if f.TCPMetric.BASynStart == 0 {
			f.TCPMetric.BASynStart = captureTime
			f.TCPMetric.BASynTTL = timeToLive
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

func (f *Flow) newTransportLayer(packet *Packet, opts *Opts) error {
	if layer := packet.Layer(layers.LayerTypeTCP); layer != nil {
		f.Transport = &TransportLayer{Protocol: FlowProtocol_TCP}

		transportPacket := layer.(*layers.TCP)
		srcPort, dstPort := int(transportPacket.SrcPort), int(transportPacket.DstPort)
		f.Transport.A, f.Transport.B = int64(srcPort), int64(dstPort)

		if app, ok := opts.AppPortMap.tcpApplication(srcPort, dstPort); ok {
			f.Application = app
		}

		if opts.TCPMetric {
			f.TCPMetric = &TCPMetric{}
		}

		if transportPacket.FIN {
			f.FinishType = FlowFinishType_TCP_FIN
		} else if transportPacket.RST {
			f.FinishType = FlowFinishType_TCP_RST
		}
	} else if layer := packet.Layer(layers.LayerTypeUDP); layer != nil {
		f.Transport = &TransportLayer{Protocol: FlowProtocol_UDP}

		transportPacket := layer.(*layers.UDP)
		srcPort, dstPort := int(transportPacket.SrcPort), int(transportPacket.DstPort)
		f.Transport.A, f.Transport.B = int64(srcPort), int64(dstPort)

		if app, ok := opts.AppPortMap.udpApplication(srcPort, dstPort); ok {
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

func (f *Flow) updateDNSLayer(layer gopacket.Layer, timestamp time.Time) error {
	d := layer.(*layers.DNS)
	dnsToAppend := &fl.DNS{
		AA:           d.AA,
		ID:           d.ID,
		QR:           d.QR,
		RA:           d.RA,
		RD:           d.RD,
		TC:           d.TC,
		ANCount:      d.ANCount,
		ARCount:      d.ARCount,
		NSCount:      d.NSCount,
		QDCount:      d.QDCount,
		ResponseCode: d.ResponseCode.String(),
		OpCode:       d.OpCode.String(),
		Z:            d.Z,
	}
	dnsToAppend.Questions = fl.GetDNSQuestions(d.Questions)
	dnsToAppend.Answers = fl.GetDNSRecords(d.Answers)
	dnsToAppend.Authorities = fl.GetDNSRecords(d.Authorities)
	dnsToAppend.Additionals = fl.GetDNSRecords(d.Additionals)
	dnsToAppend.Timestamp = timestamp
	f.DNS = dnsToAppend
	return nil
}

func (f *Flow) newApplicationLayer(packet *Packet, opts *Opts) error {
	if (opts.ExtraLayers & DHCPv4Layer) != 0 {
		if layer := packet.Layer(layers.LayerTypeDHCPv4); layer != nil {
			d := layer.(*layers.DHCPv4)
			f.DHCPv4 = &fl.DHCPv4{
				File:         d.File,
				Flags:        d.Flags,
				HardwareLen:  d.HardwareLen,
				HardwareOpts: d.HardwareOpts,
				ServerName:   d.ServerName,
				Secs:         d.Secs,
			}
			return nil
		}
	}

	if (opts.ExtraLayers & DNSLayer) != 0 {
		if layer := packet.Layer(layers.LayerTypeDNS); layer != nil {
			f.updateDNSLayer(layer, packet.GoPacket.Metadata().CaptureInfo.Timestamp)
			return nil
		}
	}

	if (opts.ExtraLayers & VRRPLayer) != 0 {
		if layer := packet.Layer(layers.LayerTypeVRRP); layer != nil {
			d := layer.(*layers.VRRPv2)
			f.VRRPv2 = &fl.VRRPv2{
				AdverInt:     d.AdverInt,
				Priority:     d.Priority,
				Version:      d.Version,
				VirtualRtrID: d.VirtualRtrID,
			}
			return nil
		}
	}

	return nil
}

// ProcessGoPacket takes a gopacket as input and filter, defrag it.
func ProcessGoPacket(packet gopacket.Packet, bpf *BPF, defragger *IPDefragger) (gopacket.Packet, *IPMetric) {
	var ipMetric *IPMetric
	if defragger != nil {
		m, ok := defragger.Defrag(packet)
		if !ok {
			return nil, nil
		}
		ipMetric = m
	}

	if bpf != nil && !bpf.Matches(packet.Data()) {
		return nil, nil
	}

	return packet, ipMetric
}

// PacketSeqFromGoPacket split original packet into multiple packets in
// case of encapsulation like GRE, VXLAN, etc.
func PacketSeqFromGoPacket(packet gopacket.Packet, outerLength int64, bpf *BPF, defragger *IPDefragger) *PacketSequence {
	ps := &PacketSequence{}

	var ipMetric *IPMetric
	packet, ipMetric = ProcessGoPacket(packet, bpf, defragger)
	if packet == nil {
		return ps
	}

	if packet.ErrorLayer() != nil {
		logging.GetLogger().Debugf("Decoding or partial decoding error : %s\n", packet.Dump())
	}

	if packet.LinkLayer() == nil && packet.NetworkLayer() == nil {
		logging.GetLogger().Debugf("Unknown packet : %s\n", packet.Dump())
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

	packetData := packet.Data()

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
// contains multiple records which generate a Packets each.
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

// lookupPath lookup through the given obj according to the given path
// return the value found if the kind matches
func lookupPath(obj interface{}, path string, kind reflect.Kind) (reflect.Value, bool) {
	nodes := strings.Split(path, ".")

	var name string
	value := reflect.ValueOf(obj)

LOOP:
	for _, node := range nodes {
		name = node
		if value.Kind() == reflect.Struct {
			t := value.Type()

			for i := 0; i != t.NumField(); i++ {
				if t.Field(i).Name == node {
					value = value.Field(i)
					if value.Kind() == reflect.Interface || value.Kind() == reflect.Ptr {
						value = value.Elem()
					}

					continue LOOP
				}
			}
		} else {
			break LOOP
		}
	}

	if name != nodes[len(nodes)-1] {
		return value, false
	}

	if kind == reflect.Interface {
		return value, true
	}

	// convert result kind to int for all size of interger as then
	// only a int64 version will be retrieve by value.Int()
	rk := value.Kind()
	switch rk {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		rk = reflect.Int
	}

	return value, rk == kind
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
	case "CaptureID":
		return f.CaptureID, nil
	}

	// sub field
	if len(fields) != 2 {
		return "", common.ErrFieldNotFound
	}

	switch name {
	case "Link", "ETHERNET":
		if f.Link != nil {
			return f.Link.GetFieldString(fields[1])
		}
	case "Network", "IPV4", "IPV6":
		if f.Network != nil {
			return f.Network.GetFieldString(fields[1])
		}
	case "ICMP":
		if f.ICMP != nil {
			return f.ICMP.GetFieldString(fields[1])
		}
	case "Transport", "UDP", "TCP", "SCTP":
		if f.Transport != nil {
			return f.Transport.GetFieldString(fields[1])
		}
	}

	// check extra layers
	if _, ok := extraLayersMap[name]; ok {
		if value, ok := lookupPath(*f, field, reflect.String); ok {
			return value.String(), nil
		}
	}

	return "", common.ErrFieldNotFound
}

// GetFieldBool returns the value of a boolean flow field
func (f *Flow) GetFieldBool(field string) (bool, error) {
	return false, common.ErrFieldNotFound
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
		if f.Metric != nil {
			return f.Metric.GetFieldInt64(fields[1])
		}
	case "LastUpdateMetric":
		if f.LastUpdateMetric != nil {
			return f.LastUpdateMetric.GetFieldInt64(fields[1])
		}
	case "TCPMetric":
		if f.TCPMetric != nil {
			return f.TCPMetric.GetFieldInt64(fields[1])
		}
	case "IPMetric":
		if f.IPMetric != nil {
			return f.IPMetric.GetFieldInt64(fields[1])
		}
	case "Link":
		if f.Link != nil {
			return f.Link.GetFieldInt64(fields[1])
		}
	case "Network":
		if f.Network != nil {
			return f.Network.GetFieldInt64(fields[1])
		}
	case "ICMP":
		if f.ICMP != nil {
			return f.ICMP.GetFieldInt64(fields[1])
		}
	case "Transport":
		if f.Transport != nil {
			return f.Transport.GetFieldInt64(fields[1])
		}
	case "RawPacketsCaptured":
		return f.RawPacketsCaptured, nil
	}

	// check extra layers
	if _, ok := extraLayersMap[name]; ok {
		if value, ok := lookupPath(*f, field, reflect.Int); ok && value.IsValid() {
			if i, err := common.ToInt64(value.Interface()); err == nil {
				return i, nil
			}
		}
	}

	return 0, common.ErrFieldNotFound
}

// GetFieldInterface returns the value of a Flow field
func (f *Flow) getFieldInterface(field string) (_ interface{}, err error) {
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
	}

	// check extra layers
	if _, ok := extraLayersMap[field]; ok {
		if value, ok := lookupPath(*f, field, reflect.Struct); ok && value.IsValid() {
			return value.Interface(), nil
		}
	}

	return 0, common.ErrFieldNotFound
}

// GetField returns the value of a field
func (f *Flow) GetField(field string) (interface{}, error) {
	if i, err := f.getFieldInterface(field); err == nil {
		return i, nil
	}

	if i, err := f.GetFieldInt64(field); err == nil {
		return i, nil
	}

	return f.GetFieldString(field)
}

// GetFieldKeys returns the list of valid field of a Flow
func (f *Flow) GetFieldKeys() []string {
	return flowFieldKeys
}

// MatchBool implements the Getter interface
func (f *Flow) MatchBool(key string, predicate common.BoolPredicate) bool {
	if b, err := f.GetFieldBool(key); err == nil {
		return predicate(b)
	}
	return false
}

// MatchInt64 implements the Getter interface
func (f *Flow) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if i, err := f.GetFieldInt64(key); err == nil {
		return predicate(i)
	}
	return false
}

// MatchString implements the Getter interface
func (f *Flow) MatchString(key string, predicate common.StringPredicate) bool {
	if s, err := f.GetFieldString(key); err == nil {
		return predicate(s)
	}
	return false
}

var flowFieldKeys []string

func structFieldKeys(t reflect.Type, prefix string) []string {
	var fFields []string
	for i := 0; i < t.NumField(); i++ {
		vField := t.Field(i)
		tField := vField.Type

		// ignore XXX fields as they are considered as private
		if strings.HasPrefix(vField.Name, "XXX_") {
			continue
		}

		vName := prefix + vField.Name

		for tField.Kind() == reflect.Ptr {
			tField = tField.Elem()
		}

		switch tField.Kind() {
		case reflect.Struct:
			fFields = append(fFields, structFieldKeys(tField, vName+".")...)
		case reflect.Slice:
			fFields = append(fFields, vName)

			se := tField.Elem()
			if se.Kind() == reflect.Ptr {
				se = se.Elem()
			}

			if se.Kind() == reflect.Struct {
				fFields = append(fFields, structFieldKeys(se, vName+".")...)
			}
		default:
			fFields = append(fFields, vName)
		}
	}

	return fFields
}

func init() {
	flowFieldKeys = structFieldKeys(reflect.TypeOf(Flow{}), "")
}
