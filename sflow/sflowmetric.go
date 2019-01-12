/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package sflow

import (
	"encoding/json"
	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/common"
)

type SFlow struct {
	Counters []layers.SFlowCounterSample  `json:"Counters,omitempty"`
	Metric *SFMetric  `json:"Metric,omitempty"`
	LastUpdateMetric *SFMetric `json:"LastUpdateMetric,omitempty"`
}

//SFlowMetadataDecoder implements a json message raw decoder
func SFlowMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var sf SFlow
	if err := json.Unmarshal(raw, &sf); err != nil {
		return nil, err
	}

	return &sf, nil
}

// GetField implements Getter interface
func (sf *SFlow) GetField(key string) (interface{}, error) {
	switch key {
	case "Metric" :
		return sf.Metric, nil
	case "LastUpdateMetric" :
		return sf.LastUpdateMetric, nil
	case "Counters" :
		return sf.Counters, nil
	}

	return nil, nil
}

// GetFieldString implements Getter interface
func (sf *SFlow) GetFieldString(key string) (string, error) {
	return "", nil
}

// GetFieldInt64 implements Getter interface
func (sf *SFlow) GetFieldInt64(key string) (int64, error) {
	return 0, nil
}

// GetFieldKeys implements Getter and Metrics interfaces
func (sf *SFlow) GetFieldKeys() []string {
	return sflowFields
}

var sflowFields []string

func init() {
	sflowFields = common.StructFieldKeys(SFlow{})
}

// SFMetric the SFlow counter samples
// easyjson:json
type SFMetric struct {
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

// GetStart returns start time
func (im *SFMetric) GetStart() int64 {
	return im.Start
}

// SetStart set start time
func (im *SFMetric) SetStart(start int64) {
	im.Start = start
}

// GetLast returns last time
func (im *SFMetric) GetLast() int64 {
	return im.Last
}

// SetLast set last tome
func (im *SFMetric) SetLast(last int64) {
	im.Last = last
}

// GetFieldInt64 implements Getter and SFlowMetrics interfaces
func (im *SFMetric) GetFieldInt64(field string) (int64, error) {
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
func (im *SFMetric) GetField(key string) (interface{}, error) {
	return im.GetFieldInt64(key)
}

// GetFieldString implements Getter interface
func (im *SFMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

// Add sum two metrics and return a new SFlowMetrics object
func (im *SFMetric) Add(m common.Metric) common.Metric {
	om, ok := m.(*SFMetric)
	if !ok {
		return im
	}

	return &SFMetric{
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
func (im *SFMetric) Sub(m common.Metric) common.Metric {
	om, ok := m.(*SFMetric)
	if !ok {
		return im
	}

	return &SFMetric{
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
func (im *SFMetric) IsZero() bool {
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

func (im *SFMetric) applyRatio(ratio float64) *SFMetric {
	return &SFMetric{
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
func (im *SFMetric) Split(cut int64) (common.Metric, common.Metric) {
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
func (im *SFMetric) GetFieldKeys() []string {
	return sflowmetricsFields
}

var sflowmetricsFields []string

func init() {
	sflowmetricsFields = common.StructFieldKeys(SFMetric{})
}