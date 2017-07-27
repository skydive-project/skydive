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

func TestDedup(t *testing.T) {
	flowset := FlowSet{
		Flows: []*Flow{
			{TrackingID: "aaa"},
			{TrackingID: "bbb"},
			{TrackingID: "aaa"},
		},
	}

	expected := FlowSet{
		Flows: []*Flow{
			{TrackingID: "aaa"},
			{TrackingID: "bbb"},
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
			{TrackingID: "aaa", NodeTID: "111"},
			{TrackingID: "bbb", NodeTID: "111"},
			{TrackingID: "aaa", NodeTID: "222"},
		},
	}

	expected := FlowSet{
		Flows: []*Flow{
			{TrackingID: "aaa", NodeTID: "111"},
			{TrackingID: "aaa", NodeTID: "222"},
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
			{TrackingID: "aaa", NodeTID: "111", Start: 0, Last: 1, Metric: &FlowMetric{}},
			{TrackingID: "bbb", NodeTID: "111", Start: 2, Last: 3, Metric: &FlowMetric{}},
			{TrackingID: "aaa", NodeTID: "222", Start: 0, Last: 1, Metric: &FlowMetric{}},
		},
	}
	flowset2 := FlowSet{
		Flows: []*Flow{
			{TrackingID: "aaa", NodeTID: "111", Start: 0, Last: 1, Metric: &FlowMetric{}},
			{TrackingID: "bbb", NodeTID: "111", Start: 4, Last: 6, Metric: &FlowMetric{}},
			{TrackingID: "aaa", NodeTID: "222", Start: 7, Last: 8, Metric: &FlowMetric{}},
			{TrackingID: "ccc", NodeTID: "333", Start: 0, Last: 1, Metric: &FlowMetric{}},
		},
	}

	expected := FlowSet{
		Flows: []*Flow{
			{TrackingID: "aaa", NodeTID: "111", Start: 0, Last: 1, Metric: &FlowMetric{}},
			{TrackingID: "aaa", NodeTID: "222", Start: 0, Last: 1, Metric: &FlowMetric{}},
			{TrackingID: "ccc", NodeTID: "333", Start: 0, Last: 1, Metric: &FlowMetric{}},
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
