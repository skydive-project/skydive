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

	"github.com/skydive-project/skydive/filters"
)

func NewTableFromFlows(flows []*Flow, updateHandler *FlowHandler, expireHandler *FlowHandler) *Table {
	nft := NewTable(updateHandler, expireHandler, NewFlowEnhancerPipeline())
	nft.updateFlows(flows)
	return nft
}

func TestNewTable(t *testing.T) {
	ft := NewTable(nil, nil, NewFlowEnhancerPipeline())
	if ft == nil {
		t.Error("new FlowTable return null")
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
	ft.updateFlows([]*Flow{f})

	ft2 := NewTestFlowTableComplex(t, nil, nil)
	if len(ft2.table) != 10 {
		t.Error("We should got only 10 flows")
	}
	ft2.updateFlows([]*Flow{f})
	if len(ft2.table) != 11 {
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
	ft.expire(0)
	afterNbFlow := fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 0 {
		t.Error("we should not expire a flow")
	}

	fc = MyTestFlowCounter{}
	beforeNbFlow = fc.NbFlow
	ft.expire(MaxInt64)
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
	ft.update(0, 0)
	afterNbFlow := fc.NbFlow
	if beforeNbFlow != 0 || afterNbFlow != 10 {
		t.Error("all flows should be updated")
	}

	fc = MyTestFlowCounter{}
	beforeNbFlow = fc.NbFlow
	ft.update(MaxInt64, MaxInt64)
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
	now := time.Now()
	ft := NewTable(nil, nil, NewFlowEnhancerPipeline())
	GenerateTestFlows(t, ft, 1, "probe-tid1", now)
	GenerateTestFlows(t, ft, 2, "probe-tid2", now)

	f := filters.NewOrFilter(
		filters.NewTermStringFilter("NodeTID", "probe-tid1"),
		filters.NewTermStringFilter("ANodeTID", "probe-tid1"),
		filters.NewTermStringFilter("BNodeTID", "probe-tid1"),
	)

	flowset := ft.getFlows(&filters.SearchQuery{Filter: f})
	if len(flowset.Flows) == 0 {
		t.Errorf("Should have flows with from probe1 returned")
	}

	for _, f := range flowset.Flows {
		if f.NodeTID != "probe-tid1" {
			t.Errorf("Only flow with probe-tid1 as NodeTID is expected, got %s", f.NodeTID)
		}
	}
}

func TestTable_getOrCreateFlow(t *testing.T) {
	ft := NewTestFlowTableComplex(t, nil, nil)
	flows := GenerateTestFlows(t, ft, 0, "probe-tid1", time.Now())
	if len(flows) != 10 {
		t.Error("missing some flows ", len(flows))
	}
	forgeTestPacket(t, int64(1234), false, ETH, IPv4, TCP)
	_, new := ft.getOrCreateFlow("abcd")
	if !new {
		t.Error("Collision in the FlowTable, should be new")
	}
	forgeTestPacket(t, int64(1234), false, ETH, IPv4, TCP)
	_, new = ft.getOrCreateFlow("abcd")
	if new {
		t.Error("Collision in the FlowTable, should be an update")
	}
	forgeTestPacket(t, int64(1234), false, ETH, IPv4, TCP)
	_, new = ft.getOrCreateFlow("abcde")
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

func TestTable_SymmetricHash(t *testing.T) {
	now := time.Now()
	ft1 := NewTable(nil, nil, NewFlowEnhancerPipeline())
	GenerateTestFlows(t, ft1, 0xca55e77e, "probe-tid", now)

	UUIDS := make(map[string]bool)
	TRIDS := make(map[string]bool)

	for _, f := range ft1.getFlows(nil).Flows {
		UUIDS[f.UUID] = true
		TRIDS[f.TrackingID] = true
	}

	ft2 := NewTable(nil, nil, NewFlowEnhancerPipeline())
	GenerateTestFlowsSymmetric(t, ft2, 0xca55e77e, "probe-tid", now)

	for _, f := range ft2.getFlows(nil).Flows {
		if _, found := UUIDS[f.UUID]; !found {
			t.Errorf("Flow UUID should support symmetrically, not found : %s", f.UUID)
		}
		if _, found := TRIDS[f.TrackingID]; !found {
			t.Errorf("Flow TrackingID should support symmetrically, not found : %s", f.TrackingID)
		}
	}
}
