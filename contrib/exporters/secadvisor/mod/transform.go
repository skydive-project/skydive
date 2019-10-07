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

	"github.com/skydive-project/skydive/contrib/exporters/core"
	"github.com/skydive-project/skydive/flow"
)

// NewTransform creates a new flow transformer based on a name string
func NewTransform(cfg *viper.Viper) (interface{}, error) {
	excludeStartedFlows := cfg.GetBool(core.CfgRoot + "transform.secadvisor.exclude_started_flows")

	runcResolver := NewResolveRunc(cfg)
	dockerResolver := NewResolveDocker(cfg)
	vmResolver := NewResolveVM(cfg)
	resolver := NewResolveMulti(runcResolver, dockerResolver, vmResolver)
	resolver = NewResolveFallback(resolver)
	resolver = NewResolveCache(resolver)

	return &securityAdvisorFlowTransformer{
		resolver:             resolver,
		flowUpdateCountCache: cache.New(10*time.Minute, 10*time.Minute),
		excludeStartedFlows:  excludeStartedFlows,
	}, nil
}

const version = "1.0.8"

// SecurityAdvisorFlowLayer is the flow layer for a security advisor flow
type SecurityAdvisorFlowLayer struct {
	Protocol string `json:"Protocol,omitempty"`
	A        string `json:"A,omitempty"`
	B        string `json:"B,omitempty"`
	AName    string `json:"A_Name,omitempty"`
	BName    string `json:"B_Name,omitempty"`
}

// SecurityAdvisorFlow represents a security advisor flow
type SecurityAdvisorFlow struct {
	UUID             string                    `json:"UUID,omitempty"`
	LinkID           int64                     `json:"-"`
	L3TrackingID     string                    `json:"-"`
	LayersPath       string                    `json:"LayersPath,omitempty"`
	Version          string                    `json:"Version,omitempty"`
	Status           string                    `json:"Status,omitempty"`
	FinishType       string                    `json:"FinishType,omitempty"`
	Network          *SecurityAdvisorFlowLayer `json:"Network,omitempty"`
	Transport        *SecurityAdvisorFlowLayer `json:"Transport,omitempty"`
	LastUpdateMetric *flow.FlowMetric          `json:"LastUpdateMetric,omitempty"`
	Metric           *flow.FlowMetric          `json:"Metric,omitempty"`
	Start            int64                     `json:"Start"`
	Last             int64                     `json:"Last"`
	UpdateCount      int64                     `json:"UpdateCount"`
	NodeType         string                    `json:"NodeType,omitempty"`
	LogStatus        string                    `json:"LogStatus,omitempty"`
}

// SecurityAdvisorFlowTransformer is a custom transformer for flows
type securityAdvisorFlowTransformer struct {
	resolver             Resolver
	flowUpdateCountCache *cache.Cache
	excludeStartedFlows  bool
}

func (ft *securityAdvisorFlowTransformer) setUpdateCount(f *flow.Flow) int64 {
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

func (ft *securityAdvisorFlowTransformer) getStatus(f *flow.Flow, updateCount int64) string {
	if f.FinishType != flow.FlowFinishType_NOT_FINISHED {
		return "ENDED"
	}

	if updateCount == 0 {
		return "STARTED"
	}

	return "UPDATED"
}

func (ft *securityAdvisorFlowTransformer) getFinishType(f *flow.Flow) string {
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

func (ft *securityAdvisorFlowTransformer) getNetwork(f *flow.Flow) *SecurityAdvisorFlowLayer {
	if f.Network == nil {
		return nil
	}

	aName, _ := ft.resolver.IPToName(f.Network.A, f.NodeTID)
	bName, _ := ft.resolver.IPToName(f.Network.B, f.NodeTID)

	return &SecurityAdvisorFlowLayer{
		Protocol: f.Network.Protocol.String(),
		A:        f.Network.A,
		B:        f.Network.B,
		AName:    aName,
		BName:    bName,
	}
}

func (ft *securityAdvisorFlowTransformer) getTransport(f *flow.Flow) *SecurityAdvisorFlowLayer {
	if f.Transport == nil {
		return nil
	}

	return &SecurityAdvisorFlowLayer{
		Protocol: f.Transport.Protocol.String(),
		A:        strconv.FormatInt(f.Transport.A, 10),
		B:        strconv.FormatInt(f.Transport.B, 10),
	}
}

func (ft *securityAdvisorFlowTransformer) getLinkID(f *flow.Flow) int64 {
	if f.Link == nil {
		return 0
	}
	return f.Link.ID
}

// Transform transforms a flow before being stored
func (ft *securityAdvisorFlowTransformer) Transform(f *flow.Flow) interface{} {

	updateCount := ft.setUpdateCount(f)
	status := ft.getStatus(f, updateCount)

	// do not report new flows (i.e. the first time you see them)
	if ft.excludeStartedFlows && status == "STARTED" {
		return nil
	}

	nodeType, _ := ft.resolver.TIDToType(f.NodeTID)

	return &SecurityAdvisorFlow{
		UUID:             f.UUID,
		LinkID:           ft.getLinkID(f),
		L3TrackingID:     f.L3TrackingID,
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
	}
}
