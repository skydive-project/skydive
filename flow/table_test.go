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
	"testing"
	"time"

	"crypto/sha1"
	"encoding/hex"
)

func TestNewTable(t *testing.T) {
	ft := NewTable()
	if ft == nil {
		t.Error("new FlowTable return null")
	}
}
func TestTable_String(t *testing.T) {
	ft := NewTable()
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

	ft2 := NewTestFlowTableComplex(t)
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
	ft := NewTestFlowTableComplex(t)

	fc := MyTestFlowCounter{}
	beforeNbFlow := fc.NbFlow
	ft.expire(fc.countFlowsCallback, 0)
	afterNbFlow := fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 0 {
		t.Error("we should not expire a flow")
	}

	fc = MyTestFlowCounter{}
	beforeNbFlow = fc.NbFlow
	ft.expire(fc.countFlowsCallback, MaxInt64)
	afterNbFlow = fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 10 {
		t.Error("we should expire all flows")
	}
}

func TestTable_updated(t *testing.T) {
	const MaxInt64 = int64(^uint64(0) >> 1)
	ft := NewTestFlowTableComplex(t)

	fc := MyTestFlowCounter{}
	beforeNbFlow := fc.NbFlow
	ft.updated(fc.countFlowsCallback, 0)
	afterNbFlow := fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 10 {
		t.Error("all flows should be updated")
	}

	fc = MyTestFlowCounter{}
	beforeNbFlow = fc.NbFlow
	ft.updated(fc.countFlowsCallback, MaxInt64)
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
	ft := NewTable()
	GenerateTestFlows(t, ft, 1, "probe1")
	GenerateTestFlows(t, ft, 2, "probe2")

	flows := ft.LookupFlowsByProbePath("probe1")
	if len(flows) == 0 {
		t.Errorf("Should have flows with from probe1 returned")
	}

	for _, f := range flows {
		if f.ProbeGraphPath != "probe1" {
			t.Errorf("Only flow with probe1 as probepath are expected, got %s", f.ProbeGraphPath)
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
	ft := NewTestFlowTableComplex(t)
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
	ft := NewTestFlowTableComplex(t)
	var flows []*Flow
	for _, f := range ft.table {
		flow := *f
		flows = append(flows, &flow)
	}
	ft2 := NewTableFromFlows(flows)
	if len(ft.table) != len(ft2.table) {
		t.Error("NewFlowTable(copy) are not the same size")
	}
	flows = flows[:0]
	for _, f := range ft.table {
		flows = append(flows, f)
	}
	ft3 := NewTableFromFlows(flows)
	if len(ft.table) != len(ft3.table) {
		t.Error("NewFlowTable(ref) are not the same size")
	}
}

func TestTable_FilterLast(t *testing.T) {
	ft := NewTestFlowTableComplex(t)
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
	ft := NewTestFlowTableComplex(t)

	var macs []string
	flows := ft.SelectLayer(FlowEndpointType_ETHERNET, macs)
	if len(ft.table) <= len(flows) && len(flows) != 0 {
		t.Errorf("SelectLayer should select none flows %d %d", len(ft.table), len(flows))
	}

	for mac := 0; mac < 0xff; mac++ {
		macs = append(macs, fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", 0x00, 0x0F, 0xAA, 0xFA, 0xAA, mac))
	}
	flows = ft.SelectLayer(FlowEndpointType_ETHERNET, macs)
	if len(ft.table) != len(flows) {
		t.Errorf("SelectLayer should select all flows %d %d", len(ft.table), len(flows))
	}
}

func TestTable_SymmeticsHash(t *testing.T) {
	ft := NewTable()
	GenerateTestFlows(t, ft, 0xca55e77e, "probe")

	foundLayers := make(map[string]bool)

	for _, f := range ft.GetFlows() {
		hasher := sha1.New()
		for _, ep := range f.GetStatistics().GetEndpoints() {
			hasher.Write(ep.Hash)
		}
		layersH := hex.EncodeToString(hasher.Sum(nil))
		foundLayers[layersH] = true
	}

	ft2 := NewTable()
	GenerateTestFlowsSymmetric(t, ft2, 0xca55e77e, "probe")

	for _, f := range ft2.GetFlows() {
		hasher := sha1.New()
		for _, ep := range f.GetStatistics().GetEndpoints() {
			hasher.Write(ep.Hash)
		}
		layersH := hex.EncodeToString(hasher.Sum(nil))
		if _, found := foundLayers[layersH]; !found {
			t.Errorf("hash endpoint should be symmeticaly, not found : %s", layersH)
			t.Fail()
		}
	}
}
