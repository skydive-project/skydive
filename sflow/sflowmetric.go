/*
 * Copyright (C) 2019 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package sflow

import (
	"encoding/json"

	"github.com/google/gopacket/layers"
	"github.com/skydive-project/skydive/common"
)

//SFlow all sflow information
type SFlow struct {
	Counters         []layers.SFlowCounterSample `json:"Counters,omitempty"`
	Metric           *SFMetric                   `json:"Metric,omitempty"`
	LastUpdateMetric *SFMetric                   `json:"LastUpdateMetric,omitempty"`
}

//SFMetadataDecoder implements a json message raw decoder
func SFMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var sf SFlow
	if err := json.Unmarshal(raw, &sf); err != nil {
		return nil, err
	}

	return &sf, nil
}

// GetField implements Getter interface
func (sf *SFlow) GetField(key string) (interface{}, error) {
	switch key {
	case "Metric":
		return sf.Metric, nil
	case "LastUpdateMetric":
		return sf.LastUpdateMetric, nil
	case "Counters":
		return sf.Counters, nil
	}

	return nil, common.ErrFieldNotFound
}

// GetFieldString implements Getter interface
func (sf *SFlow) GetFieldString(key string) (string, error) {
	return "", nil
}

// GetFieldInt64 implements Getter interface
func (sf *SFlow) GetFieldInt64(key string) (int64, error) {
	return 0, nil
}

// GetFieldKeys implements Getter interface
func (sf *SFlow) GetFieldKeys() []string {
	return sflowFields
}

var sflowFields []string

func init() {
	sflowFields = common.StructFieldKeys(SFlow{})
}

// SFMetric the SFlow Counter Samples
// easyjson:json
type SFMetric struct {
	Start              int64 `json:"Start,omitempty"`
	Last               int64 `json:"Last,omitempty"`
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
	OvsdpNHit          int64 `json:"OvsdpNHit,omitempty"`
	OvsdpNMissed       int64 `json:"OvsdpNMissed,omitempty"`
	OvsdpNLost         int64 `json:"OvsdpNLost,omitempty"`
	OvsdpNMaskHit      int64 `json:"OvsdpNMaskHit,omitempty"`
	OvsdpNFlows        int64 `json:"OvsdpNFlows,omitempty"`
	OvsdpNMasks        int64 `json:"OvsdpNMasks,omitempty"`
	OvsAppFdOpen       int64 `json:"OvsAppFdOpen,omitempty"`
	OvsAppFdMax        int64 `json:"OvsAppFdMax,omitempty"`
	OvsAppConnOpen     int64 `json:"OvsAppConnOpen,omitempty"`
	OvsAppConnMax      int64 `json:"OvsAppConnMax,omitempty"`
	OvsAppMemUsed      int64 `json:"OvsAppMemUsed,omitempty"`
	OvsAppMemMax       int64 `json:"OvsAppMemMax,omitempty"`
	VlanOctets         int64 `json:"VlanOctets,omitempty"`
	VlanUcastPkts      int64 `json:"VlanUcastPkts,omitempty"`
	VlanMulticastPkts  int64 `json:"VlanMulticastPkts,omitempty"`
	VlanBroadcastPkts  int64 `json:"VlanBroadcastPkts,omitempty"`
	VlanDiscards       int64 `json:"VlanDiscards,omitempty"`
}

// GetStart returns start time
func (sm *SFMetric) GetStart() int64 {
	return sm.Start
}

// SetStart set start time
func (sm *SFMetric) SetStart(start int64) {
	sm.Start = start
}

// GetLast returns last time
func (sm *SFMetric) GetLast() int64 {
	return sm.Last
}

// SetLast set last tome
func (sm *SFMetric) SetLast(last int64) {
	sm.Last = last
}

// GetFieldInt64 implements Getter and Metrics interfaces
func (sm *SFMetric) GetFieldInt64(field string) (int64, error) {
	switch field {
	case "Start":
		return sm.Start, nil
	case "Last":
		return sm.Last, nil
	case "IfInOctets":
		return sm.IfInOctets, nil
	case "IfInUcastPkts":
		return sm.IfInUcastPkts, nil
	case "IfInMulticastPkts":
		return sm.IfInMulticastPkts, nil
	case "IfInBroadcastPkts":
		return sm.IfInBroadcastPkts, nil
	case "IfInDiscards":
		return sm.IfInDiscards, nil
	case "IfInErrors":
		return sm.IfInErrors, nil
	case "IfInUnknownProtos":
		return sm.IfInUnknownProtos, nil
	case "IfOutOctets":
		return sm.IfOutOctets, nil
	case "IfOutUcastPkts":
		return sm.IfOutUcastPkts, nil
	case "IfOutMulticastPkts":
		return sm.IfOutMulticastPkts, nil
	case "IfOutBroadcastPkts":
		return sm.IfOutBroadcastPkts, nil
	case "IfOutDiscards":
		return sm.IfOutDiscards, nil
	case "IfOutErrors":
		return sm.IfOutErrors, nil
	case "OvsdpNHit":
		return sm.OvsdpNHit, nil
	case "OvsdpNMissed":
		return sm.OvsdpNMissed, nil
	case "OvsdpNLost":
		return sm.OvsdpNLost, nil
	case "OvsdpNMaskHit":
		return sm.OvsdpNMaskHit, nil
	case "OvsdpNFlows":
		return sm.OvsdpNFlows, nil
	case "OvsdpNMasks":
		return sm.OvsdpNMasks, nil
	case "OvsAppFdOpen":
		return sm.OvsAppFdOpen, nil
	case "OvsAppFdMax":
		return sm.OvsAppFdMax, nil
	case "OvsAppConnOpen":
		return sm.OvsAppConnOpen, nil
	case "OvsAppConnMax":
		return sm.OvsAppConnMax, nil
	case "OvsAppMemUsed":
		return sm.OvsAppMemUsed, nil
	case "OvsAppMemMax":
		return sm.OvsAppMemMax, nil
	case "VlanOctets":
		return sm.VlanOctets, nil
	case "VlanUcastPkts":
		return sm.VlanUcastPkts, nil
	case "VlanMulticastPkts":
		return sm.VlanMulticastPkts, nil
	case "VlanBroadcastPkts":
		return sm.VlanBroadcastPkts, nil
	case "VlanDiscards":
		return sm.VlanDiscards, nil
	}

	return 0, common.ErrFieldNotFound
}

// GetField implements Getter interface
func (sm *SFMetric) GetField(key string) (interface{}, error) {
	return sm.GetFieldInt64(key)
}

// GetFieldString implements Getter interface
func (sm *SFMetric) GetFieldString(key string) (string, error) {
	return "", common.ErrFieldNotFound
}

// Add sum two metrics and return a new Metrics object
func (sm *SFMetric) Add(m common.Metric) common.Metric {
	om, ok := m.(*SFMetric)
	if !ok {
		return sm
	}

	return &SFMetric{
		Start:              sm.Start,
		Last:               sm.Last,
		IfInOctets:         sm.IfInOctets + om.IfInOctets,
		IfInUcastPkts:      sm.IfInUcastPkts + om.IfInUcastPkts,
		IfInMulticastPkts:  sm.IfInMulticastPkts + om.IfInMulticastPkts,
		IfInBroadcastPkts:  sm.IfInBroadcastPkts + om.IfInBroadcastPkts,
		IfInDiscards:       sm.IfInDiscards + om.IfInDiscards,
		IfInErrors:         sm.IfInErrors + om.IfInErrors,
		IfInUnknownProtos:  sm.IfInUnknownProtos + om.IfInUnknownProtos,
		IfOutOctets:        sm.IfOutOctets + om.IfOutOctets,
		IfOutUcastPkts:     sm.IfOutUcastPkts + om.IfOutUcastPkts,
		IfOutMulticastPkts: sm.IfOutMulticastPkts + om.IfOutMulticastPkts,
		IfOutBroadcastPkts: sm.IfOutBroadcastPkts + om.IfOutBroadcastPkts,
		IfOutDiscards:      sm.IfOutDiscards + om.IfOutDiscards,
		IfOutErrors:        sm.IfOutErrors + om.IfOutErrors,
		OvsdpNHit:          sm.OvsdpNHit + om.OvsdpNHit,
		OvsdpNMissed:       sm.OvsdpNMissed + om.OvsdpNMissed,
		OvsdpNLost:         sm.OvsdpNLost + om.OvsdpNLost,
		OvsdpNMaskHit:      sm.OvsdpNMaskHit + om.OvsdpNMaskHit,
		OvsdpNFlows:        sm.OvsdpNFlows + om.OvsdpNFlows,
		OvsdpNMasks:        sm.OvsdpNMasks + om.OvsdpNMasks,
		OvsAppFdOpen:       sm.OvsAppFdOpen + om.OvsAppFdOpen,
		OvsAppFdMax:        sm.OvsAppFdMax + om.OvsAppFdMax,
		OvsAppConnOpen:     sm.OvsAppConnOpen + om.OvsAppConnOpen,
		OvsAppConnMax:      sm.OvsAppConnMax + om.OvsAppConnMax,
		OvsAppMemUsed:      sm.OvsAppMemUsed + om.OvsAppMemUsed,
		OvsAppMemMax:       sm.OvsAppMemMax + om.OvsAppMemMax,
		VlanOctets:         sm.VlanOctets + om.VlanOctets,
		VlanUcastPkts:      sm.VlanUcastPkts + om.VlanUcastPkts,
		VlanMulticastPkts:  sm.VlanMulticastPkts + om.VlanMulticastPkts,
		VlanBroadcastPkts:  sm.VlanBroadcastPkts + om.VlanBroadcastPkts,
		VlanDiscards:       sm.VlanDiscards + om.VlanDiscards,
	}
}

// Sub subtract two metrics and return a new Metrics object
func (sm *SFMetric) Sub(m common.Metric) common.Metric {
	om, ok := m.(*SFMetric)
	if !ok {
		return sm
	}

	return &SFMetric{
		Start:              sm.Start,
		Last:               sm.Last,
		IfInOctets:         sm.IfInOctets - om.IfInOctets,
		IfInUcastPkts:      sm.IfInUcastPkts - om.IfInUcastPkts,
		IfInMulticastPkts:  sm.IfInMulticastPkts - om.IfInMulticastPkts,
		IfInBroadcastPkts:  sm.IfInBroadcastPkts - om.IfInBroadcastPkts,
		IfInDiscards:       sm.IfInDiscards - om.IfInDiscards,
		IfInErrors:         sm.IfInErrors - om.IfInErrors,
		IfInUnknownProtos:  sm.IfInUnknownProtos - om.IfInUnknownProtos,
		IfOutOctets:        sm.IfOutOctets - om.IfOutOctets,
		IfOutUcastPkts:     sm.IfOutUcastPkts - om.IfOutUcastPkts,
		IfOutMulticastPkts: sm.IfOutMulticastPkts - om.IfOutMulticastPkts,
		IfOutBroadcastPkts: sm.IfOutBroadcastPkts - om.IfOutBroadcastPkts,
		IfOutDiscards:      sm.IfOutDiscards - om.IfOutDiscards,
		IfOutErrors:        sm.IfOutErrors - om.IfOutErrors,
		OvsdpNHit:          sm.OvsdpNHit - om.OvsdpNHit,
		OvsdpNMissed:       sm.OvsdpNMissed - om.OvsdpNMissed,
		OvsdpNLost:         sm.OvsdpNLost - om.OvsdpNLost,
		OvsdpNMaskHit:      sm.OvsdpNMaskHit - om.OvsdpNMaskHit,
		OvsdpNFlows:        sm.OvsdpNFlows - om.OvsdpNFlows,
		OvsdpNMasks:        sm.OvsdpNMasks - om.OvsdpNMasks,
		OvsAppFdOpen:       sm.OvsAppFdOpen - om.OvsAppFdOpen,
		OvsAppFdMax:        sm.OvsAppFdMax - om.OvsAppFdMax,
		OvsAppConnOpen:     sm.OvsAppConnOpen - om.OvsAppConnOpen,
		OvsAppConnMax:      sm.OvsAppConnMax - om.OvsAppConnMax,
		OvsAppMemUsed:      sm.OvsAppMemUsed - om.OvsAppMemUsed,
		OvsAppMemMax:       sm.OvsAppMemMax - om.OvsAppMemMax,
		VlanOctets:         sm.VlanOctets - om.VlanOctets,
		VlanUcastPkts:      sm.VlanUcastPkts - om.VlanUcastPkts,
		VlanMulticastPkts:  sm.VlanMulticastPkts - om.VlanMulticastPkts,
		VlanBroadcastPkts:  sm.VlanBroadcastPkts - om.VlanBroadcastPkts,
		VlanDiscards:       sm.VlanDiscards - om.VlanDiscards,
	}
}

// IsZero returns true if all the values are equal to zero
func (sm *SFMetric) IsZero() bool {
	// sum as these numbers can't be <= 0
	return (sm.IfInOctets +
		sm.IfInUcastPkts +
		sm.IfInMulticastPkts +
		sm.IfInBroadcastPkts +
		sm.IfInDiscards +
		sm.IfInErrors +
		sm.IfInUnknownProtos +
		sm.IfOutOctets +
		sm.IfOutUcastPkts +
		sm.IfOutMulticastPkts +
		sm.IfOutBroadcastPkts +
		sm.IfOutDiscards +
		sm.IfOutErrors +
		sm.OvsdpNHit +
		sm.OvsdpNMissed +
		sm.OvsdpNLost +
		sm.OvsdpNMaskHit +
		sm.OvsdpNFlows +
		sm.OvsdpNMasks +
		sm.OvsAppFdOpen +
		sm.OvsAppFdMax +
		sm.OvsAppConnOpen +
		sm.OvsAppConnMax +
		sm.OvsAppMemUsed +
		sm.OvsAppMemMax +
		sm.VlanOctets +
		sm.VlanUcastPkts +
		sm.VlanMulticastPkts +
		sm.VlanBroadcastPkts +
		sm.VlanDiscards) == 0
}

func (sm *SFMetric) applyRatio(ratio float64) *SFMetric {
	return &SFMetric{
		Start:              sm.Start,
		Last:               sm.Last,
		IfInOctets:         int64(float64(sm.IfInOctets) * ratio),
		IfInUcastPkts:      int64(float64(sm.IfInUcastPkts) * ratio),
		IfInMulticastPkts:  int64(float64(sm.IfInMulticastPkts) * ratio),
		IfInBroadcastPkts:  int64(float64(sm.IfInBroadcastPkts) * ratio),
		IfInDiscards:       int64(float64(sm.IfInDiscards) * ratio),
		IfInErrors:         int64(float64(sm.IfInErrors) * ratio),
		IfInUnknownProtos:  int64(float64(sm.IfInUnknownProtos) * ratio),
		IfOutOctets:        int64(float64(sm.IfOutOctets) * ratio),
		IfOutUcastPkts:     int64(float64(sm.IfOutUcastPkts) * ratio),
		IfOutMulticastPkts: int64(float64(sm.IfOutMulticastPkts) * ratio),
		IfOutBroadcastPkts: int64(float64(sm.IfOutBroadcastPkts) * ratio),
		IfOutDiscards:      int64(float64(sm.IfOutDiscards) * ratio),
		IfOutErrors:        int64(float64(sm.IfOutErrors) * ratio),
		OvsdpNHit:          int64(float64(sm.OvsdpNHit) * ratio),
		OvsdpNMissed:       int64(float64(sm.OvsdpNMissed) * ratio),
		OvsdpNLost:         int64(float64(sm.OvsdpNLost) * ratio),
		OvsdpNMaskHit:      int64(float64(sm.OvsdpNMaskHit) * ratio),
		OvsdpNFlows:        int64(float64(sm.OvsdpNFlows) * ratio),
		OvsdpNMasks:        int64(float64(sm.OvsdpNMasks) * ratio),
		OvsAppFdOpen:       int64(float64(sm.OvsAppFdOpen) * ratio),
		OvsAppFdMax:        int64(float64(sm.OvsAppFdMax) * ratio),
		OvsAppConnOpen:     int64(float64(sm.OvsAppConnOpen) * ratio),
		OvsAppConnMax:      int64(float64(sm.OvsAppConnMax) * ratio),
		OvsAppMemUsed:      int64(float64(sm.OvsAppMemUsed) * ratio),
		OvsAppMemMax:       int64(float64(sm.OvsAppMemMax) * ratio),
		VlanOctets:         int64(float64(sm.VlanOctets) * ratio),
		VlanUcastPkts:      int64(float64(sm.VlanUcastPkts) * ratio),
		VlanMulticastPkts:  int64(float64(sm.VlanMulticastPkts) * ratio),
		VlanBroadcastPkts:  int64(float64(sm.VlanBroadcastPkts) * ratio),
		VlanDiscards:       int64(float64(sm.VlanDiscards) * ratio),
	}
}

// Split splits a metric into two parts
func (sm *SFMetric) Split(cut int64) (common.Metric, common.Metric) {
	if cut < sm.Start {
		return nil, sm
	} else if cut > sm.Last {
		return sm, nil
	} else if sm.Start == sm.Last {
		return sm, nil
	} else if cut == sm.Start {
		return nil, sm
	} else if cut == sm.Last {
		return sm, nil
	}

	duration := float64(sm.Last - sm.Start)

	ratio1 := float64(cut-sm.Start) / duration
	ratio2 := float64(sm.Last-cut) / duration

	sm1 := sm.applyRatio(ratio1)
	sm1.Last = cut

	sm2 := sm.applyRatio(ratio2)
	sm2.Start = cut

	return sm1, sm2
}

// GetFieldKeys implements Getter and Metrics interfaces
func (sm *SFMetric) GetFieldKeys() []string {
	return sflowmetricsFields
}

var sflowmetricsFields []string

func init() {
	sflowmetricsFields = common.StructFieldKeys(SFMetric{})
}
