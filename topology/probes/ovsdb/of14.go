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
	"github.com/skydive-project/goloxi/of14"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/openflow"
)

type of14Handler struct {
	probe *ofProbe
	openflow.OpenFlow14Protocol
}

func (h *of14Handler) mapBuckets(buckets []*of14.Bucket) []*ofBucket {
	ofBuckets := make([]*ofBucket, len(buckets))
	for i, bucket := range buckets {
		ofBuckets[i] = newOfBucket(int64(bucket.Weight), int64(bucket.WatchPort), int64(bucket.WatchGroup), bucket.Actions)
	}
	return ofBuckets
}

func (h *of14Handler) OnMessage(msg goloxi.Message) {
	switch t := msg.(type) {
	case *of14.FlowStatsReply: // Received with ticker and in response to requests
		now := time.Now().UTC()
		for _, entry := range t.GetEntries() {
			var actions, writeActions []goloxi.IAction
			for _, instruction := range entry.GetInstructions() {
				if instruction, ok := instruction.(of14.IInstructionApplyActions); ok {
					actions = append(actions, instruction.GetActions()...)
				}
				if instruction, ok := instruction.(*of14.InstructionWriteActions); ok {
					writeActions = append(writeActions, instruction.GetActions()...)
				}
			}

			rule, err := newOfRule(entry.Cookie, entry.TableId, entry.Priority, entry.IdleTimeout, entry.HardTimeout, entry.Importance, entry.Flags, &entry.Match, actions, writeActions)
			if err != nil {
				logging.GetLogger().Error(err)
				return
			}

			stats := Stats{PacketCount: int64(entry.PacketCount), ByteCount: int64(entry.ByteCount)}
			h.probe.handleFlowStats(t.GetXid(), rule, actions, writeActions, stats, now, h.probe.lastUpdateMetric)
		}
		h.probe.lastUpdateMetric = now

	case *of14.Requestforward: // Received on group events
		h.probe.g.Lock()
		defer h.probe.g.Unlock()

		switch group := t.Request.(type) {
		case *of14.GroupAdd:
			ofGroup := &ofGroup{GroupType: group.GroupType, ID: int64(group.GroupId), Buckets: h.mapBuckets(group.Buckets)}
			h.probe.handleGroup(ofGroup, false)
		case *of14.GroupModify:
			ofGroup := &ofGroup{GroupType: group.GroupType, ID: int64(group.GroupId), Buckets: h.mapBuckets(group.Buckets)}
			h.probe.handleGroup(ofGroup, false)
		case *of14.GroupDelete:
			if group.GroupId == of14.OFPGAll {
				for _, children := range h.probe.g.LookupChildren(h.probe.node, graph.Metadata{"Type": "ofgroup"}, nil) {
					h.probe.g.DelNode(children)
				}
			} else {
				ofGroup := &ofGroup{GroupType: group.GroupType, ID: int64(group.GroupId), Buckets: h.mapBuckets(group.Buckets)}
				h.probe.handleGroup(ofGroup, true)
			}
		}

	case *of14.GroupDescStatsReply: // Received on initial sync
		h.probe.g.Lock()
		defer h.probe.g.Unlock()

		for _, group := range t.Entries {
			h.probe.handleGroup(&ofGroup{
				GroupType: group.GroupType,
				ID:        int64(group.GroupId),
				Buckets:   h.mapBuckets(group.Buckets),
			}, false)
		}
	}
}

func (h *of14Handler) NewGroupForwardRequest() (goloxi.Message, error) {
	request := of14.NewAsyncSet()
	prop := of14.NewAsyncConfigPropRequestforwardSlave()
	prop.Mask = 0x1
	prop.Length = 8
	request.Properties = append(request.Properties, prop)
	return request, nil
}

func (h *of14Handler) NewRoleRequest() (goloxi.Message, error) {
	request := of14.NewRoleRequest()
	request.Role = of14.OFPCRRoleSlave
	request.GenerationId = 0x8000000000000002
	return request, nil
}

func (h *of14Handler) NewFlowStatsRequest(match Match) (goloxi.Message, error) {
	request := of14.NewFlowStatsRequest()
	request.TableId = of14.OFPTTAll
	request.OutPort = of14.OFPPAny
	request.OutGroup = of14.OFPGAny
	if match == nil {
		request.Match = *of14.NewMatchV3()
		request.Match.Length = 4
		request.Match.Type = of14.OFPMTOXM
	} else {
		request.Match.SetLength(match.GetLength())
		request.Match.SetType(match.GetType())
		request.Match.SetOxmList(match.GetOxmList())
	}
	return request, nil
}

func (h *of14Handler) NewGroupDescStatsRequest() (goloxi.Message, error) {
	return of14.NewGroupDescStatsRequest(), nil
}
