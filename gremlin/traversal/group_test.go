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

package traversal

import (
	"math/rand"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
)

func TestGroup(t *testing.T) {
	tc := newFakeTableClient("node1")

	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "123"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"PeerIntfMAC": "456"})
	tc.g.NewNode(graph.GenID(), graph.Metadata{"MAC": "789"})

	_, flowChan := tc.t.Start(nil)
	defer tc.t.Stop()
	for tc.t.State() != common.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	icmp := newICMPFlow(222)
	icmp.Link = &flow.FlowLayer{A: "123"}
	icmp.NodeTID = "Capture1"
	icmp.TrackingID = "Tracking123"
	flowChan <- &flow.ExtFlow{Type: flow.OperationExtFlowType, Obj: &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: rand.Uint64()}}
	icmp = newICMPFlow(222)
	icmp.Link = &flow.FlowLayer{A: "123"}
	icmp.NodeTID = "Capture2"
	icmp.TrackingID = "Tracking123"
	flowChan <- &flow.ExtFlow{Type: flow.OperationExtFlowType, Obj: &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: rand.Uint64()}}

	icmp = newICMPFlow(444)
	icmp.Link = &flow.FlowLayer{B: "456"}
	icmp.TrackingID = "Tracking456"
	flowChan <- &flow.ExtFlow{Type: flow.OperationExtFlowType, Obj: &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: rand.Uint64()}}

	icmp = newICMPFlow(666)
	icmp.Link = &flow.FlowLayer{B: "789"}
	icmp.TrackingID = "Tracking789"
	flowChan <- &flow.ExtFlow{Type: flow.OperationExtFlowType, Obj: &flow.Operation{Type: flow.ReplaceOperation, Flow: icmp, Key: rand.Uint64()}}

	time.Sleep(time.Second)

	query := `G.Flows().Has("NodeTID", "Capture1")`
	res := execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	query = `G.Flows().Has("NodeTID", "Capture2")`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}

	query = `G.Flows().Group()`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	flowGroup := res.Values()[0].(map[string][]*flow.Flow)
	if len(flowGroup["Tracking123"]) != 2 {
		t.Fatalf("Tracking123 group should have 2 flows, returned: %v", flowGroup)
	}
	if len(flowGroup["Tracking456"]) != 1 {
		t.Fatalf("Tracking456 group should have 1 flow, returned: %v", flowGroup)
	}
	if len(flowGroup["Tracking999"]) != 0 {
		t.Fatalf("Tracking999 group should have 0 flow, returned: %v", flowGroup)
	}

	query = `G.Flows().Group().MoreThan(1)`
	res = execTraversalQuery(t, tc, query)
	if len(res.Values()) != 1 {
		t.Fatalf("Should return 1 result, returned: %v", res.Values())
	}
	flowGroup = res.Values()[0].(map[string][]*flow.Flow)
	if len(flowGroup["Tracking123"]) != 2 {
		t.Fatalf("Tracking123 group should have 2 flows, returned: %v", flowGroup)
	}
	if len(flowGroup["Tracking456"]) != 0 {
		t.Fatalf("Tracking456 group should have 0 flow, returned: %v", flowGroup)
	}
}
