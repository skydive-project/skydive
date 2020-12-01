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
	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/logging"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// FlowSubscriberEndpoint sends all the flows to its subscribers.
type FlowSubscriberEndpoint struct {
	insanelock.RWMutex
	pool         ws.StructSpeakerPool
	nsSubscriber map[string][]ws.Speaker
}

const (
	flowNS  = "flow"
	statsNS = "stats"
)

func (fs *FlowSubscriberEndpoint) sendFlows(ns string, flows []*flow.Flow) {
	fs.RLock()
	_, ok := fs.nsSubscriber[ns]
	fs.RUnlock()

	// at least one speaker for the flow namespace
	if ok {
		msg := ws.NewStructMessage(ns, "store", flows)
		fs.pool.BroadcastMessage(msg)
	}
}

// SendFlows sends flow to the subscribers
func (fs *FlowSubscriberEndpoint) SendFlows(flows []*flow.Flow) {
	fs.sendFlows(flowNS, flows)
	flowsByCaptureMap := make(map[string][]*flow.Flow)
	for _, f := range flows {
		if _, ok := flowsByCaptureMap[f.CaptureID]; !ok {
			flowsByCaptureMap[f.CaptureID] = []*flow.Flow{}
		}
		flowsByCaptureMap[f.CaptureID] = append(flowsByCaptureMap[f.CaptureID], f)

	}
	for captureID, flowsByCapture := range flowsByCaptureMap {
		fs.sendFlows(flowNS+"/"+captureID, flowsByCapture)
	}
}

// SendStats send stats to subscribers
func (fs *FlowSubscriberEndpoint) SendStats(stats *flow.Stats) {
	fs.RLock()
	_, ok := fs.nsSubscriber[statsNS]
	fs.RUnlock()

	// at least one speaker for the flow namespace
	if ok {
		msg := ws.NewStructMessage(statsNS, "stats", stats)
		fs.pool.BroadcastMessage(msg)
	}
}

// OnConnected Server interface
func (fs *FlowSubscriberEndpoint) OnConnected(c ws.Speaker) error {
	namespaces, ok := c.GetHeaders()["X-Websocket-Namespace"]
	if !ok {
		namespaces = []string{flowNS, statsNS}
	}

	fs.Lock()
	for _, ns := range namespaces {
		if speakers, ok := fs.nsSubscriber[ns]; ok {
			speakers = append(speakers, c)
		} else {
			fs.nsSubscriber[ns] = []ws.Speaker{c}
		}
	}
	fs.Unlock()

	logging.GetLogger().Infof("New flow subscriber using namespaces: %v", namespaces)
	return nil
}

// OnDisconnected Server interface
func (fs *FlowSubscriberEndpoint) OnDisconnected(c ws.Speaker) {
	fs.Lock()
	defer fs.Unlock()

	for ns, speakers := range fs.nsSubscriber {
		for i := range speakers {
			speakers = append(speakers[:i], speakers[i+1:]...)

			if len(speakers) == 0 {
				delete(fs.nsSubscriber, ns)
			}
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
		nsSubscriber: make(map[string][]ws.Speaker),
	}
	srv.AddEventHandler(t)
	return t
}
