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
	GenerateTestFlows(t, ft, 1, "probe-tid1")
	GenerateTestFlows(t, ft, 2, "probe-tid2")

	filters := &Filter{
		BoolFilter: &BoolFilter{
			Op: BoolFilterOp_OR,
			Filters: []*Filter{
				{
					TermStringFilter: &TermStringFilter{Key: "NodeTID", Value: "probe-tid1"},
				},
				{
					TermStringFilter: &TermStringFilter{Key: "ANodeTID", Value: "probe-tid1"},
				},
				{
					TermStringFilter: &TermStringFilter{Key: "BNodeTID", Value: "probe-tid1"},
				},
			},
		},
	}

	flowset := ft.GetFlows(&FlowSearchQuery{Filter: filters})
	if len(flowset.Flows) == 0 {
		t.Errorf("Should have flows with from probe1 returned")
	}

	for _, f := range flowset.Flows {
		if f.NodeTID != "probe-tid1" {
			t.Errorf("Only flow with probe-tid1 as NodeTID is expected, got %s", f.NodeTID)
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
	flows := GenerateTestFlows(t, ft, 0, "probe-tid1")
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
		fs := f.Metric
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

func TestTable_SymmeticsHash(t *testing.T) {
	ft1 := NewTable(nil, nil)
	GenerateTestFlows(t, ft1, 0xca55e77e, "probe-tid")

	UUIDS := make(map[string]bool)
	TRIDS := make(map[string]bool)

	for _, f := range ft1.GetFlows(nil).Flows {
		UUIDS[f.UUID] = true
		TRIDS[f.TrackingID] = true
	}

	ft2 := NewTable(nil, nil)
	GenerateTestFlowsSymmetric(t, ft2, 0xca55e77e, "probe-tid")

	for _, f := range ft2.GetFlows(nil).Flows {
		if _, found := UUIDS[f.UUID]; !found {
			t.Errorf("Flow UUID should support symmetrically, not found : %s", f.UUID)
		}
		if _, found := TRIDS[f.TrackingID]; !found {
			t.Errorf("Flow TrackingID should support symmetrically, not found : %s", f.TrackingID)
		}
	}
}
