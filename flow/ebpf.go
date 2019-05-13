// +build linux

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

package flow

import (
	"encoding/hex"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/skydive-project/skydive/common"
)

// #cgo CFLAGS: -I../probe/ebpf
// #include "flow.h"
import "C"

// EBPFFlow Wrapper type used for passing flows from probe to main agent routine
type EBPFFlow struct {
	start        time.Time
	last         time.Time
	kernFlow     *C.struct_flow
	startKTimeNs int64
	probeNodeTID string
}

// SetEBPFFlow initializes pre-allocated ebpfflow
func SetEBPFFlow(ebpfFlow *EBPFFlow, start time.Time, last time.Time, kernFlow unsafe.Pointer, startKTimeNs int64, probeNodeTID string) {
	ebpfFlow.start = start
	ebpfFlow.last = last
	ebpfFlow.kernFlow = (*C.struct_flow)(kernFlow)
	ebpfFlow.startKTimeNs = startKTimeNs
	ebpfFlow.probeNodeTID = probeNodeTID
}

//key has already been hashed by the module, just convert to string
func kernFlowKey(kernFlow *C.struct_flow) string {
	return hex.EncodeToString(C.GoBytes(unsafe.Pointer(&kernFlow.key), C.sizeof___u64))
}

func kernFlowKeyOuter(kernFlow *C.struct_flow) string {
	return hex.EncodeToString(C.GoBytes(unsafe.Pointer(&kernFlow.key_outer), C.sizeof___u64))
}

func tcpFlagTime(currFlagTime C.__u64, startKTimeNs int64, start time.Time) int64 {
	if currFlagTime == 0 {
		return 0
	}
	return common.UnixMillis(start.Add(time.Duration(int64(currFlagTime) - startKTimeNs)))
}

func kernLayersPath(kernFlow *C.struct_flow) (layersPath string, hasGRE bool) {
	notFirst := false
	path := uint64(kernFlow.layers_path)
	for i := C.LAYERS_PATH_LEN - 1; i >= 0; i-- {
		layer := (path & (uint64(C.LAYERS_PATH_MASK) << uint(i*C.LAYERS_PATH_SHIFT))) >> uint(i*C.LAYERS_PATH_SHIFT)
		if layer == 0 {
			continue
		}
		if notFirst {
			layersPath += "/"
		}
		notFirst = true
		switch layer {
		case C.ETH_LAYER:
			layersPath += "Ethernet"
		case C.ARP_LAYER:
			layersPath += "ARP"
		case C.DOT1Q_LAYER:
			layersPath += "Dot1Q"
		case C.IP4_LAYER:
			layersPath += "IPv4"
		case C.IP6_LAYER:
			layersPath += "IPv6"
		case C.ICMP4_LAYER:
			layersPath += "ICMPv4"
		case C.ICMP6_LAYER:
			layersPath += "ICMPv6"
		case C.UDP_LAYER:
			layersPath += "UDP"
		case C.TCP_LAYER:
			layersPath += "TCP"
		case C.SCTP_LAYER:
			layersPath += "SCTP"
		case C.GRE_LAYER:
			hasGRE = true
			layersPath += "GRE"
		default:
			layersPath += "Unknown"
		}
	}
	return layersPath, hasGRE
}

func (ft *Table) newFlowFromEBPF(ebpfFlow *EBPFFlow, key string) ([]string, []*Flow) {
	var flows []*Flow
	var keys []string
	f := NewFlow()
	f.Init(common.UnixMillis(ebpfFlow.start), ebpfFlow.probeNodeTID, UUIDs{})
	f.Last = common.UnixMillis(ebpfFlow.last)

	layersInfo := uint8(ebpfFlow.kernFlow.layers_info)

	// LINK
	if layersInfo&uint8(C.LINK_LAYER_INFO) > 0 {
		linkA := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.link_layer.mac_src[0]), C.ETH_ALEN)
		linkB := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.link_layer.mac_dst[0]), C.ETH_ALEN)
		f.Link = &FlowLayer{
			Protocol: FlowProtocol_ETHERNET,
			A:        net.HardwareAddr(linkA).String(),
			B:        net.HardwareAddr(linkB).String(),
			ID:       int64(ebpfFlow.kernFlow.link_layer.id),
		}
	}

	var hasGRE bool
	f.LayersPath, hasGRE = kernLayersPath(ebpfFlow.kernFlow)

	if hasGRE {
		pathSplitAt := strings.Index(f.LayersPath, "/GRE/")
		innerLayerPath := f.LayersPath[pathSplitAt+len("/GRE/"):]

		parent := f
		parent.LayersPath = f.LayersPath[:pathSplitAt] + "/GRE"

		// NETWORK
		if layersInfo&uint8(C.NETWORK_LAYER_INFO) > 0 {
			protocol := uint16(ebpfFlow.kernFlow.network_layer_outer.protocol)

			switch protocol {
			case syscall.ETH_P_IP:
				netA := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.network_layer_outer.ip_src[12]), net.IPv4len)
				netB := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.network_layer_outer.ip_dst[12]), net.IPv4len)
				parent.Network = &FlowLayer{
					Protocol: FlowProtocol_IPV4,
					A:        net.IP(netA).To4().String(),
					B:        net.IP(netB).To4().String(),
				}
			case syscall.ETH_P_IPV6:
				netA := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.network_layer_outer.ip_src[0]), net.IPv6len)
				netB := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.network_layer_outer.ip_dst[0]), net.IPv6len)
				parent.Network = &FlowLayer{
					Protocol: FlowProtocol_IPV6,
					A:        net.IP(netA).String(),
					B:        net.IP(netB).String(),
				}
			}
		}
		pkey := kernFlowKeyOuter(ebpfFlow.kernFlow)
		parent.UpdateUUID(pkey, Opts{LayerKeyMode: L3PreferedKeyMode})
		flows = append(flows, parent)
		keys = append(keys, pkey)
		// upper layer
		f = NewFlow()
		f.Init(common.UnixMillis(ebpfFlow.start), ebpfFlow.probeNodeTID, UUIDs{ParentUUID: parent.UUID})
		f.Last = common.UnixMillis(ebpfFlow.last)
		f.LayersPath = innerLayerPath
	}

	// NETWORK
	if layersInfo&uint8(C.NETWORK_LAYER_INFO) > 0 {
		protocol := uint16(ebpfFlow.kernFlow.network_layer.protocol)

		switch protocol {
		case syscall.ETH_P_IP:
			netA := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.network_layer.ip_src[12]), net.IPv4len)
			netB := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.network_layer.ip_dst[12]), net.IPv4len)
			f.Network = &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        net.IP(netA).To4().String(),
				B:        net.IP(netB).To4().String(),
			}
		case syscall.ETH_P_IPV6:
			netA := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.network_layer.ip_src[0]), net.IPv6len)
			netB := C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.network_layer.ip_dst[0]), net.IPv6len)
			f.Network = &FlowLayer{
				Protocol: FlowProtocol_IPV6,
				A:        net.IP(netA).String(),
				B:        net.IP(netB).String(),
			}
		}
	}

	// TRANSPORT
	if layersInfo&uint8(C.TRANSPORT_LAYER_INFO) > 0 {
		portA := int64(ebpfFlow.kernFlow.transport_layer.port_src)
		portB := int64(ebpfFlow.kernFlow.transport_layer.port_dst)
		protocol := uint8(ebpfFlow.kernFlow.transport_layer.protocol)

		switch protocol {
		case syscall.IPPROTO_UDP:
			f.Transport = &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        portA,
				B:        portB,
			}

			/* disabled for now as no payload is sent
			p := gopacket.NewPacket(C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.payload[0]), C.PAYLOAD_LENGTH), layers.LayerTypeUDP, gopacket.DecodeOptions{})
			if p.Layer(gopacket.LayerTypeDecodeFailure) == nil {
				path, app := LayersPath(p.Layers())
				f.LayersPath += "/" + path
				f.Application = app
			}
			*/
		case syscall.IPPROTO_TCP:
			f.Transport = &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        portA,
				B:        portB,
			}
			f.TCPMetric = &TCPMetric{
				ABSynStart: tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ab_syn, ebpfFlow.startKTimeNs, ebpfFlow.start),
				BASynStart: tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ba_syn, ebpfFlow.startKTimeNs, ebpfFlow.start),
				ABFinStart: tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ab_fin, ebpfFlow.startKTimeNs, ebpfFlow.start),
				BAFinStart: tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ba_fin, ebpfFlow.startKTimeNs, ebpfFlow.start),
				ABRstStart: tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ab_rst, ebpfFlow.startKTimeNs, ebpfFlow.start),
				BARstStart: tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ba_rst, ebpfFlow.startKTimeNs, ebpfFlow.start),
			}
			/* disabled for now as no payload is sent
			p := gopacket.NewPacket(C.GoBytes(unsafe.Pointer(&ebpfFlow.kernFlow.payload[0]), C.PAYLOAD_LENGTH), layers.LayerTypeTCP, gopacket.DecodeOptions{})
			if p.Layer(gopacket.LayerTypeDecodeFailure) == nil {
				path, app := LayersPath(p.Layers())
				f.LayersPath += "/" + path
				f.Application = app
			}
			*/
		}
	}

	// ICMP
	if layersInfo&uint8(C.ICMP_LAYER_INFO) > 0 {
		kind := uint8(ebpfFlow.kernFlow.icmp_layer.kind)
		code := uint8(ebpfFlow.kernFlow.icmp_layer.code)
		id := uint16(ebpfFlow.kernFlow.icmp_layer.id)

		f.ICMP = &ICMPLayer{
			Code: uint32(code),
			ID:   uint32(id),
		}
		if f.Network != nil && f.Network.Protocol == FlowProtocol_IPV4 {
			f.ICMP.Type = ICMPv4TypeToFlowICMPType(kind)
		} else {
			f.ICMP.Type = ICMPv6TypeToFlowICMPType(kind)
		}
	}

	appLayers := strings.Split(f.LayersPath, "/")
	f.Application = appLayers[len(appLayers)-1]

	f.Metric = &FlowMetric{
		ABBytes:   int64(ebpfFlow.kernFlow.metrics.ab_bytes),
		ABPackets: int64(ebpfFlow.kernFlow.metrics.ab_packets),
		BABytes:   int64(ebpfFlow.kernFlow.metrics.ba_bytes),
		BAPackets: int64(ebpfFlow.kernFlow.metrics.ba_packets),
		Start:     f.Start,
		Last:      f.Last,
	}

	f.UpdateUUID(key, Opts{LayerKeyMode: L3PreferedKeyMode})

	flows = append(flows, f)
	keys = append(keys, key)
	return keys, flows
}

func (ft *Table) updateFlowFromEBPF(ebpfFlow *EBPFFlow, f *Flow) *Flow {
	f.Last = common.UnixMillis(ebpfFlow.last)
	layersInfo := uint8(ebpfFlow.kernFlow.layers_info)
	if layersInfo&uint8(C.TRANSPORT_LAYER_INFO) > 0 {
		protocol := uint8(ebpfFlow.kernFlow.transport_layer.protocol)
		switch protocol {
		case syscall.IPPROTO_TCP:
			f.TCPMetric.ABSynStart = tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ab_syn, ebpfFlow.startKTimeNs, ebpfFlow.start)
			f.TCPMetric.BASynStart = tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ba_syn, ebpfFlow.startKTimeNs, ebpfFlow.start)
			f.TCPMetric.ABFinStart = tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ab_fin, ebpfFlow.startKTimeNs, ebpfFlow.start)
			f.TCPMetric.BAFinStart = tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ba_fin, ebpfFlow.startKTimeNs, ebpfFlow.start)
			f.TCPMetric.ABRstStart = tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ab_rst, ebpfFlow.startKTimeNs, ebpfFlow.start)
			f.TCPMetric.BARstStart = tcpFlagTime(ebpfFlow.kernFlow.transport_layer.ba_rst, ebpfFlow.startKTimeNs, ebpfFlow.start)
		}
	}
	f.Metric.ABBytes = int64(ebpfFlow.kernFlow.metrics.ab_bytes)
	f.Metric.ABPackets = int64(ebpfFlow.kernFlow.metrics.ab_packets)
	f.Metric.BABytes = int64(ebpfFlow.kernFlow.metrics.ba_bytes)
	f.Metric.BAPackets = int64(ebpfFlow.kernFlow.metrics.ba_packets)
	f.Metric.Start = f.Start
	f.Metric.Last = f.Last

	return f
}
