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

package flow

import (
	"github.com/golang/protobuf/proto"
	"github.com/skydive-project/skydive/graffiti/logging"
	ws "github.com/skydive-project/skydive/websocket"
)

// Namespace "Flow"
const (
	Namespace = "Flow"
)

// WSTableServer describes a mechanism to Query a flow table via Websocket
type WSTableServer struct {
	TableAllocator *TableAllocator
}

// OnTableQuery event
func (s *WSTableServer) OnTableQuery(c ws.Speaker, msg *ws.StructMessage) {
	var query TableQuery
	if err := proto.Unmarshal(msg.Obj, &query); err != nil {
		logging.GetLogger().Errorf("Unable to decode search flow message %v", msg)
		return
	}

	result := s.TableAllocator.QueryTable(&query)
	reply := msg.Reply(result, "TableResult", int(result.Status))
	c.SendMessage(reply)
}

// OnStructMessage TableQuery
func (s *WSTableServer) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	switch msg.Type {
	case "TableQuery":
		s.OnTableQuery(c, msg)
	}
}

// NewWSTableServer creates a new flow table query server based on websocket
func NewWSTableServer(allocator *TableAllocator, pool ws.StructSpeakerPool) *WSTableServer {
	s := &WSTableServer{
		TableAllocator: allocator,
	}
	pool.AddStructMessageHandler(s, []string{Namespace})
	return s
}
