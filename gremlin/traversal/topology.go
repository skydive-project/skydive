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

	"github.com/mitchellh/mapstructure"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

// InterfaceMetrics returns a Metrics step from interface metric metadata
func InterfaceMetrics(tv *traversal.GraphTraversalV) *MetricsTraversalStep {
	if tv.Error() != nil {
		return &MetricsTraversalStep{error: tv.Error()}
	}

	tv = tv.Dedup("ID", "LastUpdateMetric.Start").Sort(common.SortAscending, "LastUpdateMetric.Start")
	if tv.Error() != nil {
		return &MetricsTraversalStep{error: tv.Error()}
	}

	metrics := make(map[string][]common.Metric)
	it := tv.GraphTraversal.CurrentStepContext().PaginationRange.Iterator()
	gslice := tv.GraphTraversal.Graph.GetContext().TimeSlice

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.GetNodes() {
		if it.Done() {
			break nodeloop
		}

		m, _ := n.GetField("LastUpdateMetric")
		if m == nil {
			return nil
		}

		// NOTE(safchain) mapstructure for now, need to be change once converted from json to
		// protobuf
		var lastMetric topology.InterfaceMetric
		if err := mapstructure.WeakDecode(m, &lastMetric); err != nil {
			return &MetricsTraversalStep{error: err}
		}

		if gslice == nil || (lastMetric.Start > gslice.Start && lastMetric.Last < gslice.Last) {
			metrics[string(n.ID)] = append(metrics[string(n.ID)], &lastMetric)
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
