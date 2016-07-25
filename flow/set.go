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

import "github.com/skydive-project/skydive/common"

type FlowSet struct {
	Flows []*Flow
	Start int64
	End   int64
}

func NewFlowSet() *FlowSet {
	return &FlowSet{
		Flows: make([]*Flow, 0),
	}
}

func (fs *FlowSet) Merge(ofs *FlowSet) {

	fs.Start = common.MinInt64(fs.Start, ofs.Start)
	if fs.Start == 0 {
		fs.Start = ofs.Start
	}
	fs.End = common.MaxInt64(fs.End, ofs.End)

	fs.Flows = append(fs.Flows, ofs.Flows...)
}

func (fs *FlowSet) Bandwidth() (fsbw FlowSetBandwidth) {
	if len(fs.Flows) == 0 {
		return
	}

	fsbw.Duration = fs.End - fs.Start
	for _, f := range fs.Flows {
		fstart := f.Statistics.Start
		fend := f.Statistics.Last
		if fend == 0 {
			fend = fs.End
		}
		fduration := fend - fstart

		e := f.Statistics.GetEndpointsType(FlowEndpointType_ETHERNET)

		fdurationWindow := uint64(common.MinInt64(fend, fs.End) - common.MaxInt64(fstart, fs.Start))
		fsbw.ABpackets += uint64(e.AB.Packets * fdurationWindow / uint64(fduration))
		fsbw.ABbytes += uint64(e.AB.Bytes * fdurationWindow / uint64(fduration))
		fsbw.BApackets += uint64(e.BA.Packets * fdurationWindow / uint64(fduration))
		fsbw.BAbytes += uint64(e.BA.Bytes * fdurationWindow / uint64(fduration))
		fsbw.NBFlow++
	}
	return
}
