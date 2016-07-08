/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestNewTable(t *testing.T) {
	ft := NewTable(nil, nil)
	if ft == nil {
		t.Error("new FlowTable return null")
	}
}
func TestTable_String(t *testing.T) {
	ft := NewTable(nil, nil)
	if "0 flows" != ft.String() {
		t.Error("FlowTable too big")
	}
	ft = NewTestFlowTableSimple(t)
	if "2 flows" != ft.String() {
		t.Error("FlowTable too big")
	}
}
func TestTable_Update(t *testing.T) {
	ft := NewTestFlowTableSimple(t)
	/* simulate a collision */
	f := &Flow{}
	ft.table["789"] = f
	ft.table["789"].UUID = "78910"
	f = &Flow{}
	f.UUID = "789"
	ft.Update([]*Flow{f})

	ft2 := NewTestFlowTableComplex(t, nil, nil)
	if "10 flows" != ft2.String() {
		t.Error("We should got only 10 flows")
	}
	ft2.Update([]*Flow{f})
	if "11 flows" != ft2.String() {
		t.Error("We should got only 11 flows")
	}
}

type MyTestFlowCounter struct {
	NbFlow int
}

func (fo *MyTestFlowCounter) countFlowsCallback(flows []*Flow) {
	fo.NbFlow += len(flows)
}

func TestTable_expire(t *testing.T) {
	const MaxInt64 = int64(^uint64(0) >> 1)
	fc := MyTestFlowCounter{}
	ft := NewTestFlowTableComplex(t, nil, &FlowHandler{callback: fc.countFlowsCallback})
	beforeNbFlow := fc.NbFlow
	ft.expired(0)
	afterNbFlow := fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 0 {
		t.Error("we should not expire a flow")
	}

	fc = MyTestFlowCounter{}
	beforeNbFlow = fc.NbFlow
	ft.expired(MaxInt64)
	afterNbFlow = fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 10 {
		t.Error("we should expire all flows")
	}
}

func TestTable_updated(t *testing.T) {
	const MaxInt64 = int64(^uint64(0) >> 1)
	fc := MyTestFlowCounter{}
	ft := NewTestFlowTableComplex(t, &FlowHandler{callback: fc.countFlowsCallback}, nil)
	beforeNbFlow := fc.NbFlow
	ft.updated(0)
	afterNbFlow := fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 10 {
		t.Error("all flows should be updated")
	}

	fc = MyTestFlowCounter{}
	beforeNbFlow = fc.NbFlow
	ft.updated(MaxInt64)
	afterNbFlow = fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 0 {
		t.Error("no flows should be updated")
	}
}

func TestTable_AsyncExpire(t *testing.T) {
	t.Skip()
}

func TestTable_AsyncUpdated(t *testing.T) {
	t.Skip()
}

func TestTable_LookupFlowByProbePath(t *testing.T) {
	ft := NewTable(nil, nil)
	GenerateTestFlows(t, ft, 1, "probe1")
	GenerateTestFlows(t, ft, 2, "probe2")

	flowset := ft.GetFlows(FlowQueryFilter{NodeUUIDs: []string{"probe1"}})
	if len(flowset.Flows) == 0 {
		t.Errorf("Should have flows with from probe1 returned")
	}

	for _, f := range flowset.Flows {
		if f.ProbeNodeUUID != "probe1" {
			t.Errorf("Only flow with probe1 as NodeUUID is expected, got %s", f.ProbeNodeUUID)
		}
	}
}

func TestTable_GetFlow(t *testing.T) {
	ft := NewTestFlowTableSimple(t)
	flow := &Flow{}
	flow.UUID = "1234"
	if ft.GetFlow(flow.UUID) == nil {
		t.Fail()
	}
	flow.UUID = "12345"
	if ft.GetFlow(flow.UUID) != nil {
		t.Fail()
	}
}

func TestTable_GetOrCreateFlow(t *testing.T) {
	ft := NewTestFlowTableComplex(t, nil, nil)
	flows := GenerateTestFlows(t, ft, 0, "probe1")
	if len(flows) != 10 {
		t.Error("missing some flows ", len(flows))
	}
	forgeTestPacket(t, int64(1234), false, ETH, IPv4, TCP)
	_, new := ft.GetOrCreateFlow("abcd")
	if !new {
		t.Error("Collision in the FlowTable, should be new")
	}
	forgeTestPacket(t, int64(1234), false, ETH, IPv4, TCP)
	_, new = ft.GetOrCreateFlow("abcd")
	if new {
		t.Error("Collision in the FlowTable, should be an update")
	}
	forgeTestPacket(t, int64(1234), false, ETH, IPv4, TCP)
	_, new = ft.GetOrCreateFlow("abcde")
	if !new {
		t.Error("Collision in the FlowTable, should be a new flow")
	}
}

func TestTable_NewTableFromFlows(t *testing.T) {
	ft := NewTestFlowTableComplex(t, nil, nil)
	var flows []*Flow
	for _, f := range ft.table {
		flow := *f
		flows = append(flows, &flow)
	}
	ft2 := NewTableFromFlows(flows, nil, nil)
	if len(ft.table) != len(ft2.table) {
		t.Error("NewFlowTable(copy) are not the same size")
	}
	flows = flows[:0]
	for _, f := range ft.table {
		flows = append(flows, f)
	}
	ft3 := NewTableFromFlows(flows, nil, nil)
	if len(ft.table) != len(ft3.table) {
		t.Error("NewFlowTable(ref) are not the same size")
	}
}

func TestTable_FilterLast(t *testing.T) {
	ft := NewTestFlowTableComplex(t, nil, nil)
	/* hack to put the FlowTable 1 second older */
	for _, f := range ft.table {
		fs := f.GetStatistics()
		fs.Start -= int64(1)
		fs.Last -= int64(1)
	}
	flows := ft.FilterLast(10 * time.Minute)
	if len(flows) != 10 {
		t.Error("FilterLast should return more/less flows", len(flows), 10)
	}
	flows = ft.FilterLast(0 * time.Minute)
	if len(flows) != 0 {
		t.Error("FilterLast should return less flows", len(flows), 0)
	}
}

func TestTable_SelectLayer(t *testing.T) {
	ft := NewTestFlowTableComplex(t, nil, nil)

	var macs []string
	flowset := ft.SelectLayer(FlowEndpointType_ETHERNET, macs)
	if len(ft.table) <= len(flowset.Flows) && len(flowset.Flows) != 0 {
		t.Errorf("SelectLayer should select none flows %d %d", len(ft.table), len(flowset.Flows))
	}

	for mac := 0; mac < 0xff; mac++ {
		macs = append(macs, fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", 0x00, 0x0F, 0xAA, 0xFA, 0xAA, mac))
	}
	flowset = ft.SelectLayer(FlowEndpointType_ETHERNET, macs)
	if len(ft.table) != len(flowset.Flows) {
		t.Errorf("SelectLayer should select all flows %d %d", len(ft.table), len(flowset.Flows))
	}
}

func TestTable_SymmeticsHash(t *testing.T) {
	ft1 := NewTable(nil, nil)
	GenerateTestFlows(t, ft1, 0xca55e77e, "probe")

	UUIDS := make(map[string]bool)
	TRIDS := make(map[string]bool)

	for _, f := range ft1.GetFlows().Flows {
		UUIDS[f.UUID] = true
		TRIDS[f.TrackingID] = true
	}

	ft2 := NewTable(nil, nil)
	GenerateTestFlowsSymmetric(t, ft2, 0xca55e77e, "probe")

	for _, f := range ft2.GetFlows().Flows {
		if _, found := UUIDS[f.UUID]; !found {
			t.Errorf("Flow UUID should support symmetrically, not found : %s", f.UUID)
		}
		if _, found := TRIDS[f.TrackingID]; !found {
			t.Errorf("Flow TrackingID should support symmetrically, not found : %s", f.TrackingID)
		}
	}
}

func randomizeLayerStats(t *testing.T, seed int64, now int64, f *Flow, ftype FlowEndpointType) {
	rnd := rand.New(rand.NewSource(seed))
	s := f.GetStatistics()
	for _, e := range s.Endpoints {
		if e.Type == ftype {
			e.AB.Packets = uint64(rnd.Int63n(0x10000))
			e.AB.Bytes = e.AB.Packets * uint64(14+rnd.Intn(1501))
			e.BA.Packets = uint64(rnd.Int63n(0x10000))
			e.BA.Bytes = e.BA.Packets * uint64(14+rnd.Intn(1501))

			s.Last = 0
			s.Start = now - rnd.Int63n(100)
			if (rnd.Int() % 2) == 0 {
				s.Last = s.Start + rnd.Int63n(100)
			}
			return
		}
	}
}

func TestNewTableQuery_WindowBandwidth(t *testing.T) {
	now := int64(1462962423)

	ft := NewTable(nil, nil)
	flows := GenerateTestFlows(t, ft, 0x4567, "probequery0")
	for i, f := range flows {
		randomizeLayerStats(t, int64(0x785612+i), now, f, FlowEndpointType_ETHERNET)
	}

	fbw := ft.Window(now-100, now+100).Bandwidth()
	fbwSeed0x4567 := FlowSetBandwidth{ABpackets: 392193, ABbytes: 225250394, BApackets: 278790, BAbytes: 238466148, Duration: 200, NBFlow: 10}
	if fbw != fbwSeed0x4567 {
		t.Fatal("flows Bandwidth didn't match\n", "fbw:", fbw, "fbwSeed0x4567:", fbwSeed0x4567)
	}

	fbw = ft.Window(0, now+100).Bandwidth()
	fbw10FlowZero := fbwSeed0x4567
	fbw10FlowZero.Duration = 1462962523
	if fbw != fbw10FlowZero {
		t.Fatal("flows Bandwidth should be zero for 10 flows", fbw, fbw10FlowZero)
	}

	fbw = ft.Window(0, 0).Bandwidth()
	fbwZero := FlowSetBandwidth{}
	if fbw != fbwZero {
		t.Fatal("flows Bandwidth should be zero", fbw, fbwZero)
	}

	fbw = ft.Window(now, now-1).Bandwidth()
	if fbw != fbwZero {
		t.Fatal("flows Bandwidth should be zero", fbw, fbwZero)
	}
	graphFlows(now, flows)

	fbw = ft.Window(now-89, now-89).Bandwidth()
	if fbw != fbwZero {
		t.Fatal("flows Bandwidth should be zero", fbw, fbwZero)
	}

	/* flow half window (2 sec) */
	winFlows := ft.Window(now-89-1, now-89+1)
	fbw = winFlows.Bandwidth()
	fbwFlow := FlowSetBandwidth{ABpackets: 239, ABbytes: 106266, BApackets: 551, BAbytes: 444983, Duration: 2, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* flow 2/3 window (3 sec) */
	winFlows = ft.Window(now-89-1, now-89+2)
	fbw = winFlows.Bandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 479, ABbytes: 212532, BApackets: 1102, BAbytes: 889966, Duration: 3, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* flow full window, 1 sec */
	winFlows = ft.Window(now-89, now-89+1)
	fbw = winFlows.Bandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 239, ABbytes: 106266, BApackets: 551, BAbytes: 444983, Duration: 1, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* flow full window shifted (+2), 2 sec */
	winFlows = ft.Window(now-89+2, now-89+4)
	fbw = winFlows.Bandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 479, ABbytes: 212532, BApackets: 1102, BAbytes: 889966, Duration: 2, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* 2 flows full window, 1 sec */
	winFlows = ft.Window(now-71, now-71+1)
	fbw = winFlows.Bandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 3956, ABbytes: 3154923, BApackets: 2052, BAbytes: 1879998, Duration: 1, NBFlow: 2}
	if fbw != fbwFlow {
		t.Fatal("flows Bandwidth should be from 2 flows ", fbw, fbwFlow)
	}

	tags := make([]string, fbw.NBFlow)
	for i, fw := range winFlows.Flows {
		tags[i] = fw.GetLayerHash(FlowEndpointType_ETHERNET)
	}
	graphFlows(now, flows, tags...)

	winFlows = ft.Window(now-58, now-58+1)
	fbw = winFlows.Bandwidth()
	fbw4flows := FlowSetBandwidth{ABpackets: 33617, ABbytes: 31846830, BApackets: 7529, BAbytes: 9191349, Duration: 1, NBFlow: 4}
	if fbw != fbw4flows {
		t.Fatal("flows Bandwidth should be from 4 flows ", fbw, fbw4flows)
	}

	tags = make([]string, fbw.NBFlow)
	for i, fw := range winFlows.Flows {
		tags[i] = fw.GetLayerHash(FlowEndpointType_ETHERNET)
	}
	graphFlows(now, flows, tags...)
	t.Log(fbw)
}
