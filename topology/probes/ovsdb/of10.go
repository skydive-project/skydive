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
	"github.com/davecgh/go-spew/spew"
	goloxi "github.com/skydive-project/goloxi"
	"github.com/skydive-project/goloxi/of10"
	"github.com/skydive-project/goloxi/of14"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/openflow"
)

type of10Handler struct {
	openflow.OpenFlow10Protocol
	probe *ofProbe
}

func (h *of10Handler) OnMessage(msg goloxi.Message) {
	probe := h.probe
	switch t := msg.(type) {
	case *of10.NiciraFlowMonitorReply: // Received on connection and on events
		logging.GetLogger().Debugf("Handling flow monitor %s", spew.Sdump(t))
		nxm2oxm := func(nxm of14.INiciraMatch, matchLen uint16) *of14.MatchV3 {
			oxm := of14.NewMatchV3()
			for _, e := range nxm.GetNxmEntries() {
				oxm.OxmList = append(oxm.OxmList, e)
			}
			oxm.Length = matchLen + 4
			oxm.Type = of14.OFPMTOXM
			return oxm
		}

		for _, update := range t.Updates {
			switch u := update.(type) {
			case *of10.NiciraFlowUpdateFullAdd:
				rule, err := newOfRule(u.Cookie, u.TableId, u.Priority, u.IdleTimeout, u.HardTimeout, 0, of14.FlowModFlags(0), nxm2oxm(&u.Match, u.MatchLen), u.Actions, nil)
				if err != nil {
					logging.GetLogger().Errorf("Failed to parse update: %s", err)
					continue
				}
				msg, _ := probe.handler.NewFlowStatsRequest(nxm2oxm(&u.Match, u.MatchLen))
				probe.client.PrepareMessage(msg)
				probe.Lock()
				probe.requests[msg.GetXid()] = rule
				probe.Unlock()
				probe.client.SendMessage(msg)
			case *of10.NiciraFlowUpdateFullDeleted:
				monitorRule, err := newOfRule(u.Cookie, u.TableId, u.Priority, u.IdleTimeout, u.HardTimeout, 0, of14.FlowModFlags(0), nxm2oxm(&u.Match, u.MatchLen), u.Actions, nil)
				if err != nil {
					logging.GetLogger().Errorf("Failed to parse update: %s", err)
					continue
				}
				monitorID := monitorRule.GetID(probe.g.GetHost(), probe.bridge)
				probe.Lock()
				ruleID := probe.rules[monitorID]
				delete(probe.rules, monitorID)
				probe.Unlock()

				probe.g.Lock()
				if n := probe.g.GetNode(ruleID); n != nil {
					probe.g.DelNode(n)
				}
				probe.g.Unlock()
			}
		}
	}
}
