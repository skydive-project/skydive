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

package server

import (
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	ws "github.com/skydive-project/skydive/websocket"
)

// FlowSubscriberEndpoint sends all the flows to its subscribers.
type FlowSubscriberEndpoint struct {
	common.RWMutex
	pool         ws.StructSpeakerPool
	nsSubscriber map[string]ws.Speaker
}

const flowNS = "flow"

// SendFlows sends flow to the subscribers
func (fs *FlowSubscriberEndpoint) SendFlows(flows []*flow.Flow) {
	fs.RLock()
	_, ok := fs.nsSubscriber[flowNS]
	fs.RUnlock()

	if ok {
		msg := ws.NewStructMessage(flowNS, "store", flows)
		fs.pool.BroadcastMessage(msg)
	}
}

// OnConnected Server interface
func (fs *FlowSubscriberEndpoint) OnConnected(c ws.Speaker) {
	ns := flowNS

	if namespaces, ok := c.GetHeaders()["X-Websocket-Namespace"]; ok {
		if len(namespaces) > 0 {
			ns = namespaces[0]
		}
	}

	fs.Lock()
	fs.nsSubscriber[ns] = c
	fs.Unlock()

	logging.GetLogger().Infof("New flow subscriber using namespace: %s", ns)
}

// OnDisconnected Server interface
func (fs *FlowSubscriberEndpoint) OnDisconnected(c ws.Speaker) {
	fs.Lock()
	defer fs.Unlock()

	for k, nc := range fs.nsSubscriber {
		if nc == c {
			delete(fs.nsSubscriber, k)
		}
	}
}

// OnMessage Server interface
func (fs *FlowSubscriberEndpoint) OnMessage(c ws.Speaker, m ws.Message) {
}

// NewFlowSubscriberEndpoint returns a new server to be used by external flow subscribers
func NewFlowSubscriberEndpoint(srv *ws.StructServer) *FlowSubscriberEndpoint {
	t := &FlowSubscriberEndpoint{
		pool:         srv,
		nsSubscriber: make(map[string]ws.Speaker),
	}
	srv.AddEventHandler(t)
	return t
}
