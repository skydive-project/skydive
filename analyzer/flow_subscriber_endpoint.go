/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package analyzer

import (
	"github.com/skydive-project/skydive/flow"
	ws "github.com/skydive-project/skydive/websocket"
)

// FlowSubscriberEndpoint sends all the flows to its subscribers.
type FlowSubscriberEndpoint struct {
	pool ws.StructSpeakerPool
}

// SendFlows sends flow to the subscribers
func (fs *FlowSubscriberEndpoint) SendFlows(flows []*flow.Flow) {
	msg := ws.NewStructMessage("flow", "store", flows)
	fs.pool.BroadcastMessage(msg)
}

// NewFlowSubscriberEndpoint returns a new server to be used by external flow subscribers
func NewFlowSubscriberEndpoint(pool ws.StructSpeakerPool) *FlowSubscriberEndpoint {
	t := &FlowSubscriberEndpoint{
		pool: pool,
	}
	return t
}
