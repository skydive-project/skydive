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
	"strings"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

// Metrics step : packets counters
func InterfaceMetrics(tv *traversal.GraphTraversalV) *MetricsTraversalStep {
	if tv.Error() != nil {
		return &MetricsTraversalStep{error: tv.Error()}
	}

	tv = tv.Dedup("ID", "LastMetric.Start").Sort(common.SortAscending, "LastMetric.Start")
	if tv.Error() != nil {
		return &MetricsTraversalStep{error: tv.Error()}
	}

	metrics := make(map[string][]*common.TimedMetric)
	it := tv.GraphTraversal.CurrentStepContext().PaginationRange.Iterator()
	gslice := tv.GraphTraversal.Graph.GetContext().TimeSlice

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.GetNodes() {
		if it.Done() {
			break nodeloop
		}

		m := n.Metadata()
		lastMetric, hasLastMetric := m["LastMetric"].(map[string]interface{})
		if hasLastMetric {
			start := lastMetric["Start"].(int64)
			last := lastMetric["Last"].(int64)
			if gslice == nil || (start > gslice.Start && last < gslice.Last) {
				im := &topology.InterfaceMetric{
					RxPackets:         lastMetric["RxPackets"].(int64),
					TxPackets:         lastMetric["TxPackets"].(int64),
					RxBytes:           lastMetric["RxBytes"].(int64),
					TxBytes:           lastMetric["TxBytes"].(int64),
					RxErrors:          lastMetric["RxErrors"].(int64),
					TxErrors:          lastMetric["TxErrors"].(int64),
					RxDropped:         lastMetric["RxDropped"].(int64),
					TxDropped:         lastMetric["TxDropped"].(int64),
					Multicast:         lastMetric["Multicast"].(int64),
					Collisions:        lastMetric["Collisions"].(int64),
					RxLengthErrors:    lastMetric["RxLengthErrors"].(int64),
					RxOverErrors:      lastMetric["RxOverErrors"].(int64),
					RxCrcErrors:       lastMetric["RxCrcErrors"].(int64),
					RxFrameErrors:     lastMetric["RxFrameErrors"].(int64),
					RxFifoErrors:      lastMetric["RxFifoErrors"].(int64),
					RxMissedErrors:    lastMetric["RxMissedErrors"].(int64),
					TxAbortedErrors:   lastMetric["TxAbortedErrors"].(int64),
					TxCarrierErrors:   lastMetric["TxCarrierErrors"].(int64),
					TxFifoErrors:      lastMetric["TxFifoErrors"].(int64),
					TxHeartbeatErrors: lastMetric["TxHeartbeatErrors"].(int64),
					TxWindowErrors:    lastMetric["TxWindowErrors"].(int64),
					RxCompressed:      lastMetric["RxCompressed"].(int64),
					TxCompressed:      lastMetric["TxCompressed"].(int64),
				}
				metric := &common.TimedMetric{
					TimeSlice: *common.NewTimeSlice(start, last),
					Metric:    im,
				}
				metrics[string(n.ID)] = append(metrics[string(n.ID)], metric)
			}
		}
	}

	return NewMetricsTraversalStep(tv.GraphTraversal, metrics, nil)
}

// TopologyGremlinQuery run a gremlin query on the graph g without any extension
func TopologyGremlinQuery(g *graph.Graph, query string) (traversal.GraphTraversalStep, error) {
	tr := traversal.NewGremlinTraversalParser()
	ts, err := tr.Parse(strings.NewReader(query))
	if err != nil {
		return nil, err
	}

	return ts.Exec(g, false)
}
