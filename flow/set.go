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
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
)

// MergeContext describes a mechanism to merge flow sets
type MergeContext struct {
	Sort      bool
	SortBy    string
	SortOrder common.SortOrder
	Dedup     bool
	DedupBy   string
}

// NewFlowSet creates a new empty FlowSet
func NewFlowSet() *FlowSet {
	return &FlowSet{
		Flows: make([]*Flow, 0),
	}
}

func getDedupField(flow *Flow, field string) (interface{}, error) {
	if field == "" {
		return flow.TrackingID, nil
	}

	if v, err := flow.GetFieldString(field); err == nil {
		return v, nil
	}
	return flow.GetFieldInt64(field)
}

func compareByField(lf, rf *Flow, field string) (bool, error) {
	if field == "" {
		return lf.Last <= rf.Last, nil
	}

	// compare first int64 has it is more often used with sort
	if v1, err := lf.GetFieldInt64(field); err == nil {
		v2, _ := rf.GetFieldInt64(field)
		return v1 <= v2, nil
	}

	if v1, err := lf.GetFieldString(field); err == nil {
		v2, _ := rf.GetFieldString(field)
		return v1 <= v2, nil
	}

	return false, common.ErrFieldNotFound
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

func (fs *FlowSet) sortFlows(f []*Flow, context MergeContext) []*Flow {
	if len(f) <= 1 {
		return f
	}

	mid := len(f) / 2
	left := f[:mid]
	right := f[mid:]

	left = fs.sortFlows(left, context)
	right = fs.sortFlows(right, context)

	flows, _ := fs.mergeSortedFlows(left, right, context)

	return flows
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
		cv, err := compareByField(lf, rf, context.SortBy)
		if err != nil {
			return nil, err
		}
		if context.SortOrder != common.SortAscending {
			cv = !cv
		}

		if cv {
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

// Slice returns a slice of a FlowSet
func (fs *FlowSet) Slice(from, to int) {
	if from > len(fs.Flows) {
		from = len(fs.Flows)
	}
	if to > len(fs.Flows) {
		to = len(fs.Flows)
	}

	fs.Flows = fs.Flows[from:to]
}

// Dedup deduplicate a flows in a FlowSet
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

// Sort flows in a FlowSet
func (fs *FlowSet) Sort(order common.SortOrder, orberBy string) {
	context := MergeContext{Sort: true, SortBy: orberBy, SortOrder: order}
	fs.Flows = fs.sortFlows(fs.Flows, context)
}

// Filter flows in a FlowSet
func (fs *FlowSet) Filter(filter *filters.Filter) *FlowSet {
	flowset := NewFlowSet()
	for _, f := range fs.Flows {
		if filter == nil || filter.Eval(f) {
			if flowset.Start == 0 || flowset.Start > f.Start {
				flowset.Start = f.Start
			}
			if flowset.End == 0 || flowset.Start < f.Last {
				flowset.End = f.Last
			}
			flowset.Flows = append(flowset.Flows, f)
		}
	}
	return flowset
}
