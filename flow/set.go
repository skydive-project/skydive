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

	"github.com/skydive-project/skydive/common"
)

type FlowSetBandwidth struct {
	ABpackets uint64
	ABbytes   uint64
	BApackets uint64
	BAbytes   uint64
	Duration  int64
	NBFlow    uint64
}

func NewFlowSet() *FlowSet {
	return &FlowSet{
		Flows: make([]*Flow, 0),
	}
}

func (fs *FlowSet) Merge(ofs *FlowSet, sorted bool) {
	fs.Start = common.MinInt64(fs.Start, ofs.Start)
	if fs.Start == 0 {
		fs.Start = ofs.Start
	}
	fs.End = common.MaxInt64(fs.End, ofs.End)

	if sorted {
		fs.Flows = fs.mergeFlows(fs.Flows, ofs.Flows)
	} else {
		fs.Flows = append(fs.Flows, ofs.Flows...)
	}
}

func (fs *FlowSet) mergeFlows(left, right []*Flow) []*Flow {
	ret := make([]*Flow, 0, len(left)+len(right))
	for len(left) > 0 || len(right) > 0 {
		if len(left) == 0 {
			return append(ret, right...)
		}
		if len(right) == 0 {
			return append(ret, left...)
		}
		if left[0].Metric.Last >= right[0].Metric.Last {
			ret = append(ret, left[0])
			left = left[1:]
		} else {
			ret = append(ret, right[0])
			right = right[1:]
		}
	}
	return ret
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

		fdurationWindow := uint64(common.MinInt64(fend, fs.End) - common.MaxInt64(fstart, fs.Start))
		if fdurationWindow == 0 {
			fdurationWindow = 1
		}

		m := f.Metric
		fsbw.ABpackets += uint64(m.ABPackets * fdurationWindow / uint64(fduration))
		fsbw.ABbytes += uint64(m.ABBytes * fdurationWindow / uint64(fduration))
		fsbw.BApackets += uint64(m.BAPackets * fdurationWindow / uint64(fduration))
		fsbw.BAbytes += uint64(m.BABytes * fdurationWindow / uint64(fduration))
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
