/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package graph

import (
	"encoding/json"
	"errors"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	ws "github.com/skydive-project/skydive/websocket"
)

const (
	// Namespace used for WebSocket message
	Namespace = "Graph"
)

// Graph message type
const (
	SyncMsgType                 = "Sync"
	SyncRequestMsgType          = "SyncRequest"
	SyncReplyMsgType            = "SyncReply"
	NodeUpdatedMsgType          = "NodeUpdated"
	NodeDeletedMsgType          = "NodeDeleted"
	NodeAddedMsgType            = "NodeAdded"
	EdgeUpdatedMsgType          = "EdgeUpdated"
	EdgeDeletedMsgType          = "EdgeDeleted"
	EdgeAddedMsgType            = "EdgeAdded"
	NodePartiallyUpdatedMsgType = "NodePartiallyUpdated"
	EdgePartiallyUpdatedMsgType = "EdgePartiallyUpdated"
)

// Graph error message
var (
	ErrSyncRequestMalFormed = errors.New("SyncRequestMsg malformed")
	ErrSyncMsgMalFormed     = errors.New("SyncMsg/SyncReplyMsg malformed")
)

// SyncRequestMsg describes a graph synchro request message
type SyncRequestMsg struct {
	graph.Context
	GremlinFilter *string
}

// SyncMsg describes graph synchro message
type SyncMsg struct {
	*graph.Elements
}

// PartiallyUpdatedMsg describes multiple graph modifications
type PartiallyUpdatedMsg struct {
	ID  graph.Identifier
	Ops []graph.PartiallyUpdatedOp
}

// NewStructMessage returns a new graffiti websocket StructMessage
func NewStructMessage(typ string, i interface{}) *ws.StructMessage {
	return ws.NewStructMessage(Namespace, typ, i)
}

// UnmarshalJSON custom unmarshal function
func (s *SyncRequestMsg) UnmarshalJSON(b []byte) error {
	raw := struct {
		Time          int64
		GremlinFilter *string
	}{}

	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if raw.Time != 0 {
		s.TimeSlice = common.NewTimeSlice(raw.Time, raw.Time)
	}
	s.GremlinFilter = raw.GremlinFilter

	return nil
}

// UnmarshalMessage unmarshal graph message
func UnmarshalMessage(msg *ws.StructMessage) (string, interface{}, error) {
	switch msg.Type {
	case SyncRequestMsgType:
		var syncRequest SyncRequestMsg
		if err := json.Unmarshal(msg.Obj, &syncRequest); err != nil {
			return "", msg, err
		}
		return msg.Type, &syncRequest, nil
	case SyncMsgType, SyncReplyMsgType:
		var syncMsg SyncMsg
		if err := json.Unmarshal(msg.Obj, &syncMsg); err != nil {
			return "", msg, err
		}
		return msg.Type, &syncMsg, nil
	case NodeUpdatedMsgType, NodeDeletedMsgType, NodeAddedMsgType:
		var node graph.Node
		if err := json.Unmarshal(msg.Obj, &node); err != nil {
			return "", msg, err
		}
		return msg.Type, &node, nil
	case EdgeUpdatedMsgType, EdgeDeletedMsgType, EdgeAddedMsgType:
		var edge graph.Edge
		if err := json.Unmarshal(msg.Obj, &edge); err != nil {
			return "", msg, err
		}
		return msg.Type, &edge, nil
	case NodePartiallyUpdatedMsgType, EdgePartiallyUpdatedMsgType:
		var pu PartiallyUpdatedMsg
		if err := json.Unmarshal(msg.Obj, &pu); err != nil {
			return "", msg, err
		}

		return msg.Type, &pu, nil
	}

	return "", msg, nil
}
