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
	"sort"

	"github.com/skydive-project/skydive/common"
)

type MergeContext struct {
	Sort    bool
	SortBy  string
	Dedup   bool
	DedupBy string
}

type sortFlows []*Flow
type sortByStart struct{ sortFlows }
type sortByLast struct{ sortFlows }
type sortByABPackets struct{ sortFlows }
type sortByABBytes struct{ sortFlows }
type sortByBAPackets struct{ sortFlows }
type sortByBABytes struct{ sortFlows }

func NewFlowSet() *FlowSet {
	return &FlowSet{
		Flows: make([]*Flow, 0),
	}
}

func getDedupField(flow *Flow, field string) (string, error) {
	if field == "" {
		return flow.TrackingID, nil
	}

	// only flow string field are support for dedup as only few make sense
	// for dedup like ANodeTID, NodeTID, etc.
	return flow.GetFieldString(field)
}

func getSortField(flow *Flow, field string) int64 {
	if field == "" {
		return flow.Metric.Last
	}
	val, err := flow.GetFieldInt64(field)
	if err != nil {
		return flow.Metric.Last
	}
	return val
}

// mergeDedup merges the flowset given as argument. Both of the flowset have
// to be dedup before calling this function.
func (fs *FlowSet) mergeDedup(ofs *FlowSet, field string) error {
	if len(ofs.Flows) == 0 {
		return nil
	}

	visited := make(map[interface{}]bool)
	for _, flow := range fs.Flows {
		kvisited, err := getDedupField(flow, field)
		if err != nil {
			return err
		}
		visited[kvisited] = true
	}

	for _, flow := range ofs.Flows {
		kvisited, err := getDedupField(flow, field)
		if err != nil {
			return err
		}

		if _, ok := visited[kvisited]; !ok {
			fs.Flows = append(fs.Flows, flow)
		}
	}

	return nil
}

// Merge merges two FlowSet. If Sorted both of the FlowSet have to be sorted
// first. If Dedup both of the FlowSet have to be dedup first too.
func (fs *FlowSet) Merge(ofs *FlowSet, context MergeContext) error {
	fs.Start = common.MinInt64(fs.Start, ofs.Start)
	if fs.Start == 0 {
		fs.Start = ofs.Start
	}
	fs.End = common.MaxInt64(fs.End, ofs.End)

	var err error
	if context.Sort {
		if fs.Flows, err = fs.mergeSortedFlows(fs.Flows, ofs.Flows, context); err != nil {
			return err
		}
	} else if context.Dedup {
		return fs.mergeDedup(ofs, context.DedupBy)
	} else {
		fs.Flows = append(fs.Flows, ofs.Flows...)
	}

	return nil
}

func (fs *FlowSet) mergeSortedFlows(left, right []*Flow, context MergeContext) ([]*Flow, error) {
	var ret []*Flow

	if !context.Dedup {
		ret = make([]*Flow, 0, len(left)+len(right))
	}

	visited := make(map[interface{}]bool)
	for len(left) > 0 || len(right) > 0 {
		if len(left) == 0 {
			if context.Dedup {
				for _, flow := range right {
					kvisited, err := getDedupField(flow, context.DedupBy)
					if err != nil {
						return ret, err
					}

					if _, ok := visited[kvisited]; !ok {
						ret = append(ret, flow)
						visited[kvisited] = true
					}
				}
				return ret, nil
			}
			return append(ret, right...), nil
		}
		if len(right) == 0 {
			if context.Dedup {
				for _, flow := range left {
					kvisited, err := getDedupField(flow, context.DedupBy)
					if err != nil {
						return ret, err
					}

					if _, ok := visited[kvisited]; !ok {
						ret = append(ret, flow)
						visited[kvisited] = true
					}
				}
				return ret, nil
			}
			return append(ret, left...), nil
		}

		lf, rf := left[0], right[0]
		if getSortField(lf, context.SortBy) >= getSortField(rf, context.SortBy) {
			if !context.Dedup {
				ret = append(ret, lf)
			} else {
				kvisited, err := getDedupField(lf, context.DedupBy)
				if err != nil {
					return ret, err
				}

				if _, ok := visited[kvisited]; !ok {
					ret = append(ret, lf)
					visited[kvisited] = true
				}
			}
			left = left[1:]
		} else {
			if !context.Dedup {
				ret = append(ret, rf)
			} else {
				kvisited, err := getDedupField(rf, context.DedupBy)
				if err != nil {
					return ret, err
				}

				if _, ok := visited[kvisited]; !ok {
					ret = append(ret, rf)
					visited[kvisited] = true
				}
			}

			right = right[1:]
		}
	}

	return ret, nil
}

func (fs *FlowSet) Slice(from, to int) {
	if from > len(fs.Flows) {
		from = len(fs.Flows)
	}
	if to > len(fs.Flows) {
		to = len(fs.Flows)
	}

	fs.Flows = fs.Flows[from:to]
}

func (fs *FlowSet) Dedup(field string) error {
	var deduped []*Flow

	if len(fs.Flows) == 0 {
		return nil
	}

	visited := make(map[interface{}]bool)
	for _, flow := range fs.Flows {
		kvisited, err := getDedupField(flow, field)
		if err != nil {
			return err
		}

		if _, ok := visited[kvisited]; !ok {
			deduped = append(deduped, flow)
			visited[kvisited] = true
		}
	}
	fs.Flows = deduped

	return nil
}

func (s sortFlows) Len() int {
	return len(s)
}

func (s sortFlows) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortByLast) Less(i, j int) bool {
	return s.sortFlows[i].Metric.Last > s.sortFlows[j].Metric.Last
}

func (s sortByStart) Less(i, j int) bool {
	return s.sortFlows[i].Metric.Start > s.sortFlows[j].Metric.Start
}

func (s sortByABPackets) Less(i, j int) bool {
	return s.sortFlows[i].Metric.ABPackets > s.sortFlows[j].Metric.ABPackets
}

func (s sortByABBytes) Less(i, j int) bool {
	return s.sortFlows[i].Metric.ABBytes > s.sortFlows[j].Metric.ABBytes
}

func (s sortByBAPackets) Less(i, j int) bool {
	return s.sortFlows[i].Metric.BAPackets > s.sortFlows[j].Metric.BAPackets
}

func (s sortByBABytes) Less(i, j int) bool {
	return s.sortFlows[i].Metric.BABytes > s.sortFlows[j].Metric.BABytes
}

func (fs *FlowSet) Sort(field string) {
	switch field {
	case "Metric.Start":
		sort.Sort(sortByStart{fs.Flows})
	case "Metric.Last":
		sort.Sort(sortByLast{fs.Flows})
	case "Metric.ABPackets":
		sort.Sort(sortByABPackets{fs.Flows})
	case "Metric.ABBytes":
		sort.Sort(sortByABBytes{fs.Flows})
	case "Metric.BAPackets":
		sort.Sort(sortByBAPackets{fs.Flows})
	case "Metric.BABytes":
		sort.Sort(sortByBABytes{fs.Flows})
	}
}

func (fs *FlowSet) Filter(filter *Filter) *FlowSet {
	flowset := NewFlowSet()
	for _, f := range fs.Flows {
		if filter == nil || filter.Eval(f) {
			if flowset.Start == 0 || flowset.Start > f.Metric.Start {
				flowset.Start = f.Metric.Start
			}
			if flowset.End == 0 || flowset.Start < f.Metric.Last {
				flowset.End = f.Metric.Last
			}
			flowset.Flows = append(flowset.Flows, f)
		}
	}
	return flowset
}
