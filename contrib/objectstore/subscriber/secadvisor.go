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

package subscriber

import (
	"time"

	"github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/flow"
)

const version = "1.0.8"

// SecurityAdvisorFlow represents a transformed flow
type SecurityAdvisorFlow struct {
	UUID             string
	LayersPath       string
	Version          string
	Status           string
	FinishType       string
	Network          *flow.FlowLayer
	Transport        *flow.TransportLayer
	LastUpdateMetric *flow.FlowMetric
	Metric           *flow.FlowMetric
	Start            int64
	Last             int64
	UpdateCount      int64
}

// SecurityAdvisorFlowTransformer is a custom transformer for flows
type SecurityAdvisorFlowTransformer struct {
	flowUpdateCount *cache.Cache
}

func (ft *SecurityAdvisorFlowTransformer) setUpadateCount(f *flow.Flow) int64 {
	var count int64
	if countRaw, ok := ft.flowUpdateCount.Get(f.UUID); ok {
		count = countRaw.(int64)
	}

	if f.FinishType != flow.FlowFinishType_TIMEOUT {
		if f.FinishType == flow.FlowFinishType_NOT_FINISHED {
			ft.flowUpdateCount.Set(f.UUID, count+1, cache.DefaultExpiration)
		} else {
			ft.flowUpdateCount.Set(f.UUID, count+1, time.Minute)
		}
	} else {
		ft.flowUpdateCount.Delete(f.UUID)
	}

	return count
}

func (ft *SecurityAdvisorFlowTransformer) getStatus(f *flow.Flow, updateCount int64) string {
	if f.FinishType != flow.FlowFinishType_NOT_FINISHED {
		return "ENDED"
	}

	if updateCount == 0 {
		return "STARTED"
	}

	return "UPDATED"
}

func (ft *SecurityAdvisorFlowTransformer) getFinishType(f *flow.Flow) string {
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

// Transform transforms a flow before being stored
func (ft *SecurityAdvisorFlowTransformer) Transform(f *flow.Flow, tag Tag) interface{} {
	// do not report flows that are neither ingress or egress
	if tag != tagIngress && tag != tagEgress {
		return nil
	}

	updateCount := ft.setUpadateCount(f)
	status := ft.getStatus(f, updateCount)

	// do not report new flows (i.e. the first time you see them)
	if status == "STARTED" {
		return nil
	}

	return &SecurityAdvisorFlow{
		UUID:             f.UUID,
		LayersPath:       f.LayersPath,
		Version:          version,
		Status:           status,
		FinishType:       ft.getFinishType(f),
		Network:          f.Network,
		Transport:        f.Transport,
		LastUpdateMetric: f.LastUpdateMetric,
		Metric:           f.Metric,
		Start:            f.Start,
		Last:             f.Last,
		UpdateCount:      updateCount,
	}
}

// NewSecurityAdvisorFlowTransformer returns a new SecurityAdvisorFlowTransformer
func NewSecurityAdvisorFlowTransformer() *SecurityAdvisorFlowTransformer {
	return &SecurityAdvisorFlowTransformer{
		flowUpdateCount: cache.New(10*time.Minute, 10*time.Minute),
	}
}
