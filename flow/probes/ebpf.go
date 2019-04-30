// +build ebpf

/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package probes

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/iovisor/gobpf/elf"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/statics"
	"github.com/skydive-project/skydive/topology"
)

// #cgo CFLAGS: -I../../probe/ebpf
// #include "flow.h"
import "C"

const (
	BPF_ANY    = 0
	ebpfUpdate = 2 * time.Second
)

// EBPFFlow describes the userland side of an eBPF flow
type EBPFFlow struct {
	start time.Time
	last  time.Time
	lastK int64
}

// EBPFProbe the eBPF probe
type EBPFProbe struct {
	probeNodeTID string
	fd           int
	flowTable    *flow.Table
	module       *elf.Module
	fmap         *elf.Map
	expire       time.Duration
	quit         chan bool
}

// EBPFProbesHandler creates new eBPF probes
type EBPFProbesHandler struct {
	graph      *graph.Graph
	probes     map[graph.Identifier]*EBPFProbe
	probesLock common.RWMutex
	fpta       *FlowProbeTableAllocator
	wg         sync.WaitGroup
}

func kernFlowKeyOuter(kernFlow *C.struct_flow) string {
	hasher := sha1.New()
	hasher.Write(C.GoBytes(unsafe.Pointer(&kernFlow.key_outer), C.sizeof___u64))
	return hex.EncodeToString(hasher.Sum(nil))
}

func kernFlowKey(kernFlow *C.struct_flow) string {
	hasher := sha1.New()
	hasher.Write(C.GoBytes(unsafe.Pointer(&kernFlow.key), C.sizeof___u64))
	return hex.EncodeToString(hasher.Sum(nil))
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

func (p *EBPFProbe) newFlowOperation(ebpfFlow *EBPFFlow, kernFlow *C.struct_flow, startKTimeNs int64, start time.Time) []*flow.Operation {

	var fops []*flow.Operation
	f := flow.NewFlow()
	f.Init(common.UnixMillis(ebpfFlow.start), p.probeNodeTID, flow.UUIDs{})
	f.Last = common.UnixMillis(ebpfFlow.last)

	layersInfo := uint8(kernFlow.layers_info)

	// LINK
	if layersInfo&uint8(C.LINK_LAYER_INFO) > 0 {
		linkA := C.GoBytes(unsafe.Pointer(&kernFlow.link_layer.mac_src[0]), C.ETH_ALEN)
		linkB := C.GoBytes(unsafe.Pointer(&kernFlow.link_layer.mac_dst[0]), C.ETH_ALEN)
		f.Link = &flow.FlowLayer{
			Protocol: flow.FlowProtocol_ETHERNET,
			A:        net.HardwareAddr(linkA).String(),
			B:        net.HardwareAddr(linkB).String(),
			ID:       int64(kernFlow.link_layer.id),
		}
	}

	var hasGRE bool
	f.LayersPath, hasGRE = kernLayersPath(kernFlow)

	if hasGRE {
		pathSplitAt := strings.Index(f.LayersPath, "/GRE/")
		innerLayerPath := f.LayersPath[pathSplitAt+len("/GRE/"):]

		parent := f
		parent.LayersPath = f.LayersPath[:pathSplitAt] + "/GRE"

		// NETWORK
		if layersInfo&uint8(C.NETWORK_LAYER_INFO) > 0 {
			protocol := uint16(kernFlow.network_layer_outer.protocol)

			switch protocol {
			case syscall.ETH_P_IP:
				netA := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer_outer.ip_src[12]), net.IPv4len)
				netB := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer_outer.ip_dst[12]), net.IPv4len)
				parent.Network = &flow.FlowLayer{
					Protocol: flow.FlowProtocol_IPV4,
					A:        net.IP(netA).To4().String(),
					B:        net.IP(netB).To4().String(),
				}
			case syscall.ETH_P_IPV6:
				netA := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer_outer.ip_src[0]), net.IPv6len)
				netB := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer_outer.ip_dst[0]), net.IPv6len)
				parent.Network = &flow.FlowLayer{
					Protocol: flow.FlowProtocol_IPV6,
					A:        net.IP(netA).String(),
					B:        net.IP(netB).String(),
				}
			}
		}
		key := kernFlowKeyOuter(kernFlow)
		parent.UpdateUUID(key, flow.Opts{LayerKeyMode: flow.L3PreferedKeyMode})
		fops = append(fops, &flow.Operation{
			Type: flow.ReplaceOperation,
			Flow: parent,
			Key:  key,
		})
		// upper layer
		f = flow.NewFlow()
		f.Init(common.UnixMillis(ebpfFlow.start), p.probeNodeTID, flow.UUIDs{ParentUUID: parent.UUID})
		f.Last = common.UnixMillis(ebpfFlow.last)
		f.LayersPath = innerLayerPath
	}

	// NETWORK
	if layersInfo&uint8(C.NETWORK_LAYER_INFO) > 0 {
		protocol := uint16(kernFlow.network_layer.protocol)

		switch protocol {
		case syscall.ETH_P_IP:
			netA := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer.ip_src[12]), net.IPv4len)
			netB := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer.ip_dst[12]), net.IPv4len)
			f.Network = &flow.FlowLayer{
				Protocol: flow.FlowProtocol_IPV4,
				A:        net.IP(netA).To4().String(),
				B:        net.IP(netB).To4().String(),
			}
		case syscall.ETH_P_IPV6:
			netA := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer.ip_src[0]), net.IPv6len)
			netB := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer.ip_dst[0]), net.IPv6len)
			f.Network = &flow.FlowLayer{
				Protocol: flow.FlowProtocol_IPV6,
				A:        net.IP(netA).String(),
				B:        net.IP(netB).String(),
			}
		}
	}

	// TRANSPORT
	if layersInfo&uint8(C.TRANSPORT_LAYER_INFO) > 0 {
		portA := int64(kernFlow.transport_layer.port_src)
		portB := int64(kernFlow.transport_layer.port_dst)
		protocol := uint8(kernFlow.transport_layer.protocol)

		switch protocol {
		case syscall.IPPROTO_UDP:
			f.Transport = &flow.TransportLayer{
				Protocol: flow.FlowProtocol_UDP,
				A:        portA,
				B:        portB,
			}

			/* disabled for now as no payload is sent
			p := gopacket.NewPacket(C.GoBytes(unsafe.Pointer(&kernFlow.payload[0]), C.PAYLOAD_LENGTH), layers.LayerTypeUDP, gopacket.DecodeOptions{})
			if p.Layer(gopacket.LayerTypeDecodeFailure) == nil {
				path, app := flow.LayersPath(p.Layers())
				f.LayersPath += "/" + path
				f.Application = app
			}
			*/
		case syscall.IPPROTO_TCP:
			f.Transport = &flow.TransportLayer{
				Protocol: flow.FlowProtocol_TCP,
				A:        portA,
				B:        portB,
			}
			f.TCPMetric = &flow.TCPMetric{
				ABSynStart: tcpFlagTime(kernFlow.transport_layer.ab_syn, startKTimeNs, start),
				BASynStart: tcpFlagTime(kernFlow.transport_layer.ba_syn, startKTimeNs, start),
				ABFinStart: tcpFlagTime(kernFlow.transport_layer.ab_fin, startKTimeNs, start),
				BAFinStart: tcpFlagTime(kernFlow.transport_layer.ba_fin, startKTimeNs, start),
				ABRstStart: tcpFlagTime(kernFlow.transport_layer.ab_rst, startKTimeNs, start),
				BARstStart: tcpFlagTime(kernFlow.transport_layer.ba_rst, startKTimeNs, start),
			}
			/* disabled for now as no payload is sent
			p := gopacket.NewPacket(C.GoBytes(unsafe.Pointer(&kernFlow.payload[0]), C.PAYLOAD_LENGTH), layers.LayerTypeTCP, gopacket.DecodeOptions{})
			if p.Layer(gopacket.LayerTypeDecodeFailure) == nil {
				path, app := flow.LayersPath(p.Layers())
				f.LayersPath += "/" + path
				f.Application = app
			}
			*/
		}
	}

	// ICMP
	if layersInfo&uint8(C.ICMP_LAYER_INFO) > 0 {
		kind := uint8(kernFlow.icmp_layer.kind)
		code := uint8(kernFlow.icmp_layer.code)
		id := uint16(kernFlow.icmp_layer.id)

		f.ICMP = &flow.ICMPLayer{
			Code: uint32(code),
			ID:   uint32(id),
		}
		if f.Network != nil && f.Network.Protocol == flow.FlowProtocol_IPV4 {
			f.ICMP.Type = flow.ICMPv4TypeToFlowICMPType(kind)
		} else {
			f.ICMP.Type = flow.ICMPv6TypeToFlowICMPType(kind)
		}
	}

	appLayers := strings.Split(f.LayersPath, "/")
	f.Application = appLayers[len(appLayers)-1]

	f.Metric = &flow.FlowMetric{
		ABBytes:   int64(kernFlow.metrics.ab_bytes),
		ABPackets: int64(kernFlow.metrics.ab_packets),
		BABytes:   int64(kernFlow.metrics.ba_bytes),
		BAPackets: int64(kernFlow.metrics.ba_packets),
		Start:     f.Start,
		Last:      f.Last,
	}

	key := kernFlowKey(kernFlow)

	f.UpdateUUID(key, flow.Opts{LayerKeyMode: flow.L3PreferedKeyMode})

	fops = append(fops, &flow.Operation{
		Type: flow.ReplaceOperation,
		Flow: f,
		Key:  key,
	})
	return fops
}

func (p *EBPFProbe) updateFlowOperation(ebpfFlow *EBPFFlow, kernFlow *C.struct_flow, startKTimeNs int64, start time.Time) *flow.Operation {
	f := flow.NewFlow()
	f.Last = common.UnixMillis(ebpfFlow.last)
	layersInfo := uint8(kernFlow.layers_info)
	if layersInfo&uint8(C.TRANSPORT_LAYER_INFO) > 0 {
		protocol := uint8(kernFlow.transport_layer.protocol)
		switch protocol {
		case syscall.IPPROTO_TCP:
			f.TCPMetric = &flow.TCPMetric{
				ABSynStart: tcpFlagTime(kernFlow.transport_layer.ab_syn, startKTimeNs, start),
				BASynStart: tcpFlagTime(kernFlow.transport_layer.ba_syn, startKTimeNs, start),
				ABFinStart: tcpFlagTime(kernFlow.transport_layer.ab_fin, startKTimeNs, start),
				BAFinStart: tcpFlagTime(kernFlow.transport_layer.ba_fin, startKTimeNs, start),
				ABRstStart: tcpFlagTime(kernFlow.transport_layer.ab_rst, startKTimeNs, start),
				BARstStart: tcpFlagTime(kernFlow.transport_layer.ba_rst, startKTimeNs, start),
			}
		}
	}
	f.Metric = &flow.FlowMetric{
		ABBytes:   int64(kernFlow.metrics.ab_bytes),
		ABPackets: int64(kernFlow.metrics.ab_packets),
		BABytes:   int64(kernFlow.metrics.ba_bytes),
		BAPackets: int64(kernFlow.metrics.ba_packets),
		Start:     f.Start,
		Last:      f.Last,
	}

	key := kernFlowKey(kernFlow)

	return &flow.Operation{
		Type: flow.UpdateOperation,
		Flow: f,
		Key:  key,
	}
}

func (p *EBPFProbe) run() {
	var info syscall.Sysinfo_t
	syscall.Sysinfo(&info)

	_, flowChanOperation := p.flowTable.Start()
	defer p.flowTable.Stop()

	var startKTimeNs int64
	var start time.Time

	ebpfFlows := make(map[C.__u64]*EBPFFlow)
	updateTicker := time.NewTicker(ebpfUpdate)
	defer updateTicker.Stop()

	for {
		select {
		case now := <-updateTicker.C:
			// try to get start monotonic time
			if startKTimeNs == 0 {
				cmap := p.module.Map("u64_config_values")
				if cmap == nil {
					continue
				}

				key := uint32(C.START_TIME_NS)
				var sns int64

				p.module.LookupElement(cmap, unsafe.Pointer(&key), unsafe.Pointer(&sns))
				if sns != 0 {
					startKTimeNs = sns
					start = now
				} else {
					continue
				}
			}

			kernFlow := C.struct_flow{}
			var key, nextKey C.__u64
			for {
				found, err := p.module.LookupNextElement(p.fmap, unsafe.Pointer(&key), unsafe.Pointer(&nextKey), unsafe.Pointer(&kernFlow))
				if !found || err != nil {
					break
				}
				key = nextKey
				// delete every entry after we read the entry value
				p.module.DeleteElement(p.fmap, unsafe.Pointer(&key))

				lastK := int64(kernFlow.last)
				last := start.Add(time.Duration(lastK - startKTimeNs))

				ebpfFlow, ok := ebpfFlows[kernFlow.key]
				if !ok {
					startK := int64(kernFlow.start)

					// check that the local time computed from the kernel time is not greater than Now
					us := start.Add(time.Duration(startK - startKTimeNs))
					if us.After(now) {
						us = now
					}

					ebpfFlow = &EBPFFlow{
						start: now,
						last:  last,
					}
					ebpfFlows[kernFlow.key] = ebpfFlow

					ebpfFlow.lastK = lastK
					ebpfFlow.last = last

					for _, fops := range p.newFlowOperation(ebpfFlow, &kernFlow, startKTimeNs, start) {
						flowChanOperation <- fops
					}
				} else {
					flowChanOperation <- p.updateFlowOperation(ebpfFlow, &kernFlow, startKTimeNs, start)
				}
			}

			for k, v := range ebpfFlows {
				if time.Now().Sub(v.last).Seconds() > p.expire.Seconds() {
					delete(ebpfFlows, k)
				}
			}
		case <-p.quit:
			return
		}
	}
}

func (p *EBPFProbe) stop() {
	p.quit <- true
}

func (p *EBPFProbesHandler) registerProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	if _, ok := p.probes[n.ID]; ok {
		return nil
	}

	ifName, _ := n.GetFieldString("Name")
	if ifName == "" {
		return fmt.Errorf("No name for node %s", n.ID)
	}

	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return fmt.Errorf("No tid for node %s", n.ID)
	}

	_, nsPath, err := topology.NamespaceFromNode(p.graph, n)
	if err != nil {
		return err
	}

	module, err := loadModule()
	if err != nil {
		return err
	}

	fmap := module.Map("flow_table")
	if fmap == nil {
		return fmt.Errorf("Unable to find flow_table map")
	}

	socketFilter := module.SocketFilter("socket_flow_table")
	if socketFilter == nil {
		return errors.New("No flow_table socket filter")
	}

	var rs *common.RawSocket
	if nsPath != "" {
		rs, err = common.NewRawSocketInNs(nsPath, ifName, syscall.ETH_P_ALL)
	} else {
		rs, err = common.NewRawSocket(ifName, syscall.ETH_P_ALL)
	}
	if err != nil {
		return err
	}
	fd := rs.GetFd()

	if err := elf.AttachSocketFilter(socketFilter, fd); err != nil {
		rs.Close()
		return fmt.Errorf("Unable to attach socket filter to node: %s", n.ID)
	}

	ft := p.fpta.Alloc(tid, flow.TableOpts{})

	probe := &EBPFProbe{
		probeNodeTID: tid,
		fd:           rs.GetFd(),
		flowTable:    ft,
		module:       module,
		fmap:         fmap,
		expire:       p.fpta.Expire(),
		quit:         make(chan bool),
	}

	p.probes[n.ID] = probe

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		e.OnStarted()

		probe.run()

		if err := elf.DetachSocketFilter(socketFilter, fd); err != nil {
			logging.GetLogger().Errorf("Unable to detach eBPF probe: %s", err)
		}
		rs.Close()
		module.Close()

		e.OnStopped()
	}()
	return nil
}

func (p *EBPFProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	err := p.registerProbe(n, capture, e)
	if err != nil {
		go e.OnError(err)
	}
	return err
}

func (p *EBPFProbesHandler) unregisterProbe(id graph.Identifier) error {
	if probe, ok := p.probes[id]; ok {
		logging.GetLogger().Debugf("Terminating eBPF capture on %s", id)
		probe.stop()
		delete(p.probes, id)
	}

	return nil
}

func (p *EBPFProbesHandler) UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	err := p.unregisterProbe(n.ID)
	if err != nil {
		return err
	}

	return nil
}

func (p *EBPFProbesHandler) Start() {
}

func (p *EBPFProbesHandler) Stop() {
	p.probesLock.Lock()
	defer p.probesLock.Unlock()

	for id := range p.probes {
		p.unregisterProbe(id)
	}
	p.wg.Wait()
}

func LoadJumpMap(module *elf.Module) error {
	var jmpTable []string = []string{"socket_network_layer"}

	jmpTableMap := module.Map("jmp_map")
	if jmpTableMap == nil {
		return fmt.Errorf("Map: jmp_map not found")
	}
	for i, sym := range jmpTable {
		entry := module.SocketFilter(sym)
		if entry == nil {
			return fmt.Errorf("Symbol %s not found", sym)
		}

		index := uint32(i)
		fd := uint32(entry.Fd())
		err := module.UpdateElement(jmpTableMap, unsafe.Pointer(&index), unsafe.Pointer(&fd), BPF_ANY)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadModule() (*elf.Module, error) {
	data, err := statics.Asset("probe/ebpf/flow.o")
	if err != nil {
		return nil, fmt.Errorf("Unable to find eBPF elf binary in bindata")
	}

	reader := bytes.NewReader(data)

	module := elf.NewModuleFromReader(reader)

	// load to test if everything is ok
	err = module.Load(nil)
	if err != nil {
		// split to skip to kernel stack trace
		errs := strings.Split(err.Error(), ":")
		if len(errs) > 1 {
			logging.GetLogger().Debugf("eBPF kernel stacktrace: %s", errs[1])
		}

		return nil, fmt.Errorf("Unable to load eBPF elf binary (host %s) from bindata: %s", runtime.GOARCH, errs[0])
	}
	if err = LoadJumpMap(module); err != nil {
		return nil, err
	}
	return module, nil
}

func NewEBPFProbesHandler(g *graph.Graph, fpta *FlowProbeTableAllocator) (*EBPFProbesHandler, error) {
	return &EBPFProbesHandler{
		graph:  g,
		probes: make(map[graph.Identifier]*EBPFProbe),
		fpta:   fpta,
	}, nil
}
