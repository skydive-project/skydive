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
	"encoding/json"
	"reflect"
	"testing"
)

func TestAvgBandwidth(t *testing.T) {
	now := int64(1462962423)

	ft := NewTable(nil, nil)
	flows := GenerateTestFlows(t, ft, 0x4567, "probequery0")
	for i, f := range flows {
		randomizeLayerStats(t, int64(0x785612+i), now, f)
	}

	fbw := ft.Window(now-100, now+100).AvgBandwidth()
	fbwSeed0x4567 := FlowSetBandwidth{ABpackets: 392193, ABbytes: 225250394, BApackets: 278790, BAbytes: 238466148, Duration: 200, NBFlow: 10}
	if fbw != fbwSeed0x4567 {
		t.Fatal("flows AvgBandwidth didn't match\n", "fbw:", fbw, "fbwSeed0x4567:", fbwSeed0x4567)
	}

	fbw = ft.Window(0, now+100).AvgBandwidth()
	fbw10FlowZero := fbwSeed0x4567
	fbw10FlowZero.Duration = 1462962523
	if fbw != fbw10FlowZero {
		t.Fatal("flows AvgBandwidth should be zero for 10 flows", fbw, fbw10FlowZero)
	}

	fbw = ft.Window(0, 0).AvgBandwidth()
	fbwZero := FlowSetBandwidth{}
	if fbw != fbwZero {
		t.Fatal("flows AvgBandwidth should be zero", fbw, fbwZero)
	}

	fbw = ft.Window(now, now-1).AvgBandwidth()
	if fbw != fbwZero {
		t.Fatal("flows AvgBandwidth should be zero", fbw, fbwZero)
	}
	graphFlows(now, flows)

	fbw = ft.Window(now-89, now-89).AvgBandwidth()
	if fbw != fbwZero {
		t.Fatal("flows AvgBandwidth should be zero", fbw, fbwZero)
	}

	/* flow half window (2 sec) */
	fbw = ft.Window(now-89-1, now-89+1).AvgBandwidth()
	fbwFlow := FlowSetBandwidth{ABpackets: 239, ABbytes: 106266, BApackets: 551, BAbytes: 444983, Duration: 2, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows AvgBandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* flow 2/3 window (3 sec) */
	fbw = ft.Window(now-89-1, now-89+2).AvgBandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 479, ABbytes: 212532, BApackets: 1102, BAbytes: 889966, Duration: 3, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows AvgBandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* flow full window, 1 sec */
	fbw = ft.Window(now-89, now-89+1).AvgBandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 239, ABbytes: 106266, BApackets: 551, BAbytes: 444983, Duration: 1, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows AvgBandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* flow full window shifted (+2), 2 sec */
	fbw = ft.Window(now-89+2, now-89+4).AvgBandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 479, ABbytes: 212532, BApackets: 1102, BAbytes: 889966, Duration: 2, NBFlow: 1}
	if fbw != fbwFlow {
		t.Fatal("flows AvgBandwidth should be from 1 flow ", fbw, fbwFlow)
	}

	/* 2 flows full window, 1 sec */
	winFlows := ft.Window(now-71, now-71+1)
	fbw = winFlows.AvgBandwidth()
	fbwFlow = FlowSetBandwidth{ABpackets: 3956, ABbytes: 3154923, BApackets: 2052, BAbytes: 1879998, Duration: 1, NBFlow: 2}
	if fbw != fbwFlow {
		t.Fatal("flows AvgBandwidth should be from 2 flows ", fbw, fbwFlow)
	}

	tags := make([]string, fbw.NBFlow)
	for i, fw := range winFlows.Flows {
		tags[i] = fw.Link.HashStr()
	}
	graphFlows(now, flows, tags...)

	winFlows = ft.Window(now-58, now-58+1)
	fbw = winFlows.AvgBandwidth()
	fbw4flows := FlowSetBandwidth{ABpackets: 28148, ABbytes: 31677291, BApackets: 3232, BAbytes: 3320778, Duration: 1, NBFlow: 3}
	if fbw != fbw4flows {
		t.Fatal("flows AvgBandwidth should be from 4 flows ", fbw, fbw4flows)
	}

	tags = make([]string, fbw.NBFlow)
	for i, fw := range winFlows.Flows {
		tags[i] = fw.Link.HashStr()
	}
	graphFlows(now, flows, tags...)
}

func TestDedup(t *testing.T) {
	flowset := FlowSet{
		Flows: []*Flow{
			&Flow{TrackingID: "aaa"},
			&Flow{TrackingID: "bbb"},
			&Flow{TrackingID: "aaa"},
		},
	}

	expected := FlowSet{
		Flows: []*Flow{
			&Flow{TrackingID: "aaa"},
			&Flow{TrackingID: "bbb"},
		},
	}

	flowset.Dedup("")

	if !reflect.DeepEqual(expected, flowset) {
		e, _ := json.Marshal(expected)
		f, _ := json.Marshal(flowset)
		t.Errorf("Flowset mismatch, expected: \n\n%s\n\ngot: \n\n%s", string(e), string(f))
	}
}

func TestDedupBy(t *testing.T) {
	flowset := FlowSet{
		Flows: []*Flow{
			&Flow{TrackingID: "aaa", NodeTID: "111"},
			&Flow{TrackingID: "bbb", NodeTID: "111"},
			&Flow{TrackingID: "aaa", NodeTID: "222"},
		},
	}

	expected := FlowSet{
		Flows: []*Flow{
			&Flow{TrackingID: "aaa", NodeTID: "111"},
			&Flow{TrackingID: "aaa", NodeTID: "222"},
		},
	}

	flowset.Dedup("NodeTID")

	if !reflect.DeepEqual(expected, flowset) {
		e, _ := json.Marshal(expected)
		f, _ := json.Marshal(flowset)
		t.Errorf("Flowset mismatch, expected: \n\n%s\n\ngot: \n\n%s", string(e), string(f))
	}
}

func TestMergeDedup(t *testing.T) {
	flowset1 := FlowSet{
		Flows: []*Flow{
			&Flow{TrackingID: "aaa", NodeTID: "111", Metric: &FlowMetric{Start: 0, Last: 1}},
			&Flow{TrackingID: "bbb", NodeTID: "111", Metric: &FlowMetric{Start: 2, Last: 3}},
			&Flow{TrackingID: "aaa", NodeTID: "222", Metric: &FlowMetric{Start: 0, Last: 1}},
		},
	}
	flowset2 := FlowSet{
		Flows: []*Flow{
			&Flow{TrackingID: "aaa", NodeTID: "111", Metric: &FlowMetric{Start: 0, Last: 1}},
			&Flow{TrackingID: "bbb", NodeTID: "111", Metric: &FlowMetric{Start: 4, Last: 6}},
			&Flow{TrackingID: "aaa", NodeTID: "222", Metric: &FlowMetric{Start: 7, Last: 8}},
			&Flow{TrackingID: "ccc", NodeTID: "333", Metric: &FlowMetric{Start: 0, Last: 1}},
		},
	}

	expected := FlowSet{
		Flows: []*Flow{
			&Flow{TrackingID: "aaa", NodeTID: "111", Metric: &FlowMetric{Start: 0, Last: 1}},
			&Flow{TrackingID: "aaa", NodeTID: "222", Metric: &FlowMetric{Start: 0, Last: 1}},
			&Flow{TrackingID: "ccc", NodeTID: "333", Metric: &FlowMetric{Start: 0, Last: 1}},
		},
	}

	flowset1.Dedup("NodeTID")
	flowset2.Dedup("NodeTID")

	flowset1.Merge(&flowset2, MergeContext{Dedup: true, DedupBy: "NodeTID"})

	if !reflect.DeepEqual(expected, flowset1) {
		e, _ := json.Marshal(expected)
		f, _ := json.Marshal(flowset1)
		t.Errorf("Flowset mismatch, expected: \n\n%s\n\ngot: \n\n%s", string(e), string(f))
	}
}
