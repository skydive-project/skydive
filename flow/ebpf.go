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
	"bytes"
	"errors"
	"math/rand"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/skydive-project/skydive/common"
)

// #cgo CFLAGS: -I../ebpf
// #include "flow.h"
import "C"

// EBPFFlow Wrapper type used for passing flows from probe to main agent routine
type EBPFFlow struct {
	Start        time.Time
	Last         time.Time
	KernFlow     *C.struct_flow
	StartKTimeNs int64
}

// SetEBPFKernFlow is an helper function that aims to provide a way to set kernFlow from
// external packages as Go doesn't allow to acces to C struct from different packages.
func SetEBPFKernFlow(ebpfFlow *EBPFFlow, kernFlow unsafe.Pointer) {
	ebpfFlow.KernFlow = (*C.struct_flow)(kernFlow)
}

func kernFlowKey(kernFlow *C.struct_flow) uint64 {
	return uint64(kernFlow.key)
}

func tcpFlagTime(currFlagTime C.__u64, startKTimeNs int64, start time.Time) int64 {
	if currFlagTime == 0 {
		return 0
	}
	return common.UnixMillis(start.Add(time.Duration(int64(currFlagTime) - startKTimeNs)))
}

func kernLayersPath(kernFlow *C.struct_flow) (string, bool) {
	var layersPath strings.Builder
	var notFirst, hasGRE bool

	path := uint64(kernFlow.layers_path)

	for i := C.LAYERS_PATH_LEN - 1; i >= 0; i-- {
		layer := (path & (uint64(C.LAYERS_PATH_MASK) << uint(i*C.LAYERS_PATH_SHIFT))) >> uint(i*C.LAYERS_PATH_SHIFT)
		if layer == 0 {
			continue
		}
		if notFirst {
			layersPath.WriteString("/")
		} else {
			notFirst = true
		}

		switch layer {
		case C.ETH_LAYER:
			layersPath.WriteString("Ethernet")
		case C.ARP_LAYER:
			layersPath.WriteString("ARP")
		case C.DOT1Q_LAYER:
			layersPath.WriteString("Dot1Q")
		case C.IP4_LAYER:
			layersPath.WriteString("IPv4")
		case C.IP6_LAYER:
			layersPath.WriteString("IPv6")
		case C.ICMP4_LAYER:
			layersPath.WriteString("ICMPv4")
		case C.ICMP6_LAYER:
			layersPath.WriteString("ICMPv6")
		case C.UDP_LAYER:
			layersPath.WriteString("UDP")
		case C.TCP_LAYER:
			layersPath.WriteString("TCP")
		case C.SCTP_LAYER:
			layersPath.WriteString("SCTP")
		case C.GRE_LAYER:
			hasGRE = true
			layersPath.WriteString("GRE")
		default:
			layersPath.WriteString("Unknown")
		}
	}
	return layersPath.String(), hasGRE
}

func (ft *Table) newFlowFromEBPF(ebpfFlow *EBPFFlow, key uint64) ([]uint64, []*Flow, error) {
	var flows []*Flow
	var keys []uint64

	f := NewFlow()
	f.Init(common.UnixMillis(ebpfFlow.Start), "", &ft.uuids)
	f.Last = common.UnixMillis(ebpfFlow.Last)

	f.Metric = &FlowMetric{
		ABBytes:   int64(ebpfFlow.KernFlow.metrics.ab_bytes),
		ABPackets: int64(ebpfFlow.KernFlow.metrics.ab_packets),
		BABytes:   int64(ebpfFlow.KernFlow.metrics.ba_bytes),
		BAPackets: int64(ebpfFlow.KernFlow.metrics.ba_packets),
		Start:     f.Start,
		Last:      f.Last,
	}

	// Set the external key
	f.XXX_state.extKey = ebpfFlow.KernFlow.key

	// notify that the flow has been updated between two table updates
	f.XXX_state.updateVersion = ft.updateVersion + 1

	layersInfo := uint8(ebpfFlow.KernFlow.layers_info)

	// LINK
	if layersInfo&uint8(C.LINK_LAYER_INFO) > 0 {
		linkA := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.link_layer.mac_src[0]), C.ETH_ALEN)
		linkB := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.link_layer.mac_dst[0]), C.ETH_ALEN)
		f.Link = &FlowLayer{
			Protocol: FlowProtocol_ETHERNET,
			A:        net.HardwareAddr(linkA).String(),
			B:        net.HardwareAddr(linkB).String(),
			ID:       int64(ebpfFlow.KernFlow.link_layer.id),
		}
	}

	var hasGRE bool
	f.LayersPath, hasGRE = kernLayersPath(ebpfFlow.KernFlow)

	if hasGRE {
		pathSplitAt := strings.Index(f.LayersPath, "/GRE/")
		innerLayerPath := f.LayersPath[pathSplitAt+len("/GRE/"):]

		parent := f
		parent.LayersPath = f.LayersPath[:pathSplitAt] + "/GRE"
		parent.Application = "GRE"

		// NETWORK
		if layersInfo&uint8(C.NETWORK_LAYER_INFO) > 0 {
			protocol := uint16(ebpfFlow.KernFlow.network_layer_outer.protocol)

			switch protocol {
			case syscall.ETH_P_IP:
				netA := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer_outer.ip_src[12]), C.int(net.IPv4len))
				netB := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer_outer.ip_dst[12]), C.int(net.IPv4len))
				parent.Network = &FlowLayer{
					Protocol: FlowProtocol_IPV4,
					A:        net.IP(netA).To4().String(),
					B:        net.IP(netB).To4().String(),
				}
			case syscall.ETH_P_IPV6:
				netA := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer_outer.ip_src[0]), C.int(net.IPv6len))
				netB := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer_outer.ip_dst[0]), C.int(net.IPv6len))
				parent.Network = &FlowLayer{
					Protocol: FlowProtocol_IPV6,
					A:        net.IP(netA).String(),
					B:        net.IP(netB).String(),
				}
			}
		}

		if parent.Link == nil && parent.Network == nil {
			return nil, nil, errors.New("packet unknown, no link, no network layer")
		}

		parentKey := uint64(ebpfFlow.KernFlow.key_outer)

		parent.SetUUIDs(parentKey, Opts{LayerKeyMode: L3PreferredKeyMode})

		flows = append(flows, parent)
		keys = append(keys, parentKey)

		// inner layer
		f = NewFlow()
		f.Init(common.UnixMillis(ebpfFlow.Start), parent.UUID, &ft.uuids)
		f.Last = common.UnixMillis(ebpfFlow.Last)
		f.LayersPath = innerLayerPath
	}

	// NETWORK
	if layersInfo&uint8(C.NETWORK_LAYER_INFO) > 0 {
		protocol := uint16(ebpfFlow.KernFlow.network_layer.protocol)

		switch protocol {
		case syscall.ETH_P_IP:
			netA := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer.ip_src[12]), C.int(net.IPv4len))
			netB := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer.ip_dst[12]), C.int(net.IPv4len))
			f.Network = &FlowLayer{
				Protocol: FlowProtocol_IPV4,
				A:        net.IP(netA).To4().String(),
				B:        net.IP(netB).To4().String(),
			}
		case syscall.ETH_P_IPV6:
			netA := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer.ip_src[0]), C.int(net.IPv6len))
			netB := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer.ip_dst[0]), C.int(net.IPv6len))
			f.Network = &FlowLayer{
				Protocol: FlowProtocol_IPV6,
				A:        net.IP(netA).String(),
				B:        net.IP(netB).String(),
			}
		}
	}

	if f.Link == nil && f.Network == nil {
		return nil, nil, errors.New("packet unknown, no link, no network layer")
	}

	// TRANSPORT
	if layersInfo&uint8(C.TRANSPORT_LAYER_INFO) > 0 {
		portA := int64(ebpfFlow.KernFlow.transport_layer.port_src)
		portB := int64(ebpfFlow.KernFlow.transport_layer.port_dst)
		protocol := uint8(ebpfFlow.KernFlow.transport_layer.protocol)

		switch protocol {
		case syscall.IPPROTO_UDP:
			f.Transport = &TransportLayer{
				Protocol: FlowProtocol_UDP,
				A:        portA,
				B:        portB,
			}
		case syscall.IPPROTO_TCP:
			f.Transport = &TransportLayer{
				Protocol: FlowProtocol_TCP,
				A:        portA,
				B:        portB,
			}
			f.TCPMetric = &TCPMetric{
				ABSynStart: tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ab_syn, ebpfFlow.StartKTimeNs, ebpfFlow.Start),
				BASynStart: tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ba_syn, ebpfFlow.StartKTimeNs, ebpfFlow.Start),
				ABFinStart: tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ab_fin, ebpfFlow.StartKTimeNs, ebpfFlow.Start),
				BAFinStart: tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ba_fin, ebpfFlow.StartKTimeNs, ebpfFlow.Start),
				ABRstStart: tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ab_rst, ebpfFlow.StartKTimeNs, ebpfFlow.Start),
				BARstStart: tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ba_rst, ebpfFlow.StartKTimeNs, ebpfFlow.Start),
			}

			f.TCPMetric.ABBytes = f.Metric.ABBytes
			f.TCPMetric.BABytes = f.Metric.BABytes
			f.TCPMetric.ABPackets = f.Metric.ABPackets
			f.TCPMetric.BAPackets = f.Metric.BAPackets
		}
	}

	// ICMP
	if layersInfo&uint8(C.ICMP_LAYER_INFO) > 0 {
		kind := uint8(ebpfFlow.KernFlow.icmp_layer.kind)
		code := uint8(ebpfFlow.KernFlow.icmp_layer.code)
		id := uint16(ebpfFlow.KernFlow.icmp_layer.id)

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

	f.SetUUIDs(key, Opts{LayerKeyMode: L3PreferredKeyMode})

	flows = append(flows, f)
	keys = append(keys, key)

	return keys, flows, nil
}

func isABPacket(ebpfFlow *EBPFFlow, f *Flow) bool {
	cmp := true
	layersInfo := uint8(ebpfFlow.KernFlow.layers_info)
	if f.Link != nil {
		linkA := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.link_layer.mac_src[0]), C.ETH_ALEN)
		linkB := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.link_layer.mac_dst[0]), C.ETH_ALEN)
		cmp = strings.Compare(f.Link.A, net.HardwareAddr(linkA).String()) == 0 &&
			strings.Compare(f.Link.B, net.HardwareAddr(linkB).String()) == 0

		if bytes.Equal(linkA, linkB) {
			if f.Network != nil {
				netA := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer_outer.ip_src[0]), C.int(net.IPv6len))
				netB := C.GoBytes(unsafe.Pointer(&ebpfFlow.KernFlow.network_layer_outer.ip_dst[0]), C.int(net.IPv6len))
				cmp = strings.Compare(f.Network.A, net.IP(netA).String()) == 0 &&
					strings.Compare(f.Network.B, net.IP(netB).String()) == 0
				if bytes.Equal(netA, netB) {
					if layersInfo&uint8(C.TRANSPORT_LAYER_INFO) > 0 {
						portA := int64(ebpfFlow.KernFlow.transport_layer.port_src)
						portB := int64(ebpfFlow.KernFlow.transport_layer.port_dst)
						cmp = portA > portB
					}
				}
			}
		}
	}
	return cmp
}

func (ft *Table) updateFlowFromEBPF(ebpfFlow *EBPFFlow, f *Flow) bool {
	last := common.UnixMillis(ebpfFlow.Last)
	if last == f.Last {
		return false
	}
	f.Last = last

	isAB := isABPacket(ebpfFlow, f)
	if isAB {
		f.Metric.ABBytes += int64(ebpfFlow.KernFlow.metrics.ab_bytes)
		f.Metric.ABPackets += int64(ebpfFlow.KernFlow.metrics.ab_packets)
		f.Metric.BABytes += int64(ebpfFlow.KernFlow.metrics.ba_bytes)
		f.Metric.BAPackets += int64(ebpfFlow.KernFlow.metrics.ba_packets)
	} else {
		f.Metric.ABBytes += int64(ebpfFlow.KernFlow.metrics.ba_bytes)
		f.Metric.ABPackets += int64(ebpfFlow.KernFlow.metrics.ba_packets)
		f.Metric.BABytes += int64(ebpfFlow.KernFlow.metrics.ab_bytes)
		f.Metric.BAPackets += int64(ebpfFlow.KernFlow.metrics.ab_packets)
	}

	layersInfo := uint8(ebpfFlow.KernFlow.layers_info)
	if layersInfo&uint8(C.TRANSPORT_LAYER_INFO) > 0 {
		protocol := uint8(ebpfFlow.KernFlow.transport_layer.protocol)
		switch protocol {
		case syscall.IPPROTO_TCP:
			if isAB {
				f.TCPMetric.ABSynStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ab_syn, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.BASynStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ba_syn, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.ABFinStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ab_fin, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.BAFinStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ba_fin, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.ABRstStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ab_rst, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.BARstStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ba_rst, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
			} else {
				f.TCPMetric.ABSynStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ba_syn, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.BASynStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ab_syn, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.ABFinStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ba_fin, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.BAFinStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ab_fin, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.ABRstStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ba_rst, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
				f.TCPMetric.BARstStart = tcpFlagTime(ebpfFlow.KernFlow.transport_layer.ab_rst, ebpfFlow.StartKTimeNs, ebpfFlow.Start)
			}

			f.TCPMetric.ABBytes = f.Metric.ABBytes
			f.TCPMetric.BABytes = f.Metric.BABytes
			f.TCPMetric.ABPackets = f.Metric.ABPackets
			f.TCPMetric.BAPackets = f.Metric.BAPackets
		}
	}

	f.Metric.Start = f.Start
	f.Metric.Last = last

	return true
}

// because golang doesn't allow to use cgo in test we define this here but will be used in test
func newEBPFFlow(id uint32, linkSrc string, linkDst string) *EBPFFlow {
	ebpfFlow := new(EBPFFlow)
	kernFlow := new(C.struct_flow)

	now := time.Now()

	ebpfFlow.Start = now
	ebpfFlow.Last = now
	ebpfFlow.KernFlow = kernFlow
	ebpfFlow.StartKTimeNs = 0

	kernFlow.layers_info |= (1 << 3)
	kernFlow.icmp_layer.id = (C.__u32)(id)
	kernFlow.key = (C.__u64)(rand.Int())

	if linkSrc == "" && linkDst == "" {
		return ebpfFlow
	}
	l2 := C.struct_link_layer{}
	l2A, _ := net.ParseMAC(linkSrc)
	l2B, _ := net.ParseMAC(linkDst)
	for i := 0; i < 6 && i < len(l2A); i++ {
		l2.mac_src[i] = (C.__u8)(l2A[i])
	}
	for i := 0; i < 6 && i < len(l2B); i++ {
		l2.mac_dst[i] = (C.__u8)(l2B[i])
	}
	kernFlow.layers_info |= (1 << 0)
	kernFlow.link_layer = l2

	return ebpfFlow
}
