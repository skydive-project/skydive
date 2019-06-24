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
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/iovisor/gobpf/elf"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
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
	BPF_ANY = 0
)

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
	graph *graph.Graph
	fta   *flow.TableAllocator
	wg    sync.WaitGroup
}

func (p *EBPFProbe) run() {
	var info syscall.Sysinfo_t
	syscall.Sysinfo(&info)

	_, flowEBPFChan, _ := p.flowTable.Start()
	defer p.flowTable.Stop()

	ebpfUpdate := time.Duration(config.GetConfig().GetInt("agent.flow.ebpf.kernel_scan_interval")) * time.Second
	flowTableSz := config.GetConfig().GetInt("flow.max_entries")

	var startKTimeNs int64
	var start time.Time

	updateTicker := time.NewTicker(ebpfUpdate)
	defer updateTicker.Stop()

	flowPoolSize := 2 * cap(flowEBPFChan)
	kernFlows := make([]C.struct_flow, flowPoolSize)
	ebpfFlows := make([]flow.EBPFFlow, flowPoolSize)

	var key, nextKey C.__u64
	var nextAvailablePtr int
	for {
		select {
		case now := <-updateTicker.C:
			if statsMap := p.module.Map("stats_map"); statsMap != nil {
				var statsKey uint32
				var statsVal int64

				if p.module.LookupElement(statsMap, unsafe.Pointer(&statsKey), unsafe.Pointer(&statsVal)) == nil {
					if statsVal > 0 {
						logging.GetLogger().Warningf("flow table overflow, %d flows were dropped from kernel table", statsVal)
					}
					p.module.DeleteElement(statsMap, unsafe.Pointer(&statsKey))
				}
			}
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

			for i := 0; i != flowTableSz; i++ {
				kernFlow := &kernFlows[nextAvailablePtr]

				found, err := p.module.LookupNextElement(p.fmap, unsafe.Pointer(&key), unsafe.Pointer(&nextKey), unsafe.Pointer(kernFlow))
				if !found || err != nil {
					p.module.DeleteElement(p.fmap, unsafe.Pointer(&key))
					key = 0
					break
				}
				// delete every entry after we read the entry value
				p.module.DeleteElement(p.fmap, unsafe.Pointer(&key))
				key = nextKey

				lastK := int64(kernFlows[nextAvailablePtr].last)
				last := start.Add(time.Duration(lastK - startKTimeNs))
				startK := int64(kernFlows[nextAvailablePtr].start)
				us := start.Add(time.Duration(startK - startKTimeNs))
				if us.After(now) {
					us = now
				}

				ebpfFlow := &ebpfFlows[nextAvailablePtr]
				ebpfFlow.Start = start
				ebpfFlow.Last = last
				ebpfFlow.StartKTimeNs = startKTimeNs
				flow.SetEBPFKernFlow(ebpfFlow, unsafe.Pointer(kernFlow))

				flowEBPFChan <- ebpfFlow

				nextAvailablePtr = (nextAvailablePtr + 1) % flowPoolSize
			}
		case <-p.quit:
			return
		}
	}
}

func (p *EBPFProbe) stop() {
	p.quit <- true
}

// RegisterProbe registers an eBPF probe on an interface
func (p *EBPFProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e ProbeEventHandler) (Probe, error) {
	ifName, _ := n.GetFieldString("Name")
	if ifName == "" {
		return nil, fmt.Errorf("No name for node %s", n.ID)
	}

	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		return nil, fmt.Errorf("No tid for node %s", n.ID)
	}

	_, nsPath, err := topology.NamespaceFromNode(p.graph, n)
	if err != nil {
		return nil, err
	}

	module, err := loadModule()
	if err != nil {
		return nil, err
	}

	fmap := module.Map("flow_table")
	if fmap == nil {
		module.Close()
		return nil, fmt.Errorf("Unable to find flow_table map")
	}

	socketFilter := module.SocketFilter("socket_flow_table")
	if socketFilter == nil {
		module.Close()
		return nil, errors.New("No flow_table socket filter")
	}

	var rs *common.RawSocket
	if nsPath != "" {
		rs, err = common.NewRawSocketInNs(nsPath, ifName, syscall.ETH_P_ALL)
	} else {
		rs, err = common.NewRawSocket(ifName, syscall.ETH_P_ALL)
	}
	if err != nil {
		module.Close()
		return nil, err
	}
	fd := rs.GetFd()

	if err := elf.AttachSocketFilter(socketFilter, fd); err != nil {
		rs.Close()
		module.Close()
		return nil, fmt.Errorf("Unable to attach socket filter to node: %s", n.ID)
	}

	uuids := flow.UUIDs{NodeTID: tid, CaptureID: capture.UUID}
	ft := p.fta.Alloc(uuids, flow.TableOpts{})

	probe := &EBPFProbe{
		probeNodeTID: tid,
		fd:           rs.GetFd(),
		flowTable:    ft,
		module:       module,
		fmap:         fmap,
		expire:       p.fta.ExpireAfter(),
		quit:         make(chan bool),
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		e.OnStarted(&CaptureMetadata{})

		probe.run()

		if err := elf.DetachSocketFilter(socketFilter, fd); err != nil {
			logging.GetLogger().Errorf("Unable to detach eBPF probe: %s", err)
		}
		rs.Close()
		module.Close()

		e.OnStopped()
	}()

	return probe, nil
}

// UnregisterProbe stops an eBPF probe on an interface
func (p *EBPFProbesHandler) UnregisterProbe(n *graph.Node, e ProbeEventHandler, fp Probe) error {
	fp.(*EBPFProbe).stop()
	return nil
}

func (p *EBPFProbesHandler) Start() {
}

func (p *EBPFProbesHandler) Stop() {
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

func loadModuleFromAsset(path string) (*elf.Module, error) {
	data, err := statics.Asset(path)
	if err != nil {
		return nil, fmt.Errorf("Unable to find eBPF elf binary in bindata")
	}

	module := elf.NewModuleFromReader(bytes.NewReader(data))
	err = module.Load(nil)

	return module, err
}

func loadModule() (*elf.Module, error) {
	module, err := loadModuleFromAsset("probe/ebpf/flow-gre.o")
	if err != nil {
		logging.GetLogger().Errorf("Unable to load eBPF elf binary (host %s) from bindata: %s, trying to fallback", runtime.GOARCH, err)

		module, err = loadModuleFromAsset("probe/ebpf/flow.o")
		if err != nil {
			return nil, fmt.Errorf("Unable to load fallback eBPF elf binary (host %s) from bindata: %s", runtime.GOARCH, err)
		}
		logging.GetLogger().Info("Using fallback eBPF program")

		return module, nil
	}
	if err = LoadJumpMap(module); err != nil {
		return nil, err
	}
	return module, nil
}

func NewEBPFProbesHandler(g *graph.Graph, fta *flow.TableAllocator) (*EBPFProbesHandler, error) {
	return &EBPFProbesHandler{
		graph: g,
		fta:   fta,
	}, nil
}
