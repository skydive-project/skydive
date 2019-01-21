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
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
)

const (
	defaultAggregatesSliceLength = int64(30000) // 30 seconds
)

// MetricsTraversalExtension describes a new extension to enhance the topology
type MetricsTraversalExtension struct {
	MetricsToken traversal.Token
}

// MetricsGremlinTraversalStep describes the Metrics gremlin traversal step
type MetricsGremlinTraversalStep struct {
	traversal.GremlinTraversalContext
	key string
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
	default:
		return nil, nil
	}
	var key string
	switch len(p.Params) {
	case 0:
		key = "LastUpdateMetric"
	case 1:
		k, ok := p.Params[0].(string)
		if !ok {
			return nil, errors.New("Metrics parameter have to be a string")
		}
		switch k {
		case "LastUpdateMetric", "SFlow.LastUpdateMetric", "sflow":
		default:
			return nil, fmt.Errorf("Metric field unknown : %v", p.Params)
		}
		if k == "sflow" {
			key = "SFlow.LastUpdateMetric"
		} else {
			key = k
		}
	default:
		return nil, fmt.Errorf("Metrics accepts one parameter : %v", p.Params)
	}

	return &MetricsGremlinTraversalStep{GremlinTraversalContext: p, key: key}, nil
}

// Exec executes the metrics step
func (s *MetricsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		return InterfaceMetrics(s.StepContext, tv, s.key), nil
	case *FlowTraversalStep:
		return tv.FlowMetrics(s.StepContext), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce metrics step
func (s *MetricsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context metrics step
func (s *MetricsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.GremlinTraversalContext
}

// MetricsTraversalStep traversal step metric interface counters
type MetricsTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	metrics        map[string][]common.Metric
	error          error
}

// Sum aggregates integer values mapped by 'key' cross flows
func (m *MetricsTraversalStep) Sum(ctx traversal.StepContext, keys ...interface{}) *traversal.GraphTraversalValue {
	if m.error != nil {
		return traversal.NewGraphTraversalValueFromError(m.error)
	}

	if len(keys) > 0 {
		if len(keys) != 1 {
			return traversal.NewGraphTraversalValueFromError(fmt.Errorf("Sum requires 1 parameter"))
		}

		key, ok := keys[0].(string)
		if !ok {
			return traversal.NewGraphTraversalValueFromError(errors.New("Argument of Sum must be a string"))
		}

		var total int64
		for _, metrics := range m.metrics {
			for _, metric := range metrics {
				value, err := metric.GetFieldInt64(key)
				if err != nil {
					return traversal.NewGraphTraversalValueFromError(err)
				}
				total += value
			}
		}
		return traversal.NewGraphTraversalValue(m.GraphTraversal, total)
	}

	var total common.Metric
	for _, metrics := range m.metrics {
		for _, metric := range metrics {
			if total == nil {
				total = metric
			} else {
				total = total.Add(metric)
			}

			if total.GetStart() > metric.GetStart() {
				total.SetStart(metric.GetStart())
			}

			if total.GetLast() < metric.GetLast() {
				total.SetLast(metric.GetLast())
			}
		}
	}

	return traversal.NewGraphTraversalValue(m.GraphTraversal, total)
}

func slice(m common.Metric, start, last int64) (common.Metric, common.Metric, common.Metric) {
	s1, s2 := m.Split(start)
	if s2 == nil || s2.IsZero() {
		return s1, nil, nil
	}

	s2, s3 := s2.Split(last)
	if s2 != nil && s2.IsZero() && m.GetStart() >= start && m.GetStart() < last {
		// slice metric to low and became zero due to ratio. In that case use it directly.
		return nil, m, nil
	}

	return s1, s2, s3
}

func aggregateMetrics(m []common.Metric, start, last int64, sliceLength int64, result []common.Metric) {
	boundM, boundR := len(m)-1, len(result)

	sStart, sLast := start, start+sliceLength

	var i, j int
	for j < boundR {
		if sLast > last {
			sLast = last
		}

		if i > boundM {
			break
		}

		_, s2, s3 := slice(m[i], sStart, sLast)
		if s2 != nil {
			if result[j] == nil {
				result[j] = s2
				result[j].SetStart(sStart)
				result[j].SetLast(sLast)
			} else {
				result[j] = result[j].Add(s2)
			}

			if s3 != nil && !s3.IsZero() {
				m[i] = s3
			} else {
				i++
			}
		} else {
			sStart += sliceLength
			sLast += sliceLength
			j++
		}
	}

	return
}

// Aggregates merges multiple metrics array into one by summing overlapping
// metrics. It returns a unique array will all the aggregated metrics.
func (m *MetricsTraversalStep) Aggregates(ctx traversal.StepContext, s ...interface{}) *MetricsTraversalStep {
	if m.error != nil {
		return NewMetricsTraversalStepFromError(m.error)
	}

	sliceLength := defaultAggregatesSliceLength
	if len(s) != 0 {
		sl, ok := s[0].(int64)
		if !ok || sl <= 0 {
			return NewMetricsTraversalStepFromError(fmt.Errorf("Aggregates parameter has to be a positive number"))
		}
		sliceLength = sl * 1000 // Millisecond
	}

	context := m.GraphTraversal.Graph.GetContext()

	var start, last int64
	if context.TimeSlice != nil {
		start, last = context.TimeSlice.Start, context.TimeSlice.Last
	} else {
		// no time context then take min/max of the metrics
		for _, array := range m.metrics {
			for _, metric := range array {
				if start == 0 || start > metric.GetStart() {
					start = metric.GetStart()
				}

				if last < metric.GetLast() {
					last = metric.GetLast()
				}
			}
		}
	}

	steps := (last - start) / sliceLength
	if (last-start)%sliceLength != 0 {
		steps++
	}

	aggregated := make([]common.Metric, steps, steps)
	for _, metrics := range m.metrics {
		aggregateMetrics(metrics, start, last, sliceLength, aggregated)
	}

	// filter out empty metrics
	final := make([]common.Metric, 0)
	for _, e := range aggregated {
		if e != nil {
			final = append(final, e)
		}
	}

	return NewMetricsTraversalStep(m.GraphTraversal, map[string][]common.Metric{"Aggregated": final})
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
func (m *MetricsTraversalStep) Count(ctx traversal.StepContext, s ...interface{}) *traversal.GraphTraversalValue {
	return traversal.NewGraphTraversalValue(m.GraphTraversal, len(m.metrics))
}

// PropertyKeys returns metric fields
func (m *MetricsTraversalStep) PropertyKeys(ctx traversal.StepContext, keys ...interface{}) *traversal.GraphTraversalValue {
	if m.error != nil {
		return traversal.NewGraphTraversalValueFromError(m.error)
	}

	var s []string

	if len(m.metrics) > 0 {
		for _, metrics := range m.metrics {
			// all Metric struct are the same, take the first one
			if len(metrics) > 0 {
				s = metrics[0].GetFieldKeys()
				break
			}
		}
	}

	return traversal.NewGraphTraversalValue(m.GraphTraversal, s)
}

// NewMetricsTraversalStep creates a new traversal metric step
func NewMetricsTraversalStep(gt *traversal.GraphTraversal, metrics map[string][]common.Metric) *MetricsTraversalStep {
	m := &MetricsTraversalStep{GraphTraversal: gt, metrics: metrics}
	return m
}

// NewMetricsTraversalStepFromError creates a new traversal metric step
func NewMetricsTraversalStepFromError(err error) *MetricsTraversalStep {
	m := &MetricsTraversalStep{error: err}
	return m
}
