/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package traversal

import (
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

type fakeTableClient struct {
	g *graph.Graph
	t *flow.Table
}

func (tc *fakeTableClient) LookupFlows(flowSearchQuery filters.SearchQuery) (*flow.FlowSet, error) {
	obj, _ := proto.Marshal(&flowSearchQuery)
	resp := tc.t.Query(&flow.TableQuery{Type: "SearchQuery", Obj: obj})

	context := flow.MergeContext{
		Sort:      flowSearchQuery.Sort,
		SortBy:    flowSearchQuery.SortBy,
		SortOrder: common.SortOrder(flowSearchQuery.SortOrder),
		Dedup:     flowSearchQuery.Dedup,
		DedupBy:   flowSearchQuery.DedupBy,
	}

	fs := flow.NewFlowSet()
	for _, b := range resp.Obj {
		var fsr flow.FlowSearchReply
		if err := proto.Unmarshal(b, &fsr); err != nil {
			return nil, errors.New("Unable to decode flow search reply")
		}
		fs.Merge(fsr.FlowSet, context)
	}

	return fs, nil
}

func (tc *fakeTableClient) LookupFlowsByNodes(hnmap topology.HostNodeTIDMap, flowSearchQuery filters.SearchQuery) (*flow.FlowSet, error) {
	return tc.LookupFlows(flowSearchQuery)
}

func execTraversalQuery(t *testing.T, tc *fakeTableClient, query string) traversal.GraphTraversalStep {
	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(NewFlowTraversalExtension(tc, nil))

	ts, err := tr.Parse(strings.NewReader(query))
	if err != nil {
		t.Fatalf("%s: %s", query, err)
	}

	res, err := ts.Exec(tc.g, false)
	if err != nil {
		t.Fatalf("%s: %s", query, err)
	}

	return res
}

func newTable(nodeID string) *flow.Table {
	updHandler := flow.NewFlowHandler(func(f []*flow.Flow) {}, time.Second)
	expHandler := flow.NewFlowHandler(func(f []*flow.Flow) {}, 300*time.Second)

	return flow.NewTable(updHandler, expHandler, "", flow.TableOpts{})
}

func newFakeTableClient() *fakeTableClient {
	b, _ := graph.NewMemoryBackend()

	tc := &fakeTableClient{
		t: newTable(""),
		g: graph.NewGraph("", b, common.AnalyzerService),
	}

	return tc
}

func newICMPFlow(id uint32) *flow.Flow {
	icmp := flow.NewFlow()
	icmp.UUID = strconv.Itoa(rand.Int())
	icmp.ICMP = &flow.ICMPLayer{ID: id}
	return icmp
}

func TestHasStep(t *testing.T) {
	tc := newFakeTableClient()

	_, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newICMPFlow(222)
	flowChan <- newICMPFlow(444)

	time.Sleep(time.Second)

	query := `G.Flows().Has("ICMP.ID", 222)`
	res := execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	query = `G.Flows().Has("ICMP.ID", NE(555))`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return Z result, returned: %v", res.Values())
	}

	query = `G.Flows().Has("ICMP.ID", NE(555)).Limit(1)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}

func TestLimitStep(t *testing.T) {
	tc := newFakeTableClient()

	_, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newICMPFlow(222)
	flowChan <- newICMPFlow(444)

	time.Sleep(time.Second)

	query := `G.Flows()`
	res := execTraversalQuery(t, tc, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Limit(1)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}

func TestDedupStep(t *testing.T) {
	tc := newFakeTableClient()

	_, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newICMPFlow(222)
	flowChan <- newICMPFlow(222)

	time.Sleep(time.Second)

	query := `G.Flows()`
	res := execTraversalQuery(t, tc, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Dedup("ICMP.ID")`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	query = `G.Flows().Dedup("UUID").Limit(1)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}

func TestCaptureNodeStep(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"TID": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"TID": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"TID": "789"})

	_, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	icmp := newICMPFlow(222)
	icmp.NodeTID = "123"
	flowChan <- icmp

	icmp = newICMPFlow(444)
	icmp.NodeTID = "456"
	flowChan <- icmp

	time.Sleep(time.Second)

	query := `G.Flows()`
	res := execTraversalQuery(t, tc, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().CaptureNode()`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().CaptureNode().Limit(1)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	query = `G.Flows().Has('NodeTID', "456")`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}

func TestInStep(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "789"})

	_, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	icmp := newICMPFlow(222)
	icmp.Link = &flow.FlowLayer{A: "123"}
	flowChan <- icmp

	icmp = newICMPFlow(444)
	icmp.Link = &flow.FlowLayer{A: "456"}
	flowChan <- icmp

	icmp = newICMPFlow(666)
	icmp.Link = &flow.FlowLayer{A: "123"}
	flowChan <- icmp

	time.Sleep(time.Second)

	query := `G.Flows()`
	res := execTraversalQuery(t, tc, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().In()`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().In().Limit(1)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}

func TestOutStep(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"PeerIntfMAC": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "789"})

	_, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	icmp := newICMPFlow(222)
	icmp.Link = &flow.FlowLayer{B: "123"}
	flowChan <- icmp

	icmp = newICMPFlow(444)
	icmp.Link = &flow.FlowLayer{B: "456"}
	flowChan <- icmp

	icmp = newICMPFlow(666)
	icmp.Link = &flow.FlowLayer{B: "123"}
	flowChan <- icmp

	time.Sleep(time.Second)

	query := `G.Flows()`
	res := execTraversalQuery(t, tc, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Out()`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Out().Limit(1)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}

func TestBothStep(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"PeerIntfMAC": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "789"})

	_, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	icmp := newICMPFlow(222)
	icmp.Link = &flow.FlowLayer{A: "123"}
	flowChan <- icmp

	icmp = newICMPFlow(444)
	icmp.Link = &flow.FlowLayer{B: "456"}
	flowChan <- icmp

	icmp = newICMPFlow(666)
	icmp.Link = &flow.FlowLayer{B: "123"}
	flowChan <- icmp

	time.Sleep(time.Second)

	query := `G.Flows()`
	res := execTraversalQuery(t, tc, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Both()`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 3 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Both().Limit(1)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}
