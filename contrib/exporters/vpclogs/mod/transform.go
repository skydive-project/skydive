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
	"strconv"
	"time"

	cache "github.com/pmylund/go-cache"

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/contrib/exporters/core"
	sa "github.com/skydive-project/skydive/contrib/exporters/secadvisor/mod"
)

// NewTransform creates a new flow transformer based on a name string
func NewTransform(cfg *viper.Viper) (interface{}, error) {
	excludeStartedFlows := cfg.GetBool(core.CfgRoot + "transform.secadvisor.exclude_started_flows")

	runcResolver := sa.NewResolveRunc(cfg)
	resolver := sa.NewResolveMulti(runcResolver)
	resolver = sa.NewResolveFallback(resolver)
	resolver = sa.NewResolveCache(resolver)

	return &vpclogsFlowTransformer{
		resolver:             resolver,
		flowUpdateCountCache: cache.New(10*time.Minute, 10*time.Minute),
		excludeStartedFlows:  excludeStartedFlows,
	}, nil
}

const version = "1.0.1"

// VpclogsFlowLayer is the flow layer for a vpc log flow
type VpclogsFlowLayer struct {
	Protocol string `json:"Protocol,omitempty"`
	A        string `json:"A,omitempty"`
	B        string `json:"B,omitempty"`
	AName    string `json:"A_Name,omitempty"`
	BName    string `json:"B_Name,omitempty"`
}

// VpclogsFlow represents a vpc log flow
// this structure will be updated when the fields are finalized
type VpclogsFlow struct {
	UUID             string            `json:"UUID,omitempty"`
	LayersPath       string            `json:"LayersPath,omitempty"`
	Version          string            `json:"Version,omitempty"`
	Status           string            `json:"Status,omitempty"`
	FinishType       string            `json:"FinishType,omitempty"`
	Network          *VpclogsFlowLayer `json:"Network,omitempty"`
	Transport        *VpclogsFlowLayer `json:"Transport,omitempty"`
	LastUpdateMetric *flow.FlowMetric  `json:"LastUpdateMetric,omitempty"`
	Metric           *flow.FlowMetric  `json:"Metric,omitempty"`
	Start            int64             `json:"Start"`
	Last             int64             `json:"Last"`
	UpdateCount      int64             `json:"UpdateCount"`
	NodeType         string            `json:"NodeType,omitempty"`
	Action           string            `json:"Action,omitempty"`
	TrackingID       string            `json:"-"`
	TID              string            `json:"-"`
}

// VpclogsFlowTransformer is a custom transformer for flows
type vpclogsFlowTransformer struct {
	resolver             sa.Resolver
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

func (ft *vpclogsFlowTransformer) getFinishType(f *flow.Flow) string {
	if f.FinishType == flow.FlowFinishType_TCP_FIN {
		return "SYN_FIN"
	}
	if f.FinishType == flow.FlowFinishType_TCP_RST {
		return "SYN_RST"
	}
	if f.FinishType == flow.FlowFinishType_TIMEOUT {
		return "Timeout"
	}
	return ""
}

func (ft *vpclogsFlowTransformer) getNetwork(f *flow.Flow) *VpclogsFlowLayer {
	if f.Network == nil {
		return nil
	}

	aName, _ := ft.resolver.IPToName(f.Network.A, f.NodeTID)
	bName, _ := ft.resolver.IPToName(f.Network.B, f.NodeTID)

	return &VpclogsFlowLayer{
		Protocol: f.Network.Protocol.String(),
		A:        f.Network.A,
		B:        f.Network.B,
		AName:    aName,
		BName:    bName,
	}
}

func (ft *vpclogsFlowTransformer) getTransport(f *flow.Flow) *VpclogsFlowLayer {
	if f.Transport == nil {
		return nil
	}

	return &VpclogsFlowLayer{
		Protocol: f.Transport.Protocol.String(),
		A:        strconv.FormatInt(f.Transport.A, 10),
		B:        strconv.FormatInt(f.Transport.B, 10),
	}
}

func (v *VpclogsFlow) SetAction(action string) {
	v.Action = action
}

// Transform transforms a flow before being stored
func (ft *vpclogsFlowTransformer) Transform(f *flow.Flow) interface{} {

	logging.GetLogger().Debugf("flow = %s", f)
	updateCount := ft.setUpdateCount(f)
	status := ft.getStatus(f, updateCount)

	// do not report new flows (i.e. the first time you see them)
	if ft.excludeStartedFlows && status == "STARTED" {
		return nil
	}

	nodeType, _ := ft.resolver.TIDToType(f.NodeTID)

	return &VpclogsFlow{
		UUID:             f.UUID,
		LayersPath:       f.LayersPath,
		Version:          version,
		Status:           status,
		FinishType:       ft.getFinishType(f),
		Network:          ft.getNetwork(f),
		Transport:        ft.getTransport(f),
		LastUpdateMetric: f.LastUpdateMetric,
		Metric:           f.Metric,
		Start:            f.Start,
		Last:             f.Last,
		UpdateCount:      updateCount,
		NodeType:         nodeType,
		TrackingID:       f.L3TrackingID,
		TID:              f.NodeTID,
	}
}
