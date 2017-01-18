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

package traversal

import (
	"reflect"
	"testing"

	"github.com/skydive-project/skydive/flow"
)

func TestFlowMetricsAggregates(t *testing.T) {
	step := MetricsTraversalStep{
		metrics: map[string][]*flow.FlowMetric{
			"aa": {
				{
					ABBytes:   1,
					ABPackets: 1,
					BABytes:   1,
					BAPackets: 1,
					Start:     10,
					Last:      20,
				},
				{
					ABBytes:   2,
					ABPackets: 2,
					BABytes:   2,
					BAPackets: 2,
					Start:     20,
					Last:      30,
				},
			},
			"bb": {
				{
					ABBytes:   4,
					ABPackets: 4,
					BABytes:   4,
					BAPackets: 4,
					Start:     15,
					Last:      25,
				},
				{
					ABBytes:   8,
					ABPackets: 8,
					BABytes:   8,
					BAPackets: 8,
					Start:     40,
					Last:      50,
				},
			},
			"cc": {
				{
					ABBytes:   16,
					ABPackets: 16,
					BABytes:   16,
					BAPackets: 16,
					Start:     48,
					Last:      58,
				},
			},
		},
	}

	expected := MetricsTraversalStep{
		metrics: map[string][]*flow.FlowMetric{
			"Aggregated": {
				{
					ABBytes:   5,
					ABPackets: 5,
					BABytes:   5,
					BAPackets: 5,
					Start:     10,
					Last:      20,
				},
				{
					ABBytes:   2,
					ABPackets: 2,
					BABytes:   2,
					BAPackets: 2,
					Start:     20,
					Last:      30,
				},
				{
					ABBytes:   24,
					ABPackets: 24,
					BABytes:   24,
					BAPackets: 24,
					Start:     40,
					Last:      50,
				},
			},
		},
	}

	got := step.Aggregates()

	if !reflect.DeepEqual(expected.Values(), got.Values()) {
		e, _ := expected.MarshalJSON()
		g, _ := got.MarshalJSON()
		t.Errorf("Metrics mismatch, expected: \n\n%s\n\ngot: \n\n%s", string(e), string(g))
	}
}
