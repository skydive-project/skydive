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

package traversal

import (
	"reflect"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
)

type FakeGraphBackend struct {
	graph.MemoryBackend
}

func (b *FakeGraphBackend) IsHistorySupported() bool {
	return true
}

func testMetric(t *testing.T, metrics, expected map[string][]common.Metric, tm time.Time, dr time.Duration) {
	g := graph.NewGraph("test", &FakeGraphBackend{}, common.UnknownService)

	gt := traversal.NewGraphTraversal(g, false)
	gt = gt.Context(tm, dr)

	step := NewMetricsTraversalStep(gt, metrics)

	got := step.Aggregates(traversal.StepContext{}, int64(10))

	exp := NewMetricsTraversalStep(gt, expected)
	if !reflect.DeepEqual(exp.Values(), got.Values()) {
		e, _ := exp.MarshalJSON()
		g, _ := got.MarshalJSON()
		t.Errorf("Metrics mismatch, expected: \n\n%s\n\ngot: \n\n%s", string(e), string(g))
	}
}

// |------- Metric ------ |
// |------- Slice  -------|
func TestFlowMetricsAggregates1(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   1000,
				ABPackets: 1000,
				BABytes:   1000,
				BAPackets: 1000,
				Start:     0,
				Last:      10000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   1000,
				ABPackets: 1000,
				BABytes:   1000,
				BAPackets: 1000,
				Start:     0,
				Last:      10000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(10, 0), 10*time.Second)
}

// |--------------- Metric -------------- |
// |------- Slice  -------|
func TestFlowMetricsAggregates2(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   1000,
				ABPackets: 1000,
				BABytes:   1000,
				BAPackets: 1000,
				Start:     0,
				Last:      20000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   500,
				ABPackets: 500,
				BABytes:   500,
				BAPackets: 500,
				Start:     0,
				Last:      10000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(10, 0), 10*time.Second)
}

// |--------------- Metric -------------- |
//                 |------- Slice  -------|
func TestFlowMetricsAggregates3(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   1000,
				ABPackets: 1000,
				BABytes:   1000,
				BAPackets: 1000,
				Start:     0,
				Last:      20000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   500,
				ABPackets: 500,
				BABytes:   500,
				BAPackets: 500,
				Start:     10000,
				Last:      20000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(20, 0), 10*time.Second)
}

//          |------------ Metric ----------- |
// |------- Slice  -------|
func TestFlowMetricsAggregates4(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   1000,
				ABPackets: 1000,
				BABytes:   1000,
				BAPackets: 1000,
				Start:     10000,
				Last:      20000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   500,
				ABPackets: 500,
				BABytes:   500,
				BAPackets: 500,
				Start:     5000,
				Last:      15000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(15, 0), 10*time.Second)
}

// |--------- Metric --------- |
//                   |------- Slice  -------|
func TestFlowMetricsAggregates5(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   1000,
				ABPackets: 1000,
				BABytes:   1000,
				BAPackets: 1000,
				Start:     10000,
				Last:      20000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   500,
				ABPackets: 500,
				BABytes:   500,
				BAPackets: 500,
				Start:     15000,
				Last:      25000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(25, 0), 10*time.Second)
}

//   |------ Metric ------- |
// |---------- Slice  ------------|
func TestFlowMetricsAggregates6(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   1000,
				ABPackets: 1000,
				BABytes:   1000,
				BAPackets: 1000,
				Start:     1000,
				Last:      5000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   1000,
				ABPackets: 1000,
				BABytes:   1000,
				BAPackets: 1000,
				Start:     0,
				Last:      10000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(10, 0), 10*time.Second)
}

// |--------------------------- Metric ---------------------- |
// |--- Slice  ---|
func TestFlowMetricsAggregates7(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   1,
				ABPackets: 1,
				BABytes:   1,
				BAPackets: 1,
				Start:     0,
				Last:      60000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   1,
				ABPackets: 1,
				BABytes:   1,
				BAPackets: 1,
				Start:     0,
				Last:      10000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(10, 0), 10*time.Second)
}

// |--------------------------- Metric ---------------------- |
//            |--- Slice  ---|
func TestFlowMetricsAggregates8(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   1,
				ABPackets: 1,
				BABytes:   1,
				BAPackets: 1,
				Start:     0,
				Last:      60000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   0,
				ABPackets: 0,
				BABytes:   0,
				BAPackets: 0,
				Start:     10000,
				Last:      20000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(20, 0), 10*time.Second)
}

// |--------------------------- Metric ---------------------- |
// |------*------*-------*------ Slice  ----*-------*-----*-- |
func TestFlowMetricsAggregates9(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   18000,
				ABPackets: 18000,
				BABytes:   18000,
				BAPackets: 18000,
				Start:     0,
				Last:      30000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     0,
				Last:      10000,
			},
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     10000,
				Last:      20000,
			},
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     20000,
				Last:      30000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(30, 0), 30*time.Second)
}

// |--------------------------- Metric ---------------------- |
//           |------*------*-------*------ Slice  ----*-------*-----*-- |
func TestFlowMetricsAggregates10(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   18000,
				ABPackets: 18000,
				BABytes:   18000,
				BAPackets: 18000,
				Start:     0,
				Last:      30000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     10000,
				Last:      20000,
			},
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     20000,
				Last:      30000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(30, 0), 20*time.Second)
}

// |------------ Metric ----------| |----- Metric --------------------- |
//           |------*------*-------*------ Slice  ----*-------*-----*-- |
func TestFlowMetricsAggregates11(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     0,
				Last:      15000,
			},
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     15000,
				Last:      30000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   4000,
				ABPackets: 4000,
				BABytes:   4000,
				BAPackets: 4000,
				Start:     10000,
				Last:      20000,
			},
			&flow.FlowMetric{
				ABBytes:   4000,
				ABPackets: 4000,
				BABytes:   4000,
				BAPackets: 4000,
				Start:     20000,
				Last:      30000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(30, 0), 20*time.Second)
}

//           |--- Metric ---|--- Metric ---|
// |------------ Metric ----------|------------- Metric --------------- |
//           |------*------*-------*------ Slice  ----*-------*-----*-- |
func TestFlowMetricsAggregates12(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     0,
				Last:      15000,
			},
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     15000,
				Last:      30000,
			},
		},
		"bb": {
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     5000,
				Last:      10000,
			},
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     10000,
				Last:      20000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   13000,
				ABPackets: 13000,
				BABytes:   13000,
				BAPackets: 13000,
				Start:     5000,
				Last:      15000,
			},
			&flow.FlowMetric{
				ABBytes:   7000,
				ABPackets: 7000,
				BABytes:   7000,
				BAPackets: 7000,
				Start:     15000,
				Last:      25000,
			},
			&flow.FlowMetric{
				ABBytes:   2000,
				ABPackets: 2000,
				BABytes:   2000,
				BAPackets: 2000,
				Start:     25000,
				Last:      30000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(30, 0), 25*time.Second)
}

//           |--- Metric ---|--- Metric ---|--------- Metric ---------- |
// |------------ Metric ----------|------------- Metric --------------- |
//           |------*------*-------*------ Slice  ----*-------*-----*-- |
func TestFlowMetricsAggregates13(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     0,
				Last:      15000,
			},
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     15000,
				Last:      30000,
			},
		},
		"bb": {
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     5000,
				Last:      10000,
			},
			&flow.FlowMetric{
				ABBytes:   6000,
				ABPackets: 6000,
				BABytes:   6000,
				BAPackets: 6000,
				Start:     10000,
				Last:      20000,
			},
			&flow.FlowMetric{
				ABBytes:   2000,
				ABPackets: 2000,
				BABytes:   2000,
				BAPackets: 2000,
				Start:     20000,
				Last:      30000,
			},
		},
	}

	expected := map[string][]common.Metric{
		"Aggregated": {
			&flow.FlowMetric{
				ABBytes:   13000,
				ABPackets: 13000,
				BABytes:   13000,
				BAPackets: 13000,
				Start:     5000,
				Last:      15000,
			},
			&flow.FlowMetric{
				ABBytes:   8000,
				ABPackets: 8000,
				BABytes:   8000,
				BAPackets: 8000,
				Start:     15000,
				Last:      25000,
			},
			&flow.FlowMetric{
				ABBytes:   3000,
				ABPackets: 3000,
				BABytes:   3000,
				BAPackets: 3000,
				Start:     25000,
				Last:      30000,
			},
		},
	}

	testMetric(t, metrics, expected, time.Unix(30, 0), 25*time.Second)
}

func testMetricSum(t *testing.T, metrics map[string][]common.Metric, expected common.Metric, tm time.Time, dr time.Duration) {
	g := graph.NewGraph("test", &FakeGraphBackend{}, common.UnknownService)

	gt := traversal.NewGraphTraversal(g, false)
	gt = gt.Context(tm, dr)
	ctx := traversal.StepContext{}

	step := NewMetricsTraversalStep(gt, metrics)

	got := step.Aggregates(ctx, int64(10)).Sum(ctx).Values()[0]

	if !reflect.DeepEqual(expected, got) {
		t.Errorf("Metrics mismatch, expected: \n\n%s\n\ngot: \n\n%s", expected, got)
	}
}

func TestFlowMetricsAggregatesSum1(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   18000,
				ABPackets: 18000,
				BABytes:   18000,
				BAPackets: 18000,
				Start:     0,
				Last:      30000,
			},
		},
	}

	expected := &flow.FlowMetric{
		ABBytes:   18000,
		ABPackets: 18000,
		BABytes:   18000,
		BAPackets: 18000,
		Start:     0,
		Last:      30000,
	}

	testMetricSum(t, metrics, expected, time.Unix(30, 0), 30*time.Second)
}

func TestFlowMetricsAggregatesSum2(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   180,
				ABPackets: 180,
				BABytes:   180,
				BAPackets: 180,
				Start:     0,
				Last:      30000,
			},
		},
	}

	expected := &flow.FlowMetric{
		ABBytes:   180,
		ABPackets: 180,
		BABytes:   180,
		BAPackets: 180,
		Start:     0,
		Last:      30000,
	}

	testMetricSum(t, metrics, expected, time.Unix(30, 0), 30*time.Second)
}

func TestFlowMetricsAggregatesSum3(t *testing.T) {
	metrics := map[string][]common.Metric{
		"aa": {
			&flow.FlowMetric{
				ABBytes:   5,
				ABPackets: 5,
				BABytes:   5,
				BAPackets: 5,
				Start:     0,
				Last:      30000,
			},
		},
	}

	expected := &flow.FlowMetric{
		ABBytes:   5,
		ABPackets: 5,
		BABytes:   5,
		BAPackets: 5,
		Start:     0,
		Last:      30000,
	}

	testMetricSum(t, metrics, expected, time.Unix(30, 0), 30*time.Second)
}
