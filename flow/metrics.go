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
)

// SetStart set Start field
func (fm *FlowMetric) SetStart(start int64) {
	fm.Start = start
}

// SetLast set Last field
func (fm *FlowMetric) SetLast(last int64) {
	fm.Last = last
}

// Copy a flow metric
func (fm *FlowMetric) Copy() *FlowMetric {
	return &FlowMetric{
		ABPackets: fm.ABPackets,
		ABBytes:   fm.ABBytes,
		BAPackets: fm.BAPackets,
		BABytes:   fm.BABytes,
		Start:     fm.Start,
		Last:      fm.Last,
	}
}

// Copy TCP metric
func (tm *TCPMetric) Copy() *TCPMetric {
	return &TCPMetric{
		ABSynStart:            tm.ABSynStart,
		BASynStart:            tm.BASynStart,
		ABSynTTL:              tm.ABSynTTL,
		BASynTTL:              tm.BASynTTL,
		ABFinStart:            tm.ABFinStart,
		BAFinStart:            tm.BAFinStart,
		ABRstStart:            tm.ABRstStart,
		BARstStart:            tm.BARstStart,
		ABSegmentOutOfOrder:   tm.ABSegmentOutOfOrder,
		ABSegmentSkipped:      tm.ABSegmentSkipped,
		ABSegmentSkippedBytes: tm.ABSegmentSkippedBytes,
		ABPackets:             tm.ABPackets,
		ABBytes:               tm.ABBytes,
		ABSawStart:            tm.ABSawStart,
		ABSawEnd:              tm.ABSawEnd,
		BASegmentOutOfOrder:   tm.BASegmentOutOfOrder,
		BASegmentSkipped:      tm.BASegmentSkipped,
		BASegmentSkippedBytes: tm.BASegmentSkippedBytes,
		BAPackets:             tm.BAPackets,
		BABytes:               tm.BABytes,
		BASawStart:            tm.BASawStart,
		BASawEnd:              tm.BASawEnd,
	}
}

// GetFieldInt64 returns the field value
func (fm *FlowMetric) GetFieldInt64(field string) (int64, error) {
	switch field {
	case "ABPackets":
		return fm.ABPackets, nil
	case "ABBytes":
		return fm.ABBytes, nil
	case "BAPackets":
		return fm.BAPackets, nil
	case "BABytes":
		return fm.BABytes, nil
	}
	return 0, common.ErrFieldNotFound
}

// Add sum flow metrics
func (fm *FlowMetric) Add(m common.Metric) common.Metric {
	f2 := m.(*FlowMetric)

	return &FlowMetric{
		ABBytes:   fm.ABBytes + f2.ABBytes,
		BABytes:   fm.BABytes + f2.BABytes,
		ABPackets: fm.ABPackets + f2.ABPackets,
		BAPackets: fm.BAPackets + f2.BAPackets,
		Start:     fm.Start,
		Last:      fm.Last,
	}
}

// Sub subtracts flow metrics
func (fm *FlowMetric) Sub(m common.Metric) common.Metric {
	f2 := m.(*FlowMetric)

	return &FlowMetric{
		ABBytes:   fm.ABBytes - f2.ABBytes,
		BABytes:   fm.BABytes - f2.BABytes,
		ABPackets: fm.ABPackets - f2.ABPackets,
		BAPackets: fm.BAPackets - f2.BAPackets,
		Start:     fm.Start,
		Last:      fm.Last,
	}
}

// IsZero returns true if all the values are equal to zero
func (fm *FlowMetric) IsZero() bool {
	// sum as these numbers can't be <= 0
	return (fm.ABBytes +
		fm.ABPackets +
		fm.BABytes +
		fm.BAPackets) == 0
}

func (fm *FlowMetric) applyRatio(ratio float64) *FlowMetric {
	return &FlowMetric{
		ABBytes:   int64(float64(fm.ABBytes) * ratio),
		ABPackets: int64(float64(fm.ABPackets) * ratio),
		BABytes:   int64(float64(fm.BABytes) * ratio),
		BAPackets: int64(float64(fm.BAPackets) * ratio),
		Start:     fm.Start,
		Last:      fm.Last,
	}
}

// Slice splits a Metric into two parts
func (fm *FlowMetric) Split(cut int64) (common.Metric, common.Metric) {
	if cut < fm.Start {
		return nil, fm
	} else if cut > fm.Last {
		return fm, nil
	} else if fm.Start == fm.Last {
		return fm, nil
	} else if cut == fm.Start {
		return nil, fm
	} else if cut == fm.Last {
		return fm, nil
	}

	duration := float64(fm.Last - fm.Start)
	ratio := float64(cut-fm.Start) / duration

	m1 := fm.applyRatio(ratio)
	m1.Last = cut

	m2 := fm.Sub(m1)
	m2.(*FlowMetric).Start = cut

	return m1, m2
}
