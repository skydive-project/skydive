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
	"fmt"
	"sort"

	"github.com/skydive-project/skydive/common"
)

type FlowSetBandwidth struct {
	ABpackets int64
	ABbytes   int64
	BApackets int64
	BAbytes   int64
	Duration  int64
	NBFlow    uint64
}

type MergeContext struct {
	Sorted bool
	Dedup  bool
}

type sortByLast []*Flow

func NewFlowSet() *FlowSet {
	return &FlowSet{
		Flows: make([]*Flow, 0),
	}
}

func (fs *FlowSet) Merge(ofs *FlowSet, context MergeContext) {
	fs.Start = common.MinInt64(fs.Start, ofs.Start)
	if fs.Start == 0 {
		fs.Start = ofs.Start
	}
	fs.End = common.MaxInt64(fs.End, ofs.End)

	if context.Sorted {
		fs.Flows = fs.mergeFlows(fs.Flows, ofs.Flows, context)
	} else if context.Dedup {
		uuids := make(map[string]bool)
		for _, flow := range fs.Flows {
			uuids[flow.TrackingID] = true
		}

		for _, flow := range ofs.Flows {
			if !uuids[flow.TrackingID] {
				fs.Flows = append(fs.Flows, flow)
			}
		}
	} else {
		fs.Flows = append(fs.Flows, ofs.Flows...)
	}
}

func (fs *FlowSet) mergeFlows(left, right []*Flow, context MergeContext) []*Flow {
	var ret []*Flow

	if !context.Dedup {
		ret = make([]*Flow, 0, len(left)+len(right))
	}

	uuids := make(map[string]bool)
	for len(left) > 0 || len(right) > 0 {
		if len(left) == 0 {
			if context.Dedup {
				for _, flow := range right {
					if !uuids[flow.TrackingID] {
						ret = append(ret, flow)
						uuids[flow.TrackingID] = true
					}
				}
				return ret
			}
			return append(ret, right...)
		}
		if len(right) == 0 {
			if context.Dedup {
				for _, flow := range left {
					if !uuids[flow.TrackingID] {
						ret = append(ret, flow)
						uuids[flow.TrackingID] = true
					}
				}
				return ret
			}
			return append(ret, left...)
		}

		lf, rf := left[0], right[0]
		if lf.Metric.Last >= rf.Metric.Last {
			if !context.Dedup || !uuids[lf.TrackingID] {
				ret = append(ret, lf)
				uuids[lf.TrackingID] = true
			}
			left = left[1:]
		} else {
			if !context.Dedup || !uuids[rf.TrackingID] {
				ret = append(ret, rf)
				uuids[rf.TrackingID] = true
			}
			right = right[1:]
		}
	}
	return ret
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

func (fs *FlowSet) Dedup() {
	var deduped []*Flow

	uuids := make(map[string]bool)
	for _, flow := range fs.Flows {
		if _, ok := uuids[flow.TrackingID]; ok {
			continue
		}

		deduped = append(deduped, flow)
		uuids[flow.TrackingID] = true
	}
	fs.Flows = deduped
}

func (s sortByLast) Len() int {
	return len(s)
}

func (s sortByLast) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortByLast) Less(i, j int) bool {
	return s[i].Metric.Last > s[j].Metric.Last
}

func (fs *FlowSet) Sort() {
	sort.Sort(sortByLast(fs.Flows))
}

func (fs *FlowSet) AvgBandwidth() (fsbw FlowSetBandwidth) {
	if len(fs.Flows) == 0 {
		return
	}

	fsbw.Duration = fs.End - fs.Start
	for _, f := range fs.Flows {
		fstart := f.Metric.Start
		fend := f.Metric.Last

		fduration := fend - fstart
		if fduration == 0 {
			fduration = 1
		}

		fdurationWindow := common.MinInt64(fend, fs.End) - common.MaxInt64(fstart, fs.Start)
		if fdurationWindow == 0 {
			fdurationWindow = 1
		}

		m := f.Metric
		fsbw.ABpackets += m.ABPackets * fdurationWindow / fduration
		fsbw.ABbytes += m.ABBytes * fdurationWindow / fduration
		fsbw.BApackets += m.BAPackets * fdurationWindow / fduration
		fsbw.BAbytes += m.BABytes * fdurationWindow / fduration
		fsbw.NBFlow++
	}
	return
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

func (fs *FlowSet) Bandwidth() (fsbw FlowSetBandwidth) {
	for _, f := range fs.Flows {
		duration := f.LastUpdateMetric.Last - f.LastUpdateMetric.Start

		// set the duration to the largest flow duration. All the flow should
		// be close in term of duration ~= update timer
		if fsbw.Duration < duration {
			fsbw.Duration = duration
		}

		fsbw.ABpackets += f.LastUpdateMetric.ABPackets
		fsbw.ABbytes += f.LastUpdateMetric.ABBytes
		fsbw.BApackets += f.LastUpdateMetric.BAPackets
		fsbw.BAbytes += f.LastUpdateMetric.BABytes
		fsbw.NBFlow++
	}
	return
}

func (fsbw FlowSetBandwidth) String() string {
	return fmt.Sprintf("dt : %d seconds nbFlow %d\n\t\tAB -> BA\nPackets : %8d %8d\nBytes : %8d %8d\n",
		fsbw.Duration, fsbw.NBFlow, fsbw.ABpackets, fsbw.BApackets, fsbw.ABbytes, fsbw.BAbytes)
}
