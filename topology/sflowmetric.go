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

package topology

import (
	json "encoding/json"

	"github.com/skydive-project/skydive/common"
)

// SFlowMetric the interface packets counters
// easyjson:json
type SFlowMetric struct {
	Start              int64 `json:"Start,omitempty"`
	Last               int64 `json:"Last,omitempty"`
	IfIndex            int64 `json:"IfIndex,omitempty"`
	IfType             int64 `json:"IfType,omitempty"`
	IfSpeed            int64 `json:"IfSpeed,omitempty"`
	IfDirection        int64 `json:"IfDirection,omitempty"`
	IfStatus           int64 `json:"IfStatus,omitempty"`
	IfInOctets         int64 `json:"IfInOctets,omitempty"`
	IfInUcastPkts      int64 `json:"IfInUcastPkts,omitempty"`
	IfInMulticastPkts  int64 `json:"IfInMulticastPkts,omitempty"`
	IfInBroadcastPkts  int64 `json:"IfInBroadcastPkts,omitempty"`
	IfInDiscards       int64 `json:"IfInDiscards,omitempty"`
	IfInErrors         int64 `json:"IfInErrors,omitempty"`
	IfInUnknownProtos  int64 `json:"IfInUnknownProtos,omitempty"`
	IfOutOctets        int64 `json:"IfOutOctets,omitempty"`
	IfOutUcastPkts     int64 `json:"IfOutUcastPkts,omitempty"`
	IfOutMulticastPkts int64 `json:"IfOutMulticastPkts,omitempty"`
	IfOutBroadcastPkts int64 `json:"IfOutBroadcastPkts,omitempty"`
	IfOutDiscards      int64 `json:"IfOutDiscards,omitempty"`
	IfOutErrors        int64 `json:"IfOutErrors,omitempty"`
	IfPromiscuousMode  int64 `json:"IfPromiscuousMode,omitempty"`
}

// SFlowMetricMetadataDecoder implements a json message raw decoder
func SFlowMetricMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var metric SFlowMetric
	if err := json.Unmarshal(raw, &metric); err != nil {
		return nil, err
	}

	return &metric, nil
}

// GetStart returns start time
func (im *SFlowMetric) GetStart() int64 {
	return im.Start
}

// SetStart set start time
func (im *SFlowMetric) SetStart(start int64) {
	im.Start = start
}

// GetLast returns last time
func (im *SFlowMetric) GetLast() int64 {
	return im.Last
}

// SetLast set last tome
func (im *SFlowMetric) SetLast(last int64) {
	im.Last = last
}

// GetFieldInt64 implements Getter and SFlowMetrics interfaces
func (im *SFlowMetric) GetFieldInt64(field string) (int64, error) {
	switch field {
	case "Start":
		return im.Start, nil
	case "Last":
		return im.Last, nil
	case "IfIndex":
		return im.IfIndex, nil
	case "IfType":
		return im.IfType, nil
	case "IfSpeed":
		return im.IfSpeed, nil
	case "IfDirection":
		return im.IfDirection, nil
	case "IfStatus":
		return im.IfStatus, nil
	case "IfInOctets":
		return im.IfInOctets, nil
	case "IfInUcastPkts":
		return im.IfInUcastPkts, nil
	case "IfInMulticastPkts":
		return im.IfInMulticastPkts, nil
	case "IfInBroadcastPkts":
		return im.IfInBroadcastPkts, nil
	case "IfInDiscards":
		return im.IfInDiscards, nil
	case "IfInErrors":
		return im.IfInErrors, nil
	case "IfInUnknownProtos":
		return im.IfInUnknownProtos, nil
	case "IfOutOctets":
		return im.IfOutOctets, nil
	case "IfOutUcastPkts":
		return im.IfOutUcastPkts, nil
	case "IfOutMulticastPkts":
		return im.IfOutMulticastPkts, nil
	case "IfOutBroadcastPkts":
		return im.IfOutBroadcastPkts, nil
	case "IfOutDiscards":
		return im.IfOutDiscards, nil
	case "IfOutErrors":
		return im.IfOutErrors, nil
	case "IfPromiscuousMode":
		return im.IfPromiscuousMode, nil
	}

	return 0, common.ErrFieldNotFound
}

// GetField implements Getter interface
func (im *SFlowMetric) GetField(key string) (interface{}, error) {
	return im.GetFieldInt64(key)
}

// GetFieldString implements Getter interface
func (im *SFlowMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

// Add sum two metrics and return a new SFlowMetrics object
func (im *SFlowMetric) Add(m common.Metric) common.Metric {
	om := m.(*SFlowMetric)

	return &SFlowMetric{
		Start:              im.Start,
		Last:               im.Last,
		IfIndex:            im.IfIndex + om.IfIndex,
		IfType:             im.IfType + om.IfType,
		IfSpeed:            im.IfSpeed + om.IfSpeed,
		IfDirection:        im.IfDirection + om.IfDirection,
		IfStatus:           im.IfStatus + om.IfStatus,
		IfInOctets:         im.IfInOctets + om.IfInOctets,
		IfInUcastPkts:      im.IfInUcastPkts + om.IfInUcastPkts,
		IfInMulticastPkts:  im.IfInMulticastPkts + om.IfInMulticastPkts,
		IfInBroadcastPkts:  im.IfInBroadcastPkts + om.IfInBroadcastPkts,
		IfInDiscards:       im.IfInDiscards + om.IfInDiscards,
		IfInErrors:         im.IfInErrors + om.IfInErrors,
		IfInUnknownProtos:  im.IfInUnknownProtos + om.IfInUnknownProtos,
		IfOutOctets:        im.IfOutOctets + om.IfOutOctets,
		IfOutUcastPkts:     im.IfOutUcastPkts + om.IfOutUcastPkts,
		IfOutMulticastPkts: im.IfOutMulticastPkts + om.IfOutMulticastPkts,
		IfOutBroadcastPkts: im.IfOutBroadcastPkts + om.IfOutBroadcastPkts,
		IfOutDiscards:      im.IfOutDiscards + om.IfOutDiscards,
		IfOutErrors:        im.IfOutErrors + om.IfOutErrors,
		IfPromiscuousMode:  im.IfPromiscuousMode + om.IfPromiscuousMode,
	}
}

// Sub subtract two metrics and return a new SFlowMetrics object
func (im *SFlowMetric) Sub(m common.Metric) common.Metric {
	om := m.(*SFlowMetric)

	return &SFlowMetric{
		Start:              im.Start,
		Last:               im.Last,
		IfIndex:            im.IfIndex - om.IfIndex,
		IfType:             im.IfType - om.IfType,
		IfSpeed:            im.IfSpeed - om.IfSpeed,
		IfDirection:        im.IfDirection - om.IfDirection,
		IfStatus:           im.IfStatus - om.IfStatus,
		IfInOctets:         im.IfInOctets - om.IfInOctets,
		IfInUcastPkts:      im.IfInUcastPkts - om.IfInUcastPkts,
		IfInMulticastPkts:  im.IfInMulticastPkts - om.IfInMulticastPkts,
		IfInBroadcastPkts:  im.IfInBroadcastPkts - om.IfInBroadcastPkts,
		IfInDiscards:       im.IfInDiscards - om.IfInDiscards,
		IfInErrors:         im.IfInErrors - om.IfInErrors,
		IfInUnknownProtos:  im.IfInUnknownProtos - om.IfInUnknownProtos,
		IfOutOctets:        im.IfOutOctets - om.IfOutOctets,
		IfOutUcastPkts:     im.IfOutUcastPkts - om.IfOutUcastPkts,
		IfOutMulticastPkts: im.IfOutMulticastPkts - om.IfOutMulticastPkts,
		IfOutBroadcastPkts: im.IfOutBroadcastPkts - om.IfOutBroadcastPkts,
		IfOutDiscards:      im.IfOutDiscards - om.IfOutDiscards,
		IfOutErrors:        im.IfOutErrors - om.IfOutErrors,
		IfPromiscuousMode:  im.IfPromiscuousMode - om.IfPromiscuousMode,
	}
}

// IsZero returns true if all the values are equal to zero
func (im *SFlowMetric) IsZero() bool {
	// sum as these numbers can't be <= 0
	return (im.IfIndex +
		im.IfType +
		im.IfSpeed +
		im.IfDirection +
		im.IfStatus +
		im.IfInOctets +
		im.IfInUcastPkts +
		im.IfInMulticastPkts +
		im.IfInBroadcastPkts +
		im.IfInDiscards +
		im.IfInErrors +
		im.IfInUnknownProtos +
		im.IfOutOctets +
		im.IfOutUcastPkts +
		im.IfOutMulticastPkts +
		im.IfOutBroadcastPkts +
		im.IfOutDiscards +
		im.IfOutErrors +
		im.IfPromiscuousMode) == 0
}

func (im *SFlowMetric) applyRatio(ratio float64) *SFlowMetric {
	return &SFlowMetric{
		Start:              im.Start,
		Last:               im.Last,
		IfIndex:            int64(float64(im.IfIndex) * ratio),
		IfType:             int64(float64(im.IfType) * ratio),
		IfSpeed:            int64(float64(im.IfSpeed) * ratio),
		IfDirection:        int64(float64(im.IfDirection) * ratio),
		IfStatus:           int64(float64(im.IfStatus) * ratio),
		IfInOctets:         int64(float64(im.IfInOctets) * ratio),
		IfInUcastPkts:      int64(float64(im.IfInUcastPkts) * ratio),
		IfInMulticastPkts:  int64(float64(im.IfInMulticastPkts) * ratio),
		IfInBroadcastPkts:  int64(float64(im.IfInBroadcastPkts) * ratio),
		IfInDiscards:       int64(float64(im.IfInDiscards) * ratio),
		IfInErrors:         int64(float64(im.IfInErrors) * ratio),
		IfInUnknownProtos:  int64(float64(im.IfInUnknownProtos) * ratio),
		IfOutOctets:        int64(float64(im.IfOutOctets) * ratio),
		IfOutUcastPkts:     int64(float64(im.IfOutUcastPkts) * ratio),
		IfOutMulticastPkts: int64(float64(im.IfOutMulticastPkts) * ratio),
		IfOutBroadcastPkts: int64(float64(im.IfOutBroadcastPkts) * ratio),
		IfOutDiscards:      int64(float64(im.IfOutDiscards) * ratio),
		IfOutErrors:        int64(float64(im.IfOutErrors) * ratio),
		IfPromiscuousMode:  int64(float64(im.IfPromiscuousMode) * ratio),
	}
}

// Split splits a metric into two parts
func (im *SFlowMetric) Split(cut int64) (common.Metric, common.Metric) {
	if cut < im.Start {
		return nil, im
	} else if cut > im.Last {
		return im, nil
	} else if im.Start == im.Last {
		return im, nil
	} else if cut == im.Start {
		return nil, im
	} else if cut == im.Last {
		return im, nil
	}

	duration := float64(im.Last - im.Start)

	ratio1 := float64(cut-im.Start) / duration
	ratio2 := float64(im.Last-cut) / duration

	m1 := im.applyRatio(ratio1)
	m1.Last = cut

	m2 := im.applyRatio(ratio2)
	m2.Start = cut

	return m1, m2
}

// GetFieldKeys implements Getter and SFlowMetrics interfaces
func (im *SFlowMetric) GetFieldKeys() []string {
	return sflowmetricsFields
}

var sflowmetricsFields []string

func init() {
	sflowmetricsFields = common.StructFieldKeys(SFlowMetric{})
}
