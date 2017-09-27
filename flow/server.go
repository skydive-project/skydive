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

package flow

import (
	"encoding/json"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

// Namespace "Flow"
const (
	Namespace = "Flow"
)

// TableServer describes a mechanism to Query a flow table via Websocket
type TableServer struct {
	TableAllocator *TableAllocator
}

// OnTableQuery event
func (s *TableServer) OnTableQuery(c shttp.WSSpeaker, msg shttp.WSJSONMessage) {
	var query TableQuery
	if err := json.Unmarshal([]byte(*msg.Obj), &query); err != nil {
		logging.GetLogger().Errorf("Unable to decode search flow message %v", msg)
		return
	}

	result := s.TableAllocator.QueryTable(&query)
	reply := msg.Reply(result, "TableResult", result.status)
	c.Send(reply)
}

// OnWSMessage TableQuery
func (s *TableServer) OnWSJSONMessage(c shttp.WSSpeaker, msg shttp.WSJSONMessage) {
	switch msg.Type {
	case "TableQuery":
		s.OnTableQuery(c, msg)
	}
}

// NewServer creates a new flow table query server based on websocket
func NewServer(allocator *TableAllocator, pool shttp.WSJSONSpeakerPool) *TableServer {
	s := &TableServer{
		TableAllocator: allocator,
	}
	pool.AddJSONMessageHandler(s, []string{Namespace})
	return s
}
