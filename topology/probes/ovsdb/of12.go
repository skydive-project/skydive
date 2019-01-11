/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package ovsdb

import (
	"time"

	goloxi "github.com/skydive-project/goloxi"
	"github.com/skydive-project/goloxi/of12"
	"github.com/skydive-project/goloxi/of13"
	"github.com/skydive-project/goloxi/of15"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/openflow"
)

type of12Handler struct {
	openflow.OpenFlow12Protocol
	probe *ofProbe
}

func (h *of12Handler) OnMessage(msg goloxi.Message) {
	switch t := msg.(type) {
	case *of12.FlowStatsReply:
		now := time.Now().UTC()
		for _, entry := range t.GetEntries() {
			var actions, writeActions []goloxi.IAction
			for _, instruction := range entry.GetInstructions() {
				if instruction, ok := instruction.(of13.IInstructionApplyActions); ok {
					actions = append(actions, instruction.GetActions()...)
				}
				if instruction, ok := instruction.(*of13.InstructionWriteActions); ok {
					writeActions = append(writeActions, instruction.GetActions()...)
				}
			}

			rule, err := newOfRule(entry.Cookie, entry.TableId, entry.Priority, entry.IdleTimeout, entry.HardTimeout, 0, of15.FlowModFlags(0), &entry.Match, actions, writeActions)
			if err != nil {
				logging.GetLogger().Error(err)
				return
			}

			stats := Stats{PacketCount: int64(entry.PacketCount), ByteCount: int64(entry.ByteCount)}
			h.probe.handleFlowStats(t.GetXid(), rule, actions, writeActions, stats, now, h.probe.lastUpdateMetric)
		}

		h.probe.lastUpdateMetric = now
	}
}

func (h *of12Handler) NewGroupForwardRequest() (goloxi.Message, error) {
	return nil, common.ErrNotImplemented
}

func (h *of12Handler) NewRoleRequest() (goloxi.Message, error) {
	return nil, common.ErrNotImplemented
}

func (h *of12Handler) NewFlowStatsRequest(match Match) (goloxi.Message, error) {
	request := of12.NewFlowStatsRequest()
	request.TableId = of12.OFPTTAll
	request.OutPort = of12.OFPPAny
	request.OutGroup = of12.OFPGAny
	if match == nil {
		request.Match = *of12.NewMatchV3()
		request.Match.Length = 4
		request.Match.Type = of12.OFPMTOXM
	} else {
		request.Match.SetLength(match.GetLength())
		request.Match.SetType(match.GetType())
		request.Match.SetOxmList(match.GetOxmList())
	}
	return request, nil
}

func (h *of12Handler) NewGroupDescStatsRequest() (goloxi.Message, error) {
	return of12.NewGroupDescStatsRequest(), nil
}
