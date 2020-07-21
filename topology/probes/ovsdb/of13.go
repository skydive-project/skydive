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
	"errors"
	"time"

	goloxi "github.com/skydive-project/goloxi"
	"github.com/skydive-project/goloxi/of13"
	"github.com/skydive-project/goloxi/of15"
	"github.com/skydive-project/skydive/openflow"
)

type of13Handler struct {
	openflow.OpenFlow13Protocol
	probe *ofProbe
}

func (h *of13Handler) OnMessage(msg goloxi.Message) {
	switch t := msg.(type) {
	case *of13.FlowStatsReply: // Received with ticker and in response to requests
		now := time.Now().UTC()
		for _, entry := range t.GetEntries() {
			var actions, writeActions []goloxi.IAction
			var gotoTable uint8
			for _, instruction := range entry.GetInstructions() {
				if instruction, ok := instruction.(of13.IInstructionApplyActions); ok {
					actions = append(actions, instruction.GetActions()...)
				}
				if instruction, ok := instruction.(*of13.InstructionWriteActions); ok {
					writeActions = append(writeActions, instruction.GetActions()...)
				}
				if instruction, ok := instruction.(*of13.InstructionGotoTable); ok {
					gotoTable = instruction.GetTableId()
				}
			}

			rule, err := newOfRule(entry.Cookie, entry.TableId, entry.Priority, entry.IdleTimeout, entry.HardTimeout, 0, of15.FlowModFlags(0), &entry.Match, actions, writeActions, gotoTable)
			if err != nil {
				h.probe.Ctx.Logger.Error(err)
				return
			}

			stats := Stats{PacketCount: int64(entry.PacketCount), ByteCount: int64(entry.ByteCount)}
			h.probe.handleFlowStats(t.GetXid(), rule, actions, writeActions, stats, now, h.probe.lastUpdateMetric)
		}

		h.probe.lastUpdateMetric = now
	}
}

func (h *of13Handler) NewGroupForwardRequest() (goloxi.Message, error) {
	return nil, errors.New("Not supported on OpenFlow 1.3")
}

func (h *of13Handler) NewRoleRequest() (goloxi.Message, error) {
	return nil, errors.New("Not supported on OpenFlow 1.3")
}

func (h *of13Handler) NewFlowStatsRequest(match Match) (goloxi.Message, error) {
	request := of13.NewFlowStatsRequest()
	request.TableId = of13.OFPTTAll
	request.OutPort = of13.OFPPAny
	request.OutGroup = of13.OFPGAny
	if match == nil {
		request.Match = *of13.NewMatchV3()
		request.Match.Length = 4
		request.Match.Type = of13.OFPMTOXM
	} else {
		request.Match.SetLength(match.GetLength())
		request.Match.SetType(match.GetType())
		request.Match.SetOxmList(match.GetOxmList())
	}
	return request, nil
}

func (h *of13Handler) NewGroupDescStatsRequest() (goloxi.Message, error) {
	return of13.NewGroupDescStatsRequest(), nil
}
