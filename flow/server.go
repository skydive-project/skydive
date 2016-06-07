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

	shttp "github.com/redhat-cip/skydive/http"
	"github.com/redhat-cip/skydive/logging"
)

const (
	Namespace = "Flow"
)

type TableServer struct {
	shttp.DefaultWSClientEventHandler
	WSAsyncClient  *shttp.WSAsyncClient
	TableAllocator *TableAllocator
}

func (s *TableServer) OnMessage(msg shttp.WSMessage) {
	if msg.Namespace != Namespace {
		return
	}

	var query TableQuery

	// decode query obj depending on the msg type
	switch msg.Type {
	case "FlowSearchQuery":
		query.Obj = &FlowSearchQuery{}

		err := json.Unmarshal([]byte(*msg.Obj), &query)
		if err != nil {
			logging.GetLogger().Errorf("Unable to decode search flow message %v", msg)
			return
		}
	}

	b, _ := json.Marshal(s.TableAllocator.QueryTable(&query))
	raw := json.RawMessage(b)

	reply := shttp.WSMessage{
		Namespace: Namespace,
		Type:      msg.Type,
		UUID:      msg.UUID,
		Obj:       &raw,
	}

	s.WSAsyncClient.SendWSMessage(reply)
}

func NewServer(allocator *TableAllocator, client *shttp.WSAsyncClient) *TableServer {
	s := &TableServer{
		TableAllocator: allocator,
		WSAsyncClient:  client,
	}
	client.AddEventHandler(s)

	return s
}
