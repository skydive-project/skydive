/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package flow

import (
	"testing"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
)

func TestFlowCreateUpdate(t *testing.T) {
	flows := flowsFromPCAP(t, "pcaptraces/icmpv4-symetric.pcap", layers.LinkTypeEthernet, nil)

	// 200 packets, 50 icmp request with different endpoints, test we have only 50 flow keys
	if len(flows) != 100 {
		t.Errorf("Should return 100 flows got : %+v", flows)
	}

	// test we have only 50 uuids
	uuids := make(map[string]bool)
	for _, f := range flows {
		uuids[f.UUID] = true
	}

	if len(uuids) != 100 {
		t.Errorf("Should return 100 flow uuids got : %+v", flows)
	}
}

type fakeMessageSender struct {
	sent int
}

func (f *fakeMessageSender) SendFlows(flows []*Flow) {
	f.sent += len(flows)
}

func TestFlowExpire(t *testing.T) {
	sender := &fakeMessageSender{}

	table := NewTable(time.Hour, time.Second, sender, UUIDs{}, TableOpts{})

	fillTableFromPCAP(t, table, "pcaptraces/icmpv4-symetric.pcap", layers.LinkTypeEthernet, nil)
	table.expireNow()

	flows := table.getFlows(&filters.SearchQuery{}).Flows

	// check that everything is expired
	if len(flows) != 0 {
		t.Errorf("Should return 0 flows got : %+v", flows)
	}

	// check that the handler sent all the flows
	if sender.sent != 100 {
		t.Errorf("Should receive 100 flows got : %d", sender.sent)
	}
}

func TestGetFlowsWithFilters(t *testing.T) {
	table := NewTable(time.Hour, time.Hour, &fakeMessageSender{}, UUIDs{NodeTID: "probe-1"}, TableOpts{})

	fillTableFromPCAP(t, table, "pcaptraces/icmpv4-symetric.pcap", layers.LinkTypeEthernet, nil)

	filter := filters.NewOrFilter(
		filters.NewTermStringFilter("NodeTID", "probe-1"),
	)

	searchQuery := &filters.SearchQuery{
		Filter: filter,
	}

	flows := table.getFlows(searchQuery).Flows
	if len(flows) != 100 {
		t.Errorf("Should return 100 flow uuids got : %+v", flows)
	}

	// sort test
	searchQuery.Sort = true
	searchQuery.SortBy = "Network.A"
	searchQuery.SortOrder = string(common.SortAscending)

	flows = table.getFlows(searchQuery).Flows

	var last string
	for _, f := range flows {
		if last != "" && f.Network.A < last {
			t.Errorf("Not sorted in the right order got : %s < %s", f.Network.A, last)
		}
		last = f.Network.A
	}

	searchQuery.SortOrder = string(common.SortDescending)

	flows = table.getFlows(searchQuery).Flows

	last = ""
	for _, f := range flows {
		if last != "" && f.Network.A > last {
			t.Errorf("Not sorted in the right order got : %+v", flows)
		}
		last = f.Network.A
	}

	// dedup test
	searchQuery.Dedup = true
	searchQuery.DedupBy = "NodeTID"

	flows = table.getFlows(searchQuery).Flows
	if len(flows) != 1 {
		t.Errorf("Should return 1 flow uuid got : %+v", flows)
	}
}

func TestUpdate(t *testing.T) {
	sender := &fakeMessageSender{}

	table := NewTable(time.Second, time.Hour, sender, UUIDs{}, TableOpts{})

	flow1, _ := table.getOrCreateFlow(123)

	flow1.Metric.ABBytes = 1
	flow1.XXX_state.updateVersion = table.updateVersion + 1

	// check that LastUpdateMetric is filled after a expire before an update
	table.expire(common.UnixMillis(time.Now()))

	if flow1.LastUpdateMetric.ABBytes != 1 {
		t.Errorf("Flow should have been updated by expire : %+v", flow1)
	}

	flow2, _ := table.getOrCreateFlow(456)

	flow2.Metric.ABBytes = 2
	flow2.XXX_state.updateVersion = table.updateVersion + 1

	// should update everything between tableClock and clock
	table.updateAt(time.Now())

	if flow2.LastUpdateMetric.ABBytes != 2 {
		t.Errorf("Flow should have been updated : %+v", flow2)
	}

	if sender.sent != 2 {
		t.Errorf("Should have been notified %d: %+v", sender.sent, flow2)
	}

	// should update everything between previous updateAt and the new one
	table.updateAt(time.Now())

	if flow2.LastUpdateMetric.ABBytes != 0 {
		t.Errorf("Flow should have been updated : %+v", flow2)
	}

	if sender.sent != 2 {
		t.Errorf("Should not have been notified %d: %+v", sender.sent, flow2)
	}

	flow2.Metric.ABBytes = 10
	flow2.XXX_state.updateVersion = table.updateVersion + 1

	// should update everything between previous updateAt and the new one
	table.updateAt(time.Now())

	if flow2.LastUpdateMetric.ABBytes != 8 {
		t.Errorf("Flow should have been updated : %+v", flow2)
	}

	if sender.sent != 3 {
		t.Errorf("Should have been notified %d: %+v", sender.sent, flow2)
	}

	flow2.Metric.ABBytes = 15
	flow2.XXX_state.updateVersion = table.updateVersion + 1

	// should update everything between previous updateAt and the new one
	table.updateAt(time.Now())

	if flow2.LastUpdateMetric.ABBytes != 5 {
		t.Errorf("Flow should have been updated : %+v", flow2)
	}

	if sender.sent != 4 {
		t.Errorf("Should have been notified %d: %+v", sender.sent, flow2)
	}
}

func TestAppSpecificTimeout(t *testing.T) {
	sender := &fakeMessageSender{}

	config.GetConfig().Set("flow.application_timeout.arp", 10)
	config.GetConfig().Set("flow.application_timeout.dns", 20)

	table := NewTable(time.Second, time.Hour, sender, UUIDs{}, TableOpts{})

	flowsTime := time.Now()

	arpFlow, _ := table.getOrCreateFlow(123)
	arpFlow.Last = common.UnixMillis(flowsTime)
	arpFlow.Application = "ARP"

	dnsFlow, _ := table.getOrCreateFlow(456)
	dnsFlow.Last = common.UnixMillis(flowsTime)
	dnsFlow.Application = "DNS"

	table.updateAt(flowsTime.Add(time.Duration(15) * time.Second))

	if sender.sent == 0 || arpFlow.FinishType != FlowFinishType_TIMEOUT {
		t.Errorf("Should have been notified : %+v", arpFlow)
	}

	if sender.sent > 1 || dnsFlow.FinishType != FlowFinishType_NOT_FINISHED {
		t.Errorf("Should not have been notified : %+v", dnsFlow)
	}
}

func TestHold(t *testing.T) {
	table := NewTable(time.Minute, time.Hour, &fakeMessageSender{}, UUIDs{}, TableOpts{})

	flowTime := time.Now()

	flow1, _ := table.getOrCreateFlow(123)
	flow1.Last = common.UnixMillis(flowTime)
	flow1.FinishType = FlowFinishType_TCP_FIN

	table.updateAt(flowTime.Add(time.Duration(5) * time.Second))
	if table.table.Len() != 1 {
		t.Error("Flow should not have been deleted by update")
	}
	table.updateAt(flowTime.Add(time.Duration(15) * time.Second))
	if table.table.Len() != 0 {
		t.Error("Flow should have been deleted by update")
	}

	flow2, _ := table.getOrCreateFlow(456)
	flow2.Last = common.UnixMillis(flowTime)
	flow2.FinishType = FlowFinishType_TCP_FIN
	table.updateAt(flowTime.Add(time.Duration(5) * time.Second))
	flow2.FinishType = FlowFinishType_NOT_FINISHED
	table.updateAt(flowTime.Add(time.Duration(15) * time.Second))
	if table.table.Len() != 1 {
		t.Error("Updated flow should not have been deleted by update")
	}
}

func createBenchTable() *Table {
	return NewTable(600*time.Second, 600*time.Second, &fakeMessageSender{}, UUIDs{}, TableOpts{})
}

func BenchmarkInsert(b *testing.B) {
	table := createBenchTable()
	for n := 0; n < b.N; n++ {
		table.getOrCreateFlow(uint64(n))
	}
}

func BenchmarkReplace(b *testing.B) {
	table := createBenchTable()
	for n := 0; n < b.N; n++ {
		table.getOrCreateFlow(uint64(n))
		table.replaceFlow(uint64(n), nil)
	}
}

func BenchmarkExpire(b *testing.B) {
	table := createBenchTable()
	for n := 0; n < b.N; n++ {
		for i := 0; i != 10000; i++ {
			f, _ := table.getOrCreateFlow(uint64(n))
			f.Start = 0
			f.Last = 5
		}
		table.expire(10)
	}
}

func BenchmarkGetFlows(b *testing.B) {
	table := createBenchTable()
	for n := 0; n < b.N; n++ {
		for i := 0; i != 10000; i++ {
			table.getOrCreateFlow(uint64(n))
		}
		table.getFlows(nil)
	}
}
