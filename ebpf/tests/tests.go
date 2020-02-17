// +build ebpf

/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"unsafe"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes/ebpf"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// #cgo CFLAGS: -I../
// #include <string.h>
// #include "flow.h"
import "C"

type fakeMessageSender struct {
	sent int
}

func (f *fakeMessageSender) SendFlows(flows []*flow.Flow) {
	f.sent += len(flows)
}

func (f *fakeMessageSender) SendStats(stats flow.Stats) {
}

func getFlows(table *flow.Table, searchQuery *filters.SearchQuery) []*flow.Flow {
	ft := flow.TableQuery{
		Type:  "SearchQuery",
		Query: searchQuery,
	}
	response := table.Query(&ft)
	var fs flow.FlowSet
	fs.Unmarshal(response)
	return fs.GetFlows()
}

func getFlowChain(table *flow.Table, uuid string, flowChain map[string]*flow.Flow) {
	// lookup for the parent
	searchQuery := &filters.SearchQuery{
		Filter: filters.NewTermStringFilter("UUID", uuid),
	}

	flows := getFlows(table, searchQuery)
	if len(flows) != 1 {
		fmt.Printf("Should return only one flow got : %+v", flows)
		os.Exit(71)
	}
	fl := flows[0]

	flowChain[fl.UUID] = fl

	if fl.ParentUUID != "" {
		getFlowChain(table, fl.ParentUUID, flowChain)
	}
}

func validateAllParentChains(table *flow.Table) {
	searchQuery := &filters.SearchQuery{
		Filter: filters.NewNotNullFilter("ParentUUID"),
	}

	flowChain := make(map[string]*flow.Flow, 0)

	flows := getFlows(table, searchQuery)
	for _, fl := range flows {
		flowChain[fl.UUID] = fl

		if fl.ParentUUID != "" {
			getFlowChain(table, fl.UUID, flowChain)
		}
	}

	// we should have touch all the flow
	flows = getFlows(table, &filters.SearchQuery{})
	if len(flows) != len(flowChain) {
		fmt.Printf("Flow parent chain is incorrect : %+v", flows)
		os.Exit(102)
	}
}

func flowsFromPCAP(filename string, linkType layers.LinkType, bpf *flow.BPF, opts ...flow.TableOpts) []*flow.Flow {
	opt := flow.TableOpts{ExtraTCPMetric: true, ReassembleTCP: true, IPDefrag: true}
	if len(opts) > 0 {
		opt = opts[0]
	}

	table := flow.NewTable(time.Second, time.Second, &fakeMessageSender{}, flow.UUIDs{}, opt)
	defer table.Stop()
	err := fillTableFromPCAP(table, filename, linkType)
	if err != nil {
		fmt.Println("fillTableFromPCAP error : ", err)
		return nil
	}
	validateAllParentChains(table)
	return getFlows(table, &filters.SearchQuery{})
}

func fillTableFromPCAP(table *flow.Table, filename string, linkType layers.LinkType) error {
	ebpfProbe := ebpf.ProbesHandler{}
	ebpfProbe.Ctx.Logger = logging.GetLogger()
	module, err := ebpfProbe.LoadModule()
	if err != nil {
		return err
	}

	pcapfile, err := pcap.OpenOffline(filename)
	if err != nil {
		return err
	}
	defer pcapfile.Close()

	prog := module.Programs["bpf_flow_table"]
	fmap := module.Maps["flow_table_p1"]

	if prog == nil || fmap == nil {
		return errors.New("Loading error")
	}

	data, _, err := pcapfile.ReadPacketData()
	for err == nil && len(data) > 0 {
		p := make([]byte, 14+len(data))
		copy(p[14:], data)
		retcode, _, err := prog.Test(p)
		if err != nil {
			return fmt.Errorf("Can't test program %s: (pcap %s) %v\n", prog.String(), filename, err)
		}
		if retcode != 0 {
			return fmt.Errorf("Can't test program %s: (pcap %s) retcode %v | %s\n", prog.String(), filename, retcode, prog.VerifierLog)
		}
		data, _, err = pcapfile.ReadPacketData()
	}

	_, extFlowChan, _ := table.Start(nil)

	var startKTimeNs int64
	var start time.Time
	now := time.Now()
	flowPoolSize := 1000
	kernFlows := make([]C.struct_flow, flowPoolSize)
	extFlows := make([]flow.ExtFlow, flowPoolSize)
	for i := range extFlows {
		extFlows[i] = flow.ExtFlow{
			Type: flow.EBPFExtFlowType,
			Obj:  &flow.EBPFFlow{},
		}
	}
	nextAvailablePtr := 0

	prevKey := make([]byte, 8)
	key := make([]byte, 8)
	getFirstKey := true
	for {
		var err error
		var found bool
		if getFirstKey {
			if found, err = fmap.NextKey(nil, &key); !found {
				/* map empty */
				break
			}
			getFirstKey = false
		} else {
			found, err = fmap.NextKey(prevKey, &key)
		}
		if !found || err != nil {
			getFirstKey = true
			break
		}

		kernFlow := unsafe.Pointer(&kernFlows[nextAvailablePtr])
		if _, err = fmap.GetBytes(key, kernFlow); err != nil {
			getFirstKey = true
			break
		}
		fmap.Delete(key)
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
	}
	return nil
}

func TestPCAPTrace(filename string, expected map[string]flow.Flow) error {
	flows := flowsFromPCAP(filename, layers.LinkTypeEthernet, nil)
	if flows == nil || len(flows) == 0 {
		return fmt.Errorf("no flow returned")
	}

	for _, f := range flows {
		e, found := expected[f.TrackingID]
		if !found {
			return fmt.Errorf("found flow not expected %+v", f)
		}

		if strings.Compare(f.TrackingID, e.TrackingID) != 0 || (strings.Compare(f.LayersPath, e.LayersPath) != 0) || (f.Metric.ABPackets != e.Metric.ABPackets) || (f.Metric.BAPackets != e.Metric.BAPackets) {

			return fmt.Errorf("flow mismatch\nflow %+v\n\nexpected %+v", f, e)
		}
	}
	return nil
}

func TestFlowsEBPFPCAPTrace() {
	pcapfile := "../../flow/pcaptraces/eth-ip4-arp-dns-req-http-google.pcap"
	base := path.Base(pcapfile)
	fmt.Println("Testing", base)

	expected := make(map[string]flow.Flow)
	expected["e67ff9fcd942b47f"] = flow.Flow{
		TrackingID: "e67ff9fcd942b47f",
		LayersPath: "Ethernet/IPv4/UDP",
		Metric:     &flow.FlowMetric{ABPackets: 2, BAPackets: 2},
	}
	expected["227af0a31d1e905d"] = flow.Flow{
		TrackingID: "227af0a31d1e905d",
		LayersPath: "Ethernet/IPv4/UDP",
		Metric:     &flow.FlowMetric{ABPackets: 2, BAPackets: 2},
	}
	expected["324aadac5e538203"] = flow.Flow{
		TrackingID: "324aadac5e538203",
		LayersPath: "Ethernet/ARP",
		Metric:     &flow.FlowMetric{ABPackets: 1, BAPackets: 0},
	}
	expected["c54cbde4114b5c09"] = flow.Flow{
		TrackingID: "c54cbde4114b5c09",
		LayersPath: "Ethernet/ARP",
		Metric:     &flow.FlowMetric{ABPackets: 1, BAPackets: 0},
	}
	expected["ec9e530cf2e13c78"] = flow.Flow{
		TrackingID: "ec9e530cf2e13c78",
		LayersPath: "Ethernet/IPv4/TCP",
		Metric:     &flow.FlowMetric{ABPackets: 20, BAPackets: 18},
	}
	expected["272f6de17d0a37b2"] = flow.Flow{
		TrackingID: "272f6de17d0a37b2",
		LayersPath: "Ethernet/IPv4/TCP",
		Metric:     &flow.FlowMetric{ABPackets: 6, BAPackets: 4},
	}

	err := TestPCAPTrace(pcapfile, expected)
	if err != nil {
		fmt.Println(base, err)
		os.Exit(1)
	}
	fmt.Println("Test OK")
}

func main() {
	fmt.Println("Testing EBPF program with pcap traces")
	TestFlowsEBPFPCAPTrace()
}
