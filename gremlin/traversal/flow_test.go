/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology"
)

func newGraph(t *testing.T) *graph.Graph {
	b, err := graph.NewMemoryBackend()
	if err != nil {
		t.Error(err)
	}

	return graph.NewGraph("testhost", b, common.UnknownService)
}

func newTransversalGraph(t *testing.T) *graph.Graph {
	g := newGraph(t)

	n1, _ := g.NewNode(graph.GenID(), graph.Metadata{"Value": int64(1), "Type": "intf", "Bytes": int64(1024), "List": []string{"111", "222"}, "Map": map[string]int64{"a": 1}})
	n2, _ := g.NewNode(graph.GenID(), graph.Metadata{"Value": int64(2), "Type": "intf", "Bytes": int64(2024), "IPV4": []string{"10.0.0.1", "10.0.1.2"}})
	n3, _ := g.NewNode(graph.GenID(), graph.Metadata{"Value": int64(3), "IPV4": "192.168.0.34/24", "Map": map[string]int64{}})
	n4, _ := g.NewNode(graph.GenID(), graph.Metadata{"Value": int64(4), "Name": "Node4", "Bytes": int64(4024), "IPV4": "192.168.1.34", "Indexes": []int64{5, 6}})

	g.Link(n1, n2, graph.Metadata{"Direction": "Left", "Name": "e1"})
	g.Link(n2, n3, graph.Metadata{"Direction": "Left", "Name": "e2"})
	g.Link(n3, n4, graph.Metadata{"Name": "e3"})
	g.Link(n1, n4, graph.Metadata{"Name": "e4"})
	g.Link(n1, n3, graph.Metadata{"Mode": "Direct", "Name": "e5"})

	return g
}

type fakeTableClient struct {
	g *graph.Graph
	t *flow.Table
}

func (tc *fakeTableClient) LookupFlows(flowSearchQuery filters.SearchQuery) (*flow.FlowSet, error) {
	resp := tc.t.Query(&flow.TableQuery{Type: "SearchQuery", Query: &flowSearchQuery})

	fs := flow.NewFlowSet()
	if err := proto.Unmarshal(resp, fs); err != nil {
		return nil, errors.New("Unable to decode flow search reply")
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
	updHandler := flow.NewFlowHandler(func(f *flow.FlowArray) {}, time.Second)
	expHandler := flow.NewFlowHandler(func(f *flow.FlowArray) {}, 300*time.Second)

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
	icmp.NodeTID = "node1"
	return icmp
}

func TestHasStepOp(t *testing.T) {
	tc := newFakeTableClient()

	_, _, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: newICMPFlow(222), Key: strconv.Itoa(rand.Int())}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: newICMPFlow(444), Key: strconv.Itoa(rand.Int())}

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

	query = `G.Flows().HasEither("ICMP.ID", 222, "ICMP.ID", 444)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Has("NodeTID", "node1")`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Has("NodeTID", "node1").Has("ICMP.ID", 222)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	query = `G.Flows().HasEither("NodeTID", "node1", "ICMP.ID", 444).Has("ICMP.ID", 222)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}

func TestLimitStepOp(t *testing.T) {
	tc := newFakeTableClient()

	_, _, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: newICMPFlow(222), Key: strconv.Itoa(rand.Int())}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: newICMPFlow(444), Key: strconv.Itoa(rand.Int())}

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

func TestDedupStepOp(t *testing.T) {
	tc := newFakeTableClient()

	_, _, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: newICMPFlow(222), Key: strconv.Itoa(rand.Int())}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: newICMPFlow(222), Key: strconv.Itoa(rand.Int())}

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

func TestCaptureNodeStepOp(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"TID": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"TID": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"TID": "789"})

	_, _, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	icmp := newICMPFlow(222)
	icmp.NodeTID = "123"
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

	icmp = newICMPFlow(444)
	icmp.NodeTID = "456"
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

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

func TestInStepOp(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "789"})

	_, _, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	icmp := newICMPFlow(222)
	icmp.Link = &flow.FlowLayer{A: "123"}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

	icmp = newICMPFlow(444)
	icmp.Link = &flow.FlowLayer{A: "456"}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

	icmp = newICMPFlow(666)
	icmp.Link = &flow.FlowLayer{A: "123"}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

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

func TestOutStepOp(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"PeerIntfMAC": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "789"})

	_, _, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	icmp := newICMPFlow(222)
	icmp.Link = &flow.FlowLayer{B: "123"}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

	icmp = newICMPFlow(444)
	icmp.Link = &flow.FlowLayer{B: "456"}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

	icmp = newICMPFlow(666)
	icmp.Link = &flow.FlowLayer{B: "123"}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

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

func TestBothStepOp(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"PeerIntfMAC": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "789"})

	_, _, flowChan := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	icmp := newICMPFlow(222)
	icmp.Link = &flow.FlowLayer{A: "123"}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

	icmp = newICMPFlow(444)
	icmp.Link = &flow.FlowLayer{B: "456"}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

	icmp = newICMPFlow(666)
	icmp.Link = &flow.FlowLayer{B: "123"}
	flowChan <- &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: strconv.Itoa(rand.Int())}

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

func TestHasStepEBPF(t *testing.T) {
	tc := newFakeTableClient()

	_, flowChan, _ := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newEBPFFlow(222, "node1", "", "")
	flowChan <- newEBPFFlow(444, "node1", "", "")

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

	query = `G.Flows().HasEither("ICMP.ID", 222, "ICMP.ID", 444)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Has("NodeTID", "node1")`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 2 {
		t.Fatalf("Should return 2 result, returned: %v", res.Values())
	}

	query = `G.Flows().Has("NodeTID", "node1").Has("ICMP.ID", 222)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	query = `G.Flows().HasEither("NodeTID", "node1", "ICMP.ID", 444).Has("ICMP.ID", 222)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
}

func TestLimitStepEBPF(t *testing.T) {
	tc := newFakeTableClient()

	_, flowChan, _ := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newEBPFFlow(222, "node1", "", "")
	flowChan <- newEBPFFlow(444, "node1", "", "")

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

func TestDedupStepEBPF(t *testing.T) {
	tc := newFakeTableClient()

	_, flowChan, _ := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newEBPFFlow(222, "node1", "", "")
	flowChan <- newEBPFFlow(222, "node1", "", "")

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

func TestCaptureNodeStepEBPF(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"TID": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"TID": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"TID": "789"})

	_, flowChan, _ := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newEBPFFlow(222, "123", "", "")

	flowChan <- newEBPFFlow(444, "456", "", "")

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

func TestInStepEBPF(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "01:23:00:00:00:00"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "04:56:00:00:00:00"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "07:89:00:00:00:00"})

	_, flowChan, _ := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newEBPFFlow(222, "node1", "01:23:00:00:00:00", "")
	flowChan <- newEBPFFlow(444, "node1", "04:56:00:00:00:00", "")
	flowChan <- newEBPFFlow(666, "node1", "01:23:00:00:00:00", "")

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

func TestOutStepEBPF(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "01:23:00:00:00:00"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"PeerIntfMAC": "04:56:00:00:00:00"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "07:89:00:00:00:00"})

	_, flowChan, _ := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newEBPFFlow(222, "node1", "", "01:23:00:00:00:00")
	flowChan <- newEBPFFlow(444, "node1", "", "04:56:00:00:00:00")
	flowChan <- newEBPFFlow(666, "node1", "", "01:23:00:00:00:00")

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

func TestBothStepEBPF(t *testing.T) {
	tc := newFakeTableClient()

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "01:23:00:00:00:00"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"PeerIntfMAC": "04:56:00:00:00:00"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "07:89:00:00:00:00"})

	_, flowChan, _ := tc.t.Start()
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	flowChan <- newEBPFFlow(222, "node1", "01:23:00:00:00:00", "")
	flowChan <- newEBPFFlow(444, "node1", "", "04:56:00:00:00:00")
	flowChan <- newEBPFFlow(666, "node1", "", "01:23:00:00:00:00")

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
