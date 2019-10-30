/*
 * Copyright (C) 2019 IBM, Inc.
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

package mod

import (
	"time"

	cache "github.com/pmylund/go-cache"

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/contrib/exporters/core"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

// NewTransform creates a new flow transformer based on a name string
func NewTransform(cfg *viper.Viper) (interface{}, error) {
	excludeStartedFlows := cfg.GetBool(core.CfgRoot + "transform.secadvisor.exclude_started_flows")

	return &vpclogsFlowTransformer{
		flowUpdateCountCache: cache.New(10*time.Minute, 10*time.Minute),
		excludeStartedFlows:  excludeStartedFlows,
	}, nil
}

const (
	VpcActionReject string = "R"
	VpcActionAccept string = "A"
)

// VpclogsFlow represents a vpc log flow
// this structure will be updated when the fields are finalized
type VpclogsFlow struct {
	FinishType          flow.FlowFinishType `json:"-"`
	Flow                *flow.Flow          `json:"-"`
	LastUpdateMetric    *flow.FlowMetric    `json:"-"`
	Metric              *flow.FlowMetric    `json:"-"`
	Status              string              `json:"-"`
	Start               int64               `json:"-"`
	Protocol            string              `json:"-"`
	TID                 string              `json:"-"`
	StartTime           int64               `json:"start_time,omitempty"`
	EndTime             int64               `json:"end_time,omitempty"`
	ConnectionStartTime int64               `json:"connection_start_time,omitempty"`
	// TBD - compute direction
	Direction                      string `json:"direction,omitempty"`
	Action                         string `json:"action,omitempty"`
	InitiatorIp                    string `json:"initiator_ip,omitempty"`
	TargetIp                       string `json:"target_ip,omitempty"`
	InitiatorPort                  int64  `json:"initiator_port,omitempty"`
	TargetPort                     int64  `json:"target_port,omitempty"`
	TransportProtocol              uint8  `json:"transport_protocol,omitempty"`
	EtherType                      string `json:"ether_type,omitempty"`
	WasInitiated                   bool   `json:"was_initiated"`
	WasTerminated                  bool   `json:"was_terminated"`
	CumulativeBytesFromInitiator   int64  `json:"cumulative_bytes_from_initiator,omitempty"`
	CumulativePacketsFromInitiator int64  `json:"cumulative_packets_from_initiator,omitempty"`
	CumulativeBytesFromTarget      int64  `json:"cumulative_bytes_from_target,omitempty"`
	CumulativePacketsFromTarget    int64  `json:"cumulative_packets_from_target,omitempty"`
	BytesFromInitiator             int64  `json:"bytes_from_initiator,omitempty"`
	PacketsFromInitiator           int64  `json:"packets_from_initiator,omitempty"`
	BytesFromTarget                int64  `json:"bytes_from_target,omitempty"`
	PacketsFromTarget              int64  `json:"packets_from_target,omitempty"`
}

// VpclogsFlowTransformer is a custom transformer for flows
type vpclogsFlowTransformer struct {
	flowUpdateCountCache *cache.Cache
	excludeStartedFlows  bool
}

func (ft *vpclogsFlowTransformer) setUpdateCount(f *flow.Flow) int64 {
	var count int64
	if countRaw, ok := ft.flowUpdateCountCache.Get(f.UUID); ok {
		count = countRaw.(int64)
	}

	if f.FinishType != flow.FlowFinishType_TIMEOUT {
		if f.FinishType == flow.FlowFinishType_NOT_FINISHED {
			ft.flowUpdateCountCache.Set(f.UUID, count+1, cache.DefaultExpiration)
		} else {
			ft.flowUpdateCountCache.Set(f.UUID, count+1, time.Minute)
		}
	} else {
		ft.flowUpdateCountCache.Delete(f.UUID)
	}

	return count
}

func (ft *vpclogsFlowTransformer) getStatus(f *flow.Flow, updateCount int64) string {
	if f.FinishType != flow.FlowFinishType_NOT_FINISHED {
		return "ENDED"
	}

	if updateCount == 0 {
		return "STARTED"
	}

	return "UPDATED"
}

func (v *VpclogsFlow) SetAction(action string) {
	v.Action = action
}

// DeriveExternalizedFields builds the fields that are visible in the vpc flow logs.
// All these values are derived from values stored in the VpclogsFlow structure (not from the original flows) since these may have been updated by additional processing.
func (v *VpclogsFlow) DeriveExternalizedFields() {
	v.StartTime = v.LastUpdateMetric.Start
	v.EndTime = v.LastUpdateMetric.Last
	// In the meantime, EtherType is hard-coded
	v.EtherType = "IPv4"
	v.ConnectionStartTime = v.Start
	v.CumulativeBytesFromInitiator = v.Metric.ABBytes
	v.CumulativePacketsFromInitiator = v.Metric.ABPackets
	v.CumulativeBytesFromTarget = v.Metric.BABytes
	v.CumulativePacketsFromTarget = v.Metric.BAPackets
	v.BytesFromInitiator = v.LastUpdateMetric.ABBytes
	v.PacketsFromInitiator = v.LastUpdateMetric.ABPackets
	v.BytesFromTarget = v.LastUpdateMetric.BABytes
	v.PacketsFromTarget = v.LastUpdateMetric.BAPackets

	// We don't usually see the report a finished flow
	// TBM These 2 fields need to be handled differently
	v.WasTerminated = v.FinishType != flow.FlowFinishType_NOT_FINISHED
	v.WasInitiated = v.Status == "STARTED"

	// IP protocol type (for example, TCP = 6; UDP = 17, ICMP = 1)
	if v.Protocol == "TCP" {
		v.TransportProtocol = 6
	} else if v.Protocol == "UDP" {
		v.TransportProtocol = 17
	} else if v.Protocol == "ICMP" {
		v.TransportProtocol = 1
	}

	// TBD fill in direction
}

// Transform transforms a flow before being stored
func (ft *vpclogsFlowTransformer) Transform(f *flow.Flow) interface{} {
	logging.GetLogger().Debugf("flow = %s", f)
	updateCount := ft.setUpdateCount(f)
	status := ft.getStatus(f, updateCount)
	logging.GetLogger().Debugf("Transform, UUID = %s, status = %s, updateCount = %d", f.UUID, status, updateCount)

	// do not report new flows (i.e. the first time you see them)
	// TBD is this correct?
	if ft.excludeStartedFlows && status == "STARTED" {
		return nil
	}

	vpclogFlow := &VpclogsFlow{
		Flow:             f,
		FinishType:       f.FinishType,
		Status:           status,
		Protocol:         f.Transport.Protocol.String(),
		LastUpdateMetric: f.LastUpdateMetric,
		Metric:           f.Metric,
		Start:            f.Start,
		TID:              f.NodeTID,
		InitiatorIp:      f.Network.A,
		TargetIp:         f.Network.B,
		InitiatorPort:    f.Transport.A,
		TargetPort:       f.Transport.B,
	}
	vpclogFlow.DeriveExternalizedFields()
	return vpclogFlow
}
