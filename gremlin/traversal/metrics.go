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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

// MetricsTraversalExtension describes a new extension to enhance the topology
type MetricsTraversalExtension struct {
	MetricsToken traversal.Token
}

type MetricsGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

// NewMetricsTraversalExtension returns a new graph traversal extension
func NewMetricsTraversalExtension() *MetricsTraversalExtension {
	return &MetricsTraversalExtension{
		MetricsToken: traversalMetricsToken,
	}
}

// ScanIdent returns an associated graph token
func (e *MetricsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "METRICS":
		return e.MetricsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parse metrics step
func (e *MetricsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.MetricsToken:
		return &MetricsGremlinTraversalStep{context: p}, nil
	}
	return nil, nil
}

// Exec executes the metrics step
func (s *MetricsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch last.(type) {
	case *traversal.GraphTraversalV:
		tv := last.(*traversal.GraphTraversalV)
		return InterfaceMetrics(tv), nil
	case *FlowTraversalStep:
		fs := last.(*FlowTraversalStep)
		return fs.FlowMetrics(), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce flow step
func (s *MetricsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

func (s *MetricsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

// MetricsTraversalStep traversal step metric interface counters
type MetricsTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	metrics        map[string][]*common.TimedMetric
	error          error
}

// Sum aggregates integer values mapped by 'key' cross flows
func (m *MetricsTraversalStep) Sum(keys ...interface{}) *traversal.GraphTraversalValue {
	if m.error != nil {
		return traversal.NewGraphTraversalValue(m.GraphTraversal, nil, m.error)
	}

	if len(keys) > 0 {
		if len(keys) != 1 {
			return traversal.NewGraphTraversalValue(m.GraphTraversal, nil, fmt.Errorf("Sum requires 1 parameter"))
		}

		key, ok := keys[0].(string)
		if !ok {
			return traversal.NewGraphTraversalValue(m.GraphTraversal, nil, errors.New("Argument of Sum must be a string"))
		}

		var total int64
		for _, metrics := range m.metrics {
			for _, metric := range metrics {
				value, err := metric.GetFieldInt64(key)
				if err != nil {
					return traversal.NewGraphTraversalValue(m.GraphTraversal, nil, err)
				}
				total += value
			}
		}
		return traversal.NewGraphTraversalValue(m.GraphTraversal, total)
	}

	total := common.TimedMetric{}
	for _, metrics := range m.metrics {
		for _, metric := range metrics {
			if total.Metric == nil {
				total.Metric = metric.Metric
			} else {
				total.Metric.Add(metric.Metric)
			}

			if total.Start == 0 || total.Start > metric.Start {
				total.Start = metric.Start
			}

			if total.Last == 0 || total.Last < metric.Last {
				total.Last = metric.Last
			}
		}
	}

	return traversal.NewGraphTraversalValue(m.GraphTraversal, &total)
}

func aggregateMetrics(a, b []*common.TimedMetric) []*common.TimedMetric {
	var result []*common.TimedMetric
	boundA, boundB := len(a)-1, len(b)-1

	var i, j int
	for i <= boundA || j <= boundB {
		if i > boundA && j <= boundB {
			return append(result, b[j:]...)
		} else if j > boundB && i <= boundA {
			return append(result, a[i:]...)
		} else if a[i].Last < b[j].Start {
			// metric a is strictly before metric b
			result = append(result, a[i])
			i++
		} else if b[j].Last < a[i].Start {
			// metric b is strictly before metric a
			result = append(result, b[j])
			j++
		} else {
			start := a[i].Start
			last := a[i].Last
			if a[i].Start > b[j].Start {
				start = b[j].Start
				last = b[j].Last
			}

			// in case of an overlap then summing using the smallest start/last slice
			var metric = a[i].Metric
			metric.Add(b[j].Metric)

			result = append(result, &common.TimedMetric{
				Metric:    metric,
				TimeSlice: *common.NewTimeSlice(start, last),
			})
			i++
			j++
		}
	}
	return result
}

// Aggregates merges multiple metrics array into one by summing overlapping
// metrics. It returns a unique array will all the aggregated metrics.
func (m *MetricsTraversalStep) Aggregates() *MetricsTraversalStep {
	if m.error != nil {
		return m
	}

	var aggregated []*common.TimedMetric
	for _, metrics := range m.metrics {
		aggregated = aggregateMetrics(aggregated, metrics)
	}

	return &MetricsTraversalStep{GraphTraversal: m.GraphTraversal, metrics: map[string][]*common.TimedMetric{"Aggregated": aggregated}}
}

// Values returns the graph metric values
func (m *MetricsTraversalStep) Values() []interface{} {
	if len(m.metrics) == 0 {
		return []interface{}{}
	}
	return []interface{}{m.metrics}
}

// MarshalJSON serialize in JSON
func (m *MetricsTraversalStep) MarshalJSON() ([]byte, error) {
	values := m.Values()
	m.GraphTraversal.RLock()
	defer m.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

// Error returns error present at this step
func (m *MetricsTraversalStep) Error() error {
	return m.error
}

// Count step
func (m *MetricsTraversalStep) Count(s ...interface{}) *traversal.GraphTraversalValue {
	return traversal.NewGraphTraversalValue(m.GraphTraversal, len(m.metrics))
}

// NewMetricsTraversalStep creates a new tranversal metric step
func NewMetricsTraversalStep(gt *traversal.GraphTraversal, metrics map[string][]*common.TimedMetric, err error) *MetricsTraversalStep {
	return &MetricsTraversalStep{GraphTraversal: gt, metrics: metrics, error: err}
}
