// +build ebpf

/*
 * Copyright (C) 2017 Red Hat, Inc.
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

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/iovisor/gobpf/elf"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/statics"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

// #cgo CFLAGS: -I../../probe/ebpf
// #include "flow.h"
import "C"

const (
	ebpfUpdate = 2 * time.Second
)

type EBPFFlow struct {
	start time.Time
	last  time.Time
	lastK int64
}

type EBPFProbe struct {
	probeNodeTID string
	fd           int
	flowTable    *flow.Table
	module       *elf.Module
	fmap         *elf.Map
	expire       time.Duration
	quit         chan bool
}

type EBPFProbesHandler struct {
	graph      *graph.Graph
	probes     map[graph.Identifier]*EBPFProbe
	probesLock common.RWMutex
	fpta       *FlowProbeTableAllocator
	wg         sync.WaitGroup
}

func (p *EBPFProbe) flowFromEBPF(ebpfFlow *EBPFFlow, kernFlow *C.struct_flow, updatedAt int64) *flow.Flow {
	f := flow.NewFlow()
	f.Init(common.UnixMillis(ebpfFlow.start), p.probeNodeTID, flow.UUIDs{})
	f.Last = common.UnixMillis(ebpfFlow.last)

	layersFlag := uint8(kernFlow.layers)

	// LINK
	linkA := C.GoBytes(unsafe.Pointer(&kernFlow.link_layer.mac_src[0]), C.ETH_ALEN)
	linkB := C.GoBytes(unsafe.Pointer(&kernFlow.link_layer.mac_dst[0]), C.ETH_ALEN)
	f.Link = &flow.FlowLayer{
		Protocol: flow.FlowProtocol_ETHERNET,
		A:        net.HardwareAddr(linkA).String(),
		B:        net.HardwareAddr(linkB).String(),
	}
	f.LayersPath = "Ethernet"

	// NETWORK
	if layersFlag&uint8(C.NETWORK_LAYER) > 0 {
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
			f.LayersPath += "/IPv4"
		case syscall.ETH_P_IPV6:
			netA := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer.ip_src[0]), net.IPv6len)
			netB := C.GoBytes(unsafe.Pointer(&kernFlow.network_layer.ip_dst[0]), net.IPv6len)
			f.Network = &flow.FlowLayer{
				Protocol: flow.FlowProtocol_IPV4,
				A:        net.IP(netA).String(),
				B:        net.IP(netB).String(),
			}
			f.LayersPath += "/IPv6"
		}
	}

	// TRANSPORT
	if layersFlag&uint8(C.TRANSPORT_LAYER) > 0 {
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

			p := gopacket.NewPacket(C.GoBytes(unsafe.Pointer(&kernFlow.payload[0]), C.PAYLOAD_LENGTH), layers.LayerTypeUDP, gopacket.DecodeOptions{})
			if p.Layer(gopacket.LayerTypeDecodeFailure) == nil {
				path, app := flow.LayersPath(p.Layers())
				f.LayersPath += "/" + path
				f.Application = app
			} else {
				f.LayersPath += "/UDP"
			}
		case syscall.IPPROTO_TCP:
			f.Transport = &flow.TransportLayer{
				Protocol: flow.FlowProtocol_TCP,
				A:        portA,
				B:        portB,
			}

			p := gopacket.NewPacket(C.GoBytes(unsafe.Pointer(&kernFlow.payload[0]), C.PAYLOAD_LENGTH), layers.LayerTypeTCP, gopacket.DecodeOptions{})
			if p.Layer(gopacket.LayerTypeDecodeFailure) == nil {
				path, app := flow.LayersPath(p.Layers())
				f.LayersPath += "/" + path
				f.Application = app
			} else {
				f.LayersPath += "/TCP"
			}
		}
	}

	// ICMP
	if layersFlag&uint8(C.ICMP_LAYER) > 0 {
		kind := uint8(kernFlow.icmp_layer.kind)
		code := uint8(kernFlow.icmp_layer.code)
		id := uint16(kernFlow.icmp_layer.id)

		f.ICMP = &flow.ICMPLayer{
			Code: uint32(code),
			ID:   uint32(id),
		}
		if f.Network != nil && f.Network.Protocol == flow.FlowProtocol_IPV4 {
			f.ICMP.Type = flow.ICMPv4TypeToFlowICMPType(kind)
			f.LayersPath += "/ICMPv4"
		} else {
			f.ICMP.Type = flow.ICMPv6TypeToFlowICMPType(kind)
			f.LayersPath += "/ICMPv6"
		}
	}

	f.RTT = int64(kernFlow.rtt)

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

	hasher := sha1.New()
	hasher.Write(C.GoBytes(unsafe.Pointer(&kernFlow.key), C.sizeof___u64))
	key := hex.EncodeToString(hasher.Sum(nil))

	f.UpdateUUID(key, flow.Opts{})

	return f
}

func (p *EBPFProbe) run() {
	var info syscall.Sysinfo_t
	syscall.Sysinfo(&info)

	_, flowChan := p.flowTable.Start()
	defer p.flowTable.Stop()

	var startKTimeNs int64
	var start time.Time

	ebpfFlows := make(map[C.__u64]*EBPFFlow)

	updateTicker := time.NewTicker(ebpfUpdate)
	defer updateTicker.Stop()

	for {
		select {
		case now := <-updateTicker.C:
			unow := common.UnixMillis(now)

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
				found, _ := p.module.LookupNextElement(p.fmap, unsafe.Pointer(&key), unsafe.Pointer(&nextKey), unsafe.Pointer(&kernFlow))
				if !found {
					break
				}
				key = nextKey

				lastK := int64(kernFlow.last)

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
						last:  start.Add(time.Duration(lastK - startKTimeNs)),
					}
					ebpfFlows[kernFlow.key] = ebpfFlow
				}

				if lastK != ebpfFlow.lastK {
					ebpfFlow.lastK = lastK
					ebpfFlow.last = start.Add(time.Duration(lastK - startKTimeNs))

					fl := p.flowFromEBPF(ebpfFlow, &kernFlow, unow)
					flowChan <- fl
				}

				if now.Sub(ebpfFlow.last).Seconds() > p.expire.Seconds() {
					p.module.DeleteElement(p.fmap, unsafe.Pointer(&kernFlow.key))
					delete(ebpfFlows, kernFlow.key)
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
		return nil
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

	p.probesLock.Lock()
	p.probes[n.ID] = probe
	p.probesLock.Unlock()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		e.OnStarted()

		probe.run()

		if err := elf.DetachSocketFilter(socketFilter, fd); err != nil {
			logging.GetLogger().Errorf("Unable to detach eBPF probe: %s", err)
		}
		module.Close()

		e.OnStopped()
	}()
	return nil
}

func (p *EBPFProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error {
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

	return module, nil
}

func NewEBPFProbesHandler(g *graph.Graph, fpta *FlowProbeTableAllocator) (*EBPFProbesHandler, error) {
	if _, err := loadModule(); err != nil {
		return nil, err
	}

	return &EBPFProbesHandler{
		graph:  g,
		probes: make(map[graph.Identifier]*EBPFProbe),
		fpta:   fpta,
	}, nil
}
