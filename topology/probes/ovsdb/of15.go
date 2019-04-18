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
	"github.com/skydive-project/goloxi/of15"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/openflow"
)

type of15Handler struct {
	openflow.OpenFlow15Protocol
	probe *ofProbe
}

func (h *of15Handler) mapBuckets(buckets []*of15.Bucket) []*ofBucket {
	ofBuckets := make([]*ofBucket, len(buckets))
	for i, bucket := range buckets {
		ofBucket := newOfBucket(0, 0, 0, bucket.Actions)
		for _, prop := range bucket.Properties {
			switch prop := prop.(type) {
			case *of15.GroupBucketPropWatchGroup:
				ofBucket.WatchGroup = int64(prop.Watch)
			case *of15.GroupBucketPropWatchPort:
				ofBucket.WatchPort = int64(prop.Watch)
			case *of15.GroupBucketPropWeight:
				ofBucket.Weight = int64(prop.Weight)
			}
		}
		ofBuckets[i] = ofBucket
	}
	return ofBuckets
}

func (h *of15Handler) OnMessage(msg goloxi.Message) {
	switch t := msg.(type) {
	case *of15.FlowStatsReply: // Received with ticker and in response to requests
		now := time.Now().UTC()
		for _, entry := range t.GetEntries() {
			var actions, writeActions []goloxi.IAction
			for _, instruction := range entry.GetInstructions() {
				if instruction, ok := instruction.(of15.IInstructionApplyActions); ok {
					actions = append(actions, instruction.GetActions()...)
				}
				if instruction, ok := instruction.(*of15.InstructionWriteActions); ok {
					writeActions = append(writeActions, instruction.GetActions()...)
				}
			}

			rule, err := newOfRule(entry.Cookie, entry.TableId, entry.Priority, entry.IdleTimeout, entry.HardTimeout, entry.Importance, entry.Flags, &entry.Match, actions, writeActions)
			if err != nil {
				logging.GetLogger().Error(err)
				return
			}

			var stats Stats
			for _, oxs := range entry.Stats.OxsFields {
				switch oxs := oxs.(type) {
				case *of15.OxsByteCount:
					stats.ByteCount = int64(oxs.Value)
				case *of15.OxsPacketCount:
					stats.PacketCount = int64(oxs.Value)
				case *of15.OxsFlowCount:
					stats.FlowCount = int64(oxs.Value)
				case *of15.OxsDuration:
					stats.Duration = time.Duration((time.Second*time.Duration(oxs.Value>>32) + time.Duration(oxs.Value&0xffffffff)))
				case *of15.OxsIdleTime:
					stats.IdleTime = time.Duration((time.Second*time.Duration(oxs.Value>>32) + time.Duration(oxs.Value&0xffffffff)))
				}
			}

			h.probe.handleFlowStats(t.GetXid(), rule, actions, writeActions, stats, now, h.probe.lastUpdateMetric)
		}

		h.probe.lastUpdateMetric = now

	case *of15.Requestforward: // Received on group events
		h.probe.g.Lock()
		defer h.probe.g.Unlock()

		switch group := t.Request.(type) {
		case *of15.GroupAdd:
			ofGroup := &ofGroup{GroupType: group.GroupType, ID: int64(group.GroupId), Buckets: h.mapBuckets(group.Buckets)}
			h.probe.handleGroup(ofGroup, false)
		case *of15.GroupModify:
			ofGroup := &ofGroup{GroupType: group.GroupType, ID: int64(group.GroupId), Buckets: h.mapBuckets(group.Buckets)}
			h.probe.handleGroup(ofGroup, false)
		case *of15.GroupDelete:
			if group.GroupId == of15.OFPGAll {
				for _, children := range h.probe.g.LookupChildren(h.probe.node, graph.Metadata{"Type": "ofgroup"}, nil) {
					h.probe.g.DelNode(children)
				}
			} else {
				ofGroup := &ofGroup{GroupType: group.GroupType, ID: int64(group.GroupId), Buckets: h.mapBuckets(group.Buckets)}
				h.probe.handleGroup(ofGroup, true)
			}
		}

	case *of15.GroupDescStatsReply: // Received on initial sync
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

func (h *of15Handler) NewGroupForwardRequest() (goloxi.Message, error) {
	request := of15.NewAsyncSet()
	prop := of15.NewAsyncConfigPropRequestforwardSlave()
	prop.Mask = 0x1
	prop.Length = 8
	request.Properties = append(request.Properties, prop)
	return request, nil
}

func (h *of15Handler) NewRoleRequest() (goloxi.Message, error) {
	request := of15.NewRoleRequest()
	request.Role = of15.OFPCRRoleSlave
	request.GenerationId = 0x8000000000000002
	return request, nil
}

func (h *of15Handler) NewFlowStatsRequest(match Match) (goloxi.Message, error) {
	request := of15.NewFlowStatsRequest()
	request.TableId = of15.OFPTTAll
	request.OutPort = of15.OFPPAny
	request.OutGroup = of15.OFPGAny
	if match == nil {
		request.Match = *of15.NewMatchV3()
		request.Match.Length = 4
		request.Match.Type = of15.OFPMTOXM
	} else {
		request.Match.SetLength(match.GetLength())
		request.Match.SetType(match.GetType())
		request.Match.SetOxmList(match.GetOxmList())
	}
	return request, nil
}

func (h *of15Handler) NewGroupDescStatsRequest() (goloxi.Message, error) {
	request := of15.NewGroupDescStatsRequest()
	request.GroupId = of15.OFPGAll
	return request, nil
}
