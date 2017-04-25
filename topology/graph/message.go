/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package graph

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
)

const (
	SyncRequestMsgType      = "SyncRequest"
	SyncReplyMsgType        = "SyncReply"
	HostGraphDeletedMsgType = "HostGraphDeleted"
	NodeUpdatedMsgType      = "NodeUpdated"
	NodeDeletedMsgType      = "NodeDeleted"
	NodeAddedMsgType        = "NodeAdded"
	EdgeUpdatedMsgType      = "EdgeUpdated"
	EdgeDeletedMsgType      = "EdgeDeleted"
	EdgeAddedMsgType        = "EdgeAdded"
)

var (
	SyncRequestMalFormed  = errors.New("SyncRequestMsg malformed")
	SyncReplyMsgMalFormed = errors.New("SyncReplyMsg malformed")
)

type SyncReplyMsg struct {
	Nodes []*Node
	Edges []*Edge
}

func UnmarshalWSMessage(msg shttp.WSMessage) (string, interface{}, error) {
	var obj interface{}
	if err := common.JsonDecode(bytes.NewReader([]byte(*msg.Obj)), &obj); err != nil {
		return "", msg, err
	}

	switch msg.Type {
	case SyncRequestMsgType:
		m, ok := obj.(map[string]interface{})
		if !ok {
			return "", msg, SyncRequestMalFormed
		}

		var context GraphContext
		switch v := m["Time"].(type) {
		case json.Number:
			i, err := v.Int64()
			if err != nil {
				return "", msg, err
			}
			context.TimeSlice = common.NewTimeSlice(i, i)
		}
		return msg.Type, context, nil
	case SyncReplyMsgType:
		result := &SyncReplyMsg{}

		els, ok := obj.(map[string]interface{})
		if !ok {
			return "", msg, SyncReplyMsgMalFormed
		}
		inodes, ok := els["Nodes"]
		if !ok || inodes == nil {
			return "", result, nil
		}
		nodes, ok := inodes.([]interface{})
		if !ok {
			return "", msg, SyncReplyMsgMalFormed
		}

		for _, n := range nodes {
			var node Node
			if err := node.Decode(n); err != nil {
				return "", msg, err
			}
			result.Nodes = append(result.Nodes, &node)
		}

		iedges, ok := els["Edges"]
		if !ok || iedges == nil {
			return "", result, nil
		}

		edges, ok := iedges.([]interface{})
		if !ok {
			return "", msg, SyncReplyMsgMalFormed
		}
		for _, e := range edges {
			var edge Edge
			if err := edge.Decode(e); err != nil {
				return "", msg, err
			}
			result.Edges = append(result.Edges, &edge)
		}

		return msg.Type, result, nil
	case HostGraphDeletedMsgType:
		return msg.Type, obj, nil
	case NodeUpdatedMsgType, NodeDeletedMsgType, NodeAddedMsgType:
		var node Node
		if err := node.Decode(obj); err != nil {
			return "", msg, err
		}

		return msg.Type, &node, nil
	case EdgeUpdatedMsgType, EdgeDeletedMsgType, EdgeAddedMsgType:
		var edge Edge
		if err := edge.Decode(obj); err != nil {
			return "", msg, err
		}

		return msg.Type, &edge, nil
	}

	return "", msg, nil
}
