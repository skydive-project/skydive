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

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/common"
	awsflowlogs "github.com/skydive-project/skydive/contrib/exporters/awsflowlogs/mod"
)

type mangleLogStatus struct {
	linkIDs map[int64]bool
	flows   map[string]*SecurityAdvisorFlow
}

// Mangle log-status book keeping
func (m *mangleLogStatus) Mangle(in []interface{}) (out []interface{}) {
	now := common.UnixMillis(time.Now())

	// OK: all flows
	for _, flow := range in {
		flow := flow.(*SecurityAdvisorFlow)
		flow.LogStatus = string(awsflowlogs.LogStatusOk)
		out = append(out, flow)
	}

	// NODATA: when nothing was seen per intrerface in measurement window
	linkIDs := make(map[int64]bool)
	for _, flow := range in {
		flow := flow.(*SecurityAdvisorFlow)
		linkIDs[flow.LinkID] = true
	}

	for linkID := range m.linkIDs {
		if !linkIDs[linkID] {
			flow := &SecurityAdvisorFlow{
				LinkID:    linkID,
				Start:     now,
				Last:      now,
				LogStatus: string(awsflowlogs.LogStatusNoData),
			}
			out = append(out, flow)
		}

	}

	for _, flow := range in {
		flow := flow.(*SecurityAdvisorFlow)
		m.linkIDs[flow.LinkID] = true
	}

	// SKIPDATA: flows were lost due to error or resource constraint
	for _, flow := range in {
		flow := flow.(*SecurityAdvisorFlow)
		if _, ok := m.flows[flow.UUID]; ok {
			if flow.LastUpdateMetric != m.flows[flow.UUID].Metric {
				skipFlow := &SecurityAdvisorFlow{
					UUID:         flow.UUID,
					LinkID:       flow.LinkID,
					Network:      flow.Network,
					Start:        now,
					Last:         now,
					L3TrackingID: flow.L3TrackingID,
					LogStatus:    string(awsflowlogs.LogStatusSkipData),
				}
				out = append(out, skipFlow)
			}
		}
	}

	for _, flow := range in {
		flow := flow.(*SecurityAdvisorFlow)
		m.flows[flow.UUID] = flow
	}

	return
}

// NewMangleLogStatus create a new mangle
func NewMangleLogStatus(cfg *viper.Viper) (interface{}, error) {
	return &mangleLogStatus{
		linkIDs: make(map[int64]bool),
		flows:   make(map[string]*SecurityAdvisorFlow),
	}, nil
}
