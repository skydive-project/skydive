//go:generate go run ../scripts/gendecoder.go -strict=false

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

	"github.com/skydive-project/skydive/common"
)

// SFlow all sflow information
// easyjson:json
// gendecoder
type SFlow struct {
	IfMetrics        map[int64]*IfMetric `json:"IfMetrics,omitempty"`
	Metric           *SFMetric           `json:"Metric,omitempty"`
	LastUpdateMetric *SFMetric           `json:"LastUpdateMetric,omitempty"`
}

// SFMetadataDecoder implements a json message raw decoder
func SFMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var sf SFlow
	if err := json.Unmarshal(raw, &sf); err != nil {
		return nil, err
	}

	return &sf, nil
}

// SFMetric the SFlow Counter Samples
// easyjson:json
// gendecoder
type SFMetric struct {
	IfMetric
	OvsMetric
	VlanMetric
	EthMetric

	Start int64 `json:"Start,omitempty"`
	Last  int64 `json:"Last,omitempty"`
}

// EthMetric the SFlow ethernet counters
// easyjson:json
// gendecoder
type EthMetric struct {
	EthAlignmentErrors           int64 `json:"EthAlignmentErrors,omitempty"`
	EthFCSErrors                 int64 `json:"EthFCSErrors,omitempty"`
	EthSingleCollisionFrames     int64 `json:"EthSingleCollisionFrames,omitempty"`
	EthMultipleCollisionFrames   int64 `json:"EthMultipleCollisionFrames,omitempty"`
	EthSQETestErrors             int64 `json:"EthSQETestErrors,omitempty"`
	EthDeferredTransmissions     int64 `json:"EthDeferredTransmissions,omitempty"`
	EthLateCollisions            int64 `json:"EthLateCollisions,omitempty"`
	EthExcessiveCollisions       int64 `json:"EthExcessiveCollisions,omitempty"`
	EthInternalMacReceiveErrors  int64 `json:"EthInternalMacReceiveErrors,omitempty"`
	EthInternalMacTransmitErrors int64 `json:"EthInternalMacTransmitErrors,omitempty"`
	EthCarrierSenseErrors        int64 `json:"EthCarrierSenseErrors,omitempty"`
	EthFrameTooLongs             int64 `json:"EthFrameTooLongs,omitempty"`
	EthSymbolErrors              int64 `json:"EthSymbolErrors,omitempty"`
}

// VlanMetric the SFlow vlan counters
// easyjson:json
// gendecoder
type VlanMetric struct {
	VlanOctets        int64 `json:"VlanOctets,omitempty"`
	VlanUcastPkts     int64 `json:"VlanUcastPkts,omitempty"`
	VlanMulticastPkts int64 `json:"VlanMulticastPkts,omitempty"`
	VlanBroadcastPkts int64 `json:"VlanBroadcastPkts,omitempty"`
	VlanDiscards      int64 `json:"VlanDiscards,omitempty"`
}

// OvsMetric the SFlow ovs counters
// easyjson:json
// gendecoder
type OvsMetric struct {
	OvsDpNHit      int64 `json:"OvsDpNHit,omitempty"`
	OvsDpNMissed   int64 `json:"OvsDpNMissed,omitempty"`
	OvsDpNLost     int64 `json:"OvsDpNLost,omitempty"`
	OvsDpNMaskHit  int64 `json:"OvsDpNMaskHit,omitempty"`
	OvsDpNFlows    int64 `json:"OvsDpNFlows,omitempty"`
	OvsDpNMasks    int64 `json:"OvsDpNMasks,omitempty"`
	OvsAppFdOpen   int64 `json:"OvsAppFdOpen,omitempty"`
	OvsAppFdMax    int64 `json:"OvsAppFdMax,omitempty"`
	OvsAppConnOpen int64 `json:"OvsAppConnOpen,omitempty"`
	OvsAppConnMax  int64 `json:"OvsAppConnMax,omitempty"`
	OvsAppMemUsed  int64 `json:"OvsAppMemUsed,omitempty"`
	OvsAppMemMax   int64 `json:"OvsAppMemMax,omitempty"`
}

// IfMetric the SFlow Interface counters
// easyjson:json
// gendecoder
type IfMetric struct {
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

// Add sum two metrics and return a new Metrics object
func (sm *SFMetric) Add(m common.Metric) common.Metric {
	om, ok := m.(*SFMetric)
	if !ok {
		return sm
	}

	return &SFMetric{
		Start: sm.Start,
		Last:  sm.Last,
		IfMetric: IfMetric{
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
		},
		OvsMetric: OvsMetric{
			OvsDpNHit:      sm.OvsDpNHit + om.OvsDpNHit,
			OvsDpNMissed:   sm.OvsDpNMissed + om.OvsDpNMissed,
			OvsDpNLost:     sm.OvsDpNLost + om.OvsDpNLost,
			OvsDpNMaskHit:  sm.OvsDpNMaskHit + om.OvsDpNMaskHit,
			OvsDpNFlows:    sm.OvsDpNFlows + om.OvsDpNFlows,
			OvsDpNMasks:    sm.OvsDpNMasks + om.OvsDpNMasks,
			OvsAppFdOpen:   sm.OvsAppFdOpen + om.OvsAppFdOpen,
			OvsAppFdMax:    sm.OvsAppFdMax + om.OvsAppFdMax,
			OvsAppConnOpen: sm.OvsAppConnOpen + om.OvsAppConnOpen,
			OvsAppConnMax:  sm.OvsAppConnMax + om.OvsAppConnMax,
			OvsAppMemUsed:  sm.OvsAppMemUsed + om.OvsAppMemUsed,
			OvsAppMemMax:   sm.OvsAppMemMax + om.OvsAppMemMax,
		},
		VlanMetric: VlanMetric{
			VlanOctets:        sm.VlanOctets + om.VlanOctets,
			VlanUcastPkts:     sm.VlanUcastPkts + om.VlanUcastPkts,
			VlanMulticastPkts: sm.VlanMulticastPkts + om.VlanMulticastPkts,
			VlanBroadcastPkts: sm.VlanBroadcastPkts + om.VlanBroadcastPkts,
			VlanDiscards:      sm.VlanDiscards + om.VlanDiscards,
		},
		EthMetric: EthMetric{
			EthAlignmentErrors:           sm.EthAlignmentErrors + om.EthAlignmentErrors,
			EthFCSErrors:                 sm.EthFCSErrors + om.EthFCSErrors,
			EthSingleCollisionFrames:     sm.EthSingleCollisionFrames + om.EthSingleCollisionFrames,
			EthMultipleCollisionFrames:   sm.EthMultipleCollisionFrames + om.EthMultipleCollisionFrames,
			EthSQETestErrors:             sm.EthSQETestErrors + om.EthSQETestErrors,
			EthDeferredTransmissions:     sm.EthDeferredTransmissions + om.EthDeferredTransmissions,
			EthLateCollisions:            sm.EthLateCollisions + om.EthLateCollisions,
			EthExcessiveCollisions:       sm.EthExcessiveCollisions + om.EthExcessiveCollisions,
			EthInternalMacReceiveErrors:  sm.EthInternalMacReceiveErrors + om.EthInternalMacReceiveErrors,
			EthInternalMacTransmitErrors: sm.EthInternalMacTransmitErrors + om.EthInternalMacTransmitErrors,
			EthCarrierSenseErrors:        sm.EthCarrierSenseErrors + om.EthCarrierSenseErrors,
			EthFrameTooLongs:             sm.EthFrameTooLongs + om.EthFrameTooLongs,
			EthSymbolErrors:              sm.EthSymbolErrors + om.EthSymbolErrors,
		},
	}
}

// Sub subtract two metrics and return a new Metrics object
func (sm *SFMetric) Sub(m common.Metric) common.Metric {
	om, ok := m.(*SFMetric)
	if !ok {
		return sm
	}

	return &SFMetric{
		Start: sm.Start,
		Last:  sm.Last,
		IfMetric: IfMetric{
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
		},
		OvsMetric: OvsMetric{
			OvsDpNHit:      sm.OvsDpNHit - om.OvsDpNHit,
			OvsDpNMissed:   sm.OvsDpNMissed - om.OvsDpNMissed,
			OvsDpNLost:     sm.OvsDpNLost - om.OvsDpNLost,
			OvsDpNMaskHit:  sm.OvsDpNMaskHit - om.OvsDpNMaskHit,
			OvsDpNFlows:    sm.OvsDpNFlows - om.OvsDpNFlows,
			OvsDpNMasks:    sm.OvsDpNMasks - om.OvsDpNMasks,
			OvsAppFdOpen:   sm.OvsAppFdOpen - om.OvsAppFdOpen,
			OvsAppFdMax:    sm.OvsAppFdMax - om.OvsAppFdMax,
			OvsAppConnOpen: sm.OvsAppConnOpen - om.OvsAppConnOpen,
			OvsAppConnMax:  sm.OvsAppConnMax - om.OvsAppConnMax,
			OvsAppMemUsed:  sm.OvsAppMemUsed - om.OvsAppMemUsed,
			OvsAppMemMax:   sm.OvsAppMemMax - om.OvsAppMemMax,
		},
		VlanMetric: VlanMetric{
			VlanOctets:        sm.VlanOctets - om.VlanOctets,
			VlanUcastPkts:     sm.VlanUcastPkts - om.VlanUcastPkts,
			VlanMulticastPkts: sm.VlanMulticastPkts - om.VlanMulticastPkts,
			VlanBroadcastPkts: sm.VlanBroadcastPkts - om.VlanBroadcastPkts,
			VlanDiscards:      sm.VlanDiscards - om.VlanDiscards,
		},
		EthMetric: EthMetric{
			EthAlignmentErrors:           sm.EthAlignmentErrors - om.EthAlignmentErrors,
			EthFCSErrors:                 sm.EthFCSErrors - om.EthFCSErrors,
			EthSingleCollisionFrames:     sm.EthSingleCollisionFrames - om.EthSingleCollisionFrames,
			EthMultipleCollisionFrames:   sm.EthMultipleCollisionFrames - om.EthMultipleCollisionFrames,
			EthSQETestErrors:             sm.EthSQETestErrors - om.EthSQETestErrors,
			EthDeferredTransmissions:     sm.EthDeferredTransmissions - om.EthDeferredTransmissions,
			EthLateCollisions:            sm.EthLateCollisions - om.EthLateCollisions,
			EthExcessiveCollisions:       sm.EthExcessiveCollisions - om.EthExcessiveCollisions,
			EthInternalMacReceiveErrors:  sm.EthInternalMacReceiveErrors - om.EthInternalMacReceiveErrors,
			EthInternalMacTransmitErrors: sm.EthInternalMacTransmitErrors - om.EthInternalMacTransmitErrors,
			EthCarrierSenseErrors:        sm.EthCarrierSenseErrors - om.EthCarrierSenseErrors,
			EthFrameTooLongs:             sm.EthFrameTooLongs - om.EthFrameTooLongs,
			EthSymbolErrors:              sm.EthSymbolErrors - om.EthSymbolErrors,
		},
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
		sm.OvsDpNHit +
		sm.OvsDpNMissed +
		sm.OvsDpNLost +
		sm.OvsDpNMaskHit +
		sm.OvsDpNFlows +
		sm.OvsDpNMasks +
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
		sm.VlanDiscards +
		sm.EthAlignmentErrors +
		sm.EthFCSErrors +
		sm.EthSingleCollisionFrames +
		sm.EthMultipleCollisionFrames +
		sm.EthSQETestErrors +
		sm.EthDeferredTransmissions +
		sm.EthLateCollisions +
		sm.EthExcessiveCollisions +
		sm.EthInternalMacReceiveErrors +
		sm.EthInternalMacTransmitErrors +
		sm.EthCarrierSenseErrors +
		sm.EthFrameTooLongs +
		sm.EthSymbolErrors) == 0
}

func (sm *SFMetric) applyRatio(ratio float64) *SFMetric {
	return &SFMetric{
		Start: sm.Start,
		Last:  sm.Last,
		IfMetric: IfMetric{
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
		},
		OvsMetric: OvsMetric{
			OvsDpNHit:      int64(float64(sm.OvsDpNHit) * ratio),
			OvsDpNMissed:   int64(float64(sm.OvsDpNMissed) * ratio),
			OvsDpNLost:     int64(float64(sm.OvsDpNLost) * ratio),
			OvsDpNMaskHit:  int64(float64(sm.OvsDpNMaskHit) * ratio),
			OvsDpNFlows:    int64(float64(sm.OvsDpNFlows) * ratio),
			OvsDpNMasks:    int64(float64(sm.OvsDpNMasks) * ratio),
			OvsAppFdOpen:   int64(float64(sm.OvsAppFdOpen) * ratio),
			OvsAppFdMax:    int64(float64(sm.OvsAppFdMax) * ratio),
			OvsAppConnOpen: int64(float64(sm.OvsAppConnOpen) * ratio),
			OvsAppConnMax:  int64(float64(sm.OvsAppConnMax) * ratio),
			OvsAppMemUsed:  int64(float64(sm.OvsAppMemUsed) * ratio),
			OvsAppMemMax:   int64(float64(sm.OvsAppMemMax) * ratio),
		},
		VlanMetric: VlanMetric{
			VlanOctets:        int64(float64(sm.VlanOctets) * ratio),
			VlanUcastPkts:     int64(float64(sm.VlanUcastPkts) * ratio),
			VlanMulticastPkts: int64(float64(sm.VlanMulticastPkts) * ratio),
			VlanBroadcastPkts: int64(float64(sm.VlanBroadcastPkts) * ratio),
			VlanDiscards:      int64(float64(sm.VlanDiscards) * ratio),
		},
		EthMetric: EthMetric{
			EthAlignmentErrors:           int64(float64(sm.EthAlignmentErrors) * ratio),
			EthFCSErrors:                 int64(float64(sm.EthFCSErrors) * ratio),
			EthSingleCollisionFrames:     int64(float64(sm.EthSingleCollisionFrames) * ratio),
			EthMultipleCollisionFrames:   int64(float64(sm.EthMultipleCollisionFrames) * ratio),
			EthSQETestErrors:             int64(float64(sm.EthSQETestErrors) * ratio),
			EthDeferredTransmissions:     int64(float64(sm.EthDeferredTransmissions) * ratio),
			EthLateCollisions:            int64(float64(sm.EthLateCollisions) * ratio),
			EthExcessiveCollisions:       int64(float64(sm.EthExcessiveCollisions) * ratio),
			EthInternalMacReceiveErrors:  int64(float64(sm.EthInternalMacReceiveErrors) * ratio),
			EthInternalMacTransmitErrors: int64(float64(sm.EthInternalMacTransmitErrors) * ratio),
			EthCarrierSenseErrors:        int64(float64(sm.EthCarrierSenseErrors) * ratio),
			EthFrameTooLongs:             int64(float64(sm.EthFrameTooLongs) * ratio),
			EthSymbolErrors:              int64(float64(sm.EthSymbolErrors) * ratio),
		},
	}
}

// Split splits a metric into two parts
func (sm *SFMetric) Split(cut int64) (common.Metric, common.Metric) {
	if cut <= sm.Start {
		return nil, sm
	} else if cut >= sm.Last || sm.Start == sm.Last {
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
