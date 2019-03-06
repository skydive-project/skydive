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

// SecurityAdvisorFlow represents a transformed flow
type SecurityAdvisorFlow struct {
	UUID             string
	LayersPath       string
	Network          *flow.FlowLayer
	Transport        *flow.TransportLayer
	LastUpdateMetric *flow.FlowMetric
	Metric           *flow.FlowMetric
	Start            int64
	Last             int64
	FinishType       flow.FlowFinishType
}

// SecurityAdvisorFlowTransformer is a custom transformer for flows
type SecurityAdvisorFlowTransformer struct {
	seenFlows *cache.Cache
}

// Transform transforms a flow before being stored
func (ft *SecurityAdvisorFlowTransformer) Transform(f *flow.Flow) interface{} {
	// do not report new flows (i.e. the first time you see them)
	if f.FinishType != flow.FlowFinishType_TIMEOUT {
		_, seen := ft.seenFlows.Get(f.UUID)
		if f.FinishType == flow.FlowFinishType_NOT_FINISHED {
			ft.seenFlows.Set(f.UUID, true, cache.DefaultExpiration)
			if !seen {
				return nil
			}
		} else {
			ft.seenFlows.Set(f.UUID, true, time.Minute)
		}
	} else {
		ft.seenFlows.Delete(f.UUID)
	}

	return &SecurityAdvisorFlow{
		UUID:             f.UUID,
		LayersPath:       f.LayersPath,
		Network:          f.Network,
		Transport:        f.Transport,
		LastUpdateMetric: f.LastUpdateMetric,
		Metric:           f.Metric,
		Start:            f.Start,
		Last:             f.Last,
		FinishType:       f.FinishType,
	}
}

// NewSecurityAdvisorFlowTransformer returns a new SecurityAdvisorFlowTransformer
func NewSecurityAdvisorFlowTransformer() *SecurityAdvisorFlowTransformer {
	return &SecurityAdvisorFlowTransformer{
		seenFlows: cache.New(10*time.Minute, 10*time.Minute),
	}
}
