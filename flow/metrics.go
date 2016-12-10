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

// Copy a flow metric
func (fm *FlowMetric) Copy() *FlowMetric {
	return &FlowMetric{
		ABPackets: fm.ABPackets,
		ABBytes:   fm.ABBytes,
		BAPackets: fm.BAPackets,
		BABytes:   fm.BABytes,
	}
}

// Copy extended metric
func (fm *ExtFlowMetric) Copy() *ExtFlowMetric {
	return &ExtFlowMetric{
		ABPackets:     fm.ABPackets,
		ABBytes:       fm.ABBytes,
		BAPackets:     fm.BAPackets,
		BABytes:       fm.BABytes,
		ABSynStart:    fm.ABSynStart,
		BASynStart:    fm.BASynStart,
		ABSynData:     fm.ABSynData,
		BASynData:     fm.BASynData,
		ABSynTTL:      fm.ABSynTTL,
		BASynTTL:      fm.BASynTTL,
		CaptureSyn:    fm.CaptureSyn,
		ABFinStart:    fm.ABFinStart,
		BAFinStart:    fm.BAFinStart,
		ABRstStart:    fm.ABRstStart,
		BARstStart:    fm.BARstStart,
		ABExpectedSeq: fm.ABExpectedSeq,
		BAExpectedSeq: fm.BAExpectedSeq,
		LenBySeq:      fm.LenBySeq,
	}
}

// GetFieldInt64 returns the field value
func (f *FlowMetric) GetFieldInt64(field string) (int64, error) {
	switch field {
	case "ABPackets":
		return f.ABPackets, nil
	case "ABBytes":
		return f.ABBytes, nil
	case "BAPackets":
		return f.BAPackets, nil
	case "BABytes":
		return f.BABytes, nil
	}
	return 0, common.ErrFieldNotFound
}

// GetFieldInt64 for extended metric
func (f *ExtFlowMetric) GetFieldInt64(field string) (int64, error) {

	switch field {
	case "ABPackets":
		return f.ABPackets, nil
	case "ABBytes":
		return f.ABBytes, nil
	case "BAPackets":
		return f.BAPackets, nil
	case "BABytes":
		return f.BABytes, nil
	}
	return 0, common.ErrFieldNotFound
}

// Add sum flow metrics
func (f *FlowMetric) Add(m common.Metric) common.Metric {
	f2 := m.(*FlowMetric)

	f.ABBytes += f2.ABBytes
	f.BABytes += f2.BABytes
	f.ABPackets += f2.ABPackets
	f.BAPackets += f2.BAPackets

	return f
}

// Add sum for extended metric
func (f *ExtFlowMetric) Add(m common.Metric) common.Metric {

	f2 := m.(*ExtFlowMetric)

	f.ABBytes += f2.ABBytes
	f.BABytes += f2.BABytes
	f.ABPackets += f2.ABPackets
	f.BAPackets += f2.BAPackets

	return f
}
