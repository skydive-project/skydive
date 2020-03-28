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

package ebpf

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/newtools/ebpf"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/ebpf/statics"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/rawsocket"
	"github.com/skydive-project/skydive/topology"
)

/*
#cgo CFLAGS: -I../../../ebpf
#include "flow.h"
#include <string.h>
#include <sys/resource.h>
#include <sys/socket.h>

int probe_bpf_attach_socket(int sock, int fd)
{
	return setsockopt(sock, SOL_SOCKET, SO_ATTACH_BPF, &fd, sizeof(fd));
}

int probe_bpf_detach_socket(int sock, int fd)
{
	return setsockopt(sock, SOL_SOCKET, SO_DETACH_BPF, &fd, sizeof(fd));
}
*/
import "C"

// Probe the eBPF probe
type Probe struct {
	Ctx          probes.Context
	probeNodeTID string
	fd           int
	flowTable    *flow.Table
	module       *ebpf.Collection
	fmap         []*ebpf.Map
	cmap         *ebpf.Map
	expire       time.Duration
	quit         chan bool
	flowPage     int
}

// ProbesHandler creates new eBPF probes
type ProbesHandler struct {
	Ctx probes.Context
	wg  sync.WaitGroup
}

func (p *Probe) swapPage() {
	key := uint32(C.FLOW_PAGE)
	writeInPage := int64(p.flowPage)

	p.cmap.Put(key, writeInPage)

	if p.flowPage == 1 {
		p.flowPage = 0
	} else {
		p.flowPage = 1
	}
}

func (p *Probe) run() {
	var info syscall.Sysinfo_t
	syscall.Sysinfo(&info)

	_, extFlowChan, statsChan := p.flowTable.Start(nil)
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

	zeroKey := make([]byte, 8)
	prevKey := make([]byte, 8)
	delPrevKey := func() {
		if string(prevKey) != string(zeroKey) {
			p.fmap[p.flowPage].Delete(prevKey)
		}
	}
	key := make([]byte, 8)
	var nextAvailablePtr int
	now := time.Now()
	getFirstKey := true
	for {
		select {
		case <-p.quit:
			return
		case now = <-updateNow.C:
		default:
			if statsMap := p.module.Maps["stats_map"]; statsMap != nil {
				var dropKey uint32
				var dropValue int64

				if found, err := statsMap.Get(dropKey, &dropValue); err == nil && found {
					if dropValue > 0 {
						statsChan <- flow.Stats{KernelFlowDropped: dropValue}
					}
					statsMap.Delete(dropKey)
				}
			}
			// try to get start monotonic time
			if startKTimeNs == 0 {
				key := uint32(C.START_TIME_NS)
				var sns int64

				if found, err := p.cmap.Get(key, &sns); err == nil && found && sns != 0 {
					startKTimeNs = sns
					start = now
				}
				time.Sleep(time.Second)
				continue
			}

			tCancel := now.Add(ebpfMaxPollDelay)

			for {
				var err error
				var found bool
				if getFirstKey {
					if found, err = p.fmap[p.flowPage].NextKey(nil, &key); !found {
						/* map empty */
						p.swapPage()
						time.Sleep(time.Second)
						break
					}
					getFirstKey = false
				} else {
					found, err = p.fmap[p.flowPage].NextKey(prevKey, &key)
				}
				if !found || err != nil {
					delPrevKey()
					getFirstKey = true
					prevKey = zeroKey
					break
				}

				kernFlow := unsafe.Pointer(&kernFlows[nextAvailablePtr])
				if _, err = p.fmap[p.flowPage].GetBytes(key, kernFlow); err != nil {
					delPrevKey()
					getFirstKey = true
					prevKey = zeroKey
					break
				}
				delPrevKey()
				prevKey = key

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
				flow.SetEBPFKernFlow(ebpfFlow, kernFlow)

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

func (p *Probe) stop() {
	p.quit <- true
}

// RegisterProbe registers an eBPF probe on an interface
func (p *ProbesHandler) RegisterProbe(n *graph.Node, capture *types.Capture, e probes.ProbeEventHandler) (probes.Probe, error) {
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

	module, err := p.LoadModule()
	if err != nil {
		return nil, err
	}

	fmap1 := module.Maps["flow_table_p1"]
	if fmap1 == nil {
		module.Close()
		return nil, fmt.Errorf("Unable to find flow_table_p1 map")
	}

	fmap2 := module.Maps["flow_table_p2"]
	if fmap2 == nil {
		module.Close()
		return nil, fmt.Errorf("Unable to find flow_table_p2 map")
	}

	cmap := module.Maps["u64_config_values"]
	if cmap == nil {
		module.Close()
		return nil, fmt.Errorf("Unable to find u64_config_values map")
	}

	socketFilter := module.Programs["bpf_flow_table"]
	if socketFilter == nil {
		module.Close()
		return nil, errors.New("No flow_table socket filter")
	}

	var rs *rawsocket.RawSocket
	if nsPath != "" {
		rs, err = rawsocket.NewRawSocketInNs(nsPath, ifName, syscall.ETH_P_ALL)
	} else {
		rs, err = rawsocket.NewRawSocket(ifName, syscall.ETH_P_ALL)
	}
	if err != nil {
		module.Close()
		return nil, err
	}
	fd := rs.GetFd()

	if ret := C.probe_bpf_attach_socket(C.int(fd), C.int(socketFilter.FD())); ret != 0 {
		rs.Close()
		module.Close()
		return nil, fmt.Errorf("Unable to attach socket filter to node: %s", n.ID)
	}

	uuids := flow.UUIDs{NodeTID: tid, CaptureID: capture.UUID}
	ft := p.Ctx.FTA.Alloc(uuids, flow.TableOpts{})

	probe := &Probe{
		Ctx:          p.Ctx,
		probeNodeTID: tid,
		fd:           rs.GetFd(),
		flowTable:    ft,
		module:       module,
		fmap:         []*ebpf.Map{fmap1, fmap2},
		cmap:         cmap,
		flowPage:     1,
		expire:       p.Ctx.FTA.ExpireAfter(),
		quit:         make(chan bool),
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		e.OnStarted(&probes.CaptureMetadata{})

		probe.run()

		if ret := C.probe_bpf_detach_socket(C.int(fd), C.int(socketFilter.FD())); ret != 0 {
			p.Ctx.Logger.Errorf("Unable to detach eBPF probe: %s", err)
		}
		rs.Close()
		module.Close()

		e.OnStopped()
	}()

	return probe, nil
}

// UnregisterProbe stops an eBPF probe on an interface
func (p *ProbesHandler) UnregisterProbe(n *graph.Node, e probes.ProbeEventHandler, fp probes.Probe) error {
	fp.(*Probe).stop()
	return nil
}

func (p *ProbesHandler) Start() error {
	return nil
}

func (p *ProbesHandler) Stop() {
	p.wg.Wait()
}

func LoadJumpMap(module *ebpf.Collection) error {
	var jmpTable []string = []string{"network_layer"}

	jmpTableMap := module.Maps["jmp_map"]
	if jmpTableMap == nil {
		return fmt.Errorf("Map: jmp_map not found")
	}
	for i, sym := range jmpTable {
		entry := module.Programs[sym]
		if entry == nil {
			return fmt.Errorf("Symbol %s not found", sym)
		}

		fd := uint32(entry.FD())
		err := jmpTableMap.Put(uint32(i), &fd)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *ProbesHandler) loadModuleFromAsset(path string) (*ebpf.Collection, error) {
	data, err := statics.Asset(path)
	if err != nil {
		return nil, fmt.Errorf("Unable to find eBPF elf binary in bindata")
	}

	collspec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Can't load %s: %v\n", path, err)
	}

	module, err := ebpf.NewCollection(collspec)
	if err != nil {
		return nil, fmt.Errorf("Can't create collection %s: %v\n", path, err)
	}

	if err == nil {
		p.Ctx.Logger.Infof("Loaded eBPF module %s", path)
	}
	return module, err
}

func (p *ProbesHandler) LoadModule() (*ebpf.Collection, error) {
	err := syscall.Setrlimit(C.RLIMIT_MEMLOCK, &syscall.Rlimit{
		Cur: math.MaxUint64,
		Max: math.MaxUint64,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to adjust rlimit map lock")
	}

	module, err := p.loadModuleFromAsset("ebpf/flow-gre.o")
	if err != nil {
		p.Ctx.Logger.Errorf("Unable to load eBPF elf binary (host %s) from bindata: %s, trying to fallback", runtime.GOARCH, err)

		module, err = p.loadModuleFromAsset("ebpf/flow.o")
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
func (p *ProbesHandler) CaptureTypes() []string {
	return []string{"ebpf"}
}

// NewProbe returns a new eBPF probe
func NewProbe(ctx probes.Context, bundle *probe.Bundle) (probes.FlowProbeHandler, error) {
	return &ProbesHandler{Ctx: ctx}, nil
}
