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
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
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
	Ctx          Context
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
	Ctx Context
	wg  sync.WaitGroup
}

func (p *EBPFProbe) run() {
	var info syscall.Sysinfo_t
	syscall.Sysinfo(&info)

	expiredChan := make(chan interface{}, 1000)
	defer close(expiredChan)

	_, extFlowChan := p.flowTable.Start(expiredChan)
	defer p.flowTable.Stop()

	ebpfPollingRate := time.Second / time.Duration(p.Ctx.Config.GetInt("agent.flow.ebpf.polling_rate"))
	const ebpfMaxPollDelay = 10 * time.Second

	var startKTimeNs int64
	var start time.Time

	updateNow := time.NewTicker(ebpfMaxPollDelay)
	defer updateNow.Stop()

	flowPoolSize := 2 * cap(extFlowChan)
	kernFlows := make([]C.struct_flow, flowPoolSize)

	extFlows := make([]flow.ExtFlow, flowPoolSize)
	for i := range extFlows {
		extFlows[i] = flow.ExtFlow{
			Type: flow.EBPFExtFlowType,
			Obj:  &flow.EBPFFlow{},
		}
	}

	var key, nextKey C.__u64
	var nextAvailablePtr int
	now := time.Now()
	for {
		select {
		case <-p.quit:
			return
		case key := <-expiredChan:
			p.module.DeleteElement(p.fmap, unsafe.Pointer(&key))
		case now = <-updateNow.C:
		default:
			if statsMap := p.module.Map("stats_map"); statsMap != nil {
				var statsKey uint32
				var statsVal int64

				if p.module.LookupElement(statsMap, unsafe.Pointer(&statsKey), unsafe.Pointer(&statsVal)) == nil {
					if statsVal > 0 {
						p.Ctx.Logger.Warningf("flow table overflow, %d flows were dropped from kernel table", statsVal)
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

			tCancel := now.Add(ebpfMaxPollDelay)
			for {
				kernFlow := &kernFlows[nextAvailablePtr]

				found, err := p.module.LookupNextElement(p.fmap, unsafe.Pointer(&key), unsafe.Pointer(&nextKey), unsafe.Pointer(kernFlow))
				if !found || err != nil {
					key = 0
					time.Sleep(ebpfPollingRate)
					break
				}
				key = nextKey

				lastK := int64(kernFlows[nextAvailablePtr].last)
				last := start.Add(time.Duration(lastK - startKTimeNs))
				startK := int64(kernFlows[nextAvailablePtr].start)
				startFlow := start.Add(time.Duration(startK - startKTimeNs))
				if startFlow.After(now) {
					startFlow = now
				}

				extFlow := extFlows[nextAvailablePtr]

				ebpfFlow := extFlow.Obj.(*flow.EBPFFlow)
				ebpfFlow.Start = startFlow
				ebpfFlow.Last = last
				ebpfFlow.StartKTimeNs = startKTimeNs
				flow.SetEBPFKernFlow(ebpfFlow, unsafe.Pointer(kernFlow))

				extFlowChan <- &extFlow

				nextAvailablePtr = (nextAvailablePtr + 1) % flowPoolSize

				if now.After(tCancel) {
					break
				}
				time.Sleep(ebpfPollingRate)
				now = now.Add(ebpfPollingRate)
			}
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

	_, nsPath, err := topology.NamespaceFromNode(p.Ctx.Graph, n)
	if err != nil {
		return nil, err
	}

	module, err := p.loadModule()
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
	ft := p.Ctx.FTA.Alloc(uuids, flow.TableOpts{})

	probe := &EBPFProbe{
		Ctx:          p.Ctx,
		probeNodeTID: tid,
		fd:           rs.GetFd(),
		flowTable:    ft,
		module:       module,
		fmap:         fmap,
		expire:       p.Ctx.FTA.ExpireAfter(),
		quit:         make(chan bool),
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		e.OnStarted(&CaptureMetadata{})

		probe.run()

		if err := elf.DetachSocketFilter(socketFilter, fd); err != nil {
			p.Ctx.Logger.Errorf("Unable to detach eBPF probe: %s", err)
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

func (p *EBPFProbesHandler) loadModuleFromAsset(path string) (*elf.Module, error) {
	data, err := statics.Asset(path)
	if err != nil {
		return nil, fmt.Errorf("Unable to find eBPF elf binary in bindata")
	}

	module := elf.NewModuleFromReader(bytes.NewReader(data))
	err = module.Load(nil)

	if err == nil {
		p.Ctx.Logger.Infof("Loaded eBPF module %s", path)
	}
	return module, err
}

func (p *EBPFProbesHandler) loadModule() (*elf.Module, error) {
	module, err := p.loadModuleFromAsset("probe/ebpf/flow-gre.o")
	if err != nil {
		p.Ctx.Logger.Errorf("Unable to load eBPF elf binary (host %s) from bindata: %s, trying to fallback", runtime.GOARCH, err)

		module, err = p.loadModuleFromAsset("probe/ebpf/flow.o")
		if err != nil {
			return nil, fmt.Errorf("Unable to load fallback eBPF elf binary (host %s) from bindata: %s", runtime.GOARCH, err)
		}
		p.Ctx.Logger.Info("Using fallback eBPF program")

		return module, nil
	}
	if err = LoadJumpMap(module); err != nil {
		return nil, err
	}
	return module, nil
}

// CaptureTypes supported
func (p *EBPFProbesHandler) CaptureTypes() []string {
	return []string{"ebpf"}
}

// Init initializes a new eBPF probe
func (p *EBPFProbesHandler) Init(ctx Context, bundle *probe.Bundle) (FlowProbeHandler, error) {
	p.Ctx = ctx
	return p, nil
}
