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
	"errors"
	"strings"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology/probes/socketinfo"
)

// InterfaceMetrics returns a Metrics step from interface metric metadata
func InterfaceMetrics(ctx traversal.StepContext, tv *traversal.GraphTraversalV, key string) *MetricsTraversalStep {
	if tv.Error() != nil {
		return NewMetricsTraversalStepFromError(tv.Error())
	}

	startField := key + ".Start"

	tv = tv.Dedup(ctx, "ID", startField).Sort(ctx, common.SortAscending, startField)

	if tv.Error() != nil {
		return NewMetricsTraversalStepFromError(tv.Error())
	}

	metrics := make(map[string][]common.Metric)
	it := ctx.PaginationRange.Iterator()
	gslice := tv.GraphTraversal.Graph.GetContext().TimeSlice

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

nodeloop:
	for _, n := range tv.GetNodes() {
		if it.Done() {
			break nodeloop
		}

		m, _ := n.GetField(key)
		if m == nil {
			continue
		}

		lastmetric, ok := m.(common.Metric)
		if !ok {
			return NewMetricsTraversalStepFromError(errors.New("wrong interface metric type"))
		}

		if gslice == nil || (lastmetric.GetStart() > gslice.Start && lastmetric.GetLast() < gslice.Last) && it.Next() {
			metrics[string(n.ID)] = append(metrics[string(n.ID)], lastmetric)
		}
	}

	return NewMetricsTraversalStep(tv.GraphTraversal, metrics)
}

// Sockets returns a sockets step from host/namespace sockets
func Sockets(ctx traversal.StepContext, tv *traversal.GraphTraversalV) *SocketsTraversalStep {
	if tv.Error() != nil {
		return &SocketsTraversalStep{error: tv.Error()}
	}

	it := ctx.PaginationRange.Iterator()

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	sockets := make(map[string][]*socketinfo.ConnectionInfo)
nodeloop:
	for _, n := range tv.GetNodes() {
		if it.Done() {
			break nodeloop
		}

		m, _ := n.GetField("Sockets")
		if m == nil {
			continue
		}

		id := string(n.ID)
		if _, found := sockets[id]; !found {
			sockets[id] = make([]*socketinfo.ConnectionInfo, 0)
		}

		for _, socket := range getSockets(n) {
			if it.Done() {
				break
			} else if it.Next() {
				sockets[id] = append(sockets[id], socket)
			}
		}
	}

	return &SocketsTraversalStep{GraphTraversal: tv.GraphTraversal, sockets: sockets}
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
