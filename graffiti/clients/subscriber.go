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

package clients

import (
	"net/http"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/messages"
	"github.com/skydive-project/skydive/graffiti/websocket"
)

type Subscriber struct {
	*websocket.StructSpeaker
	g      *graph.Graph
	logger logging.Logger
}

// OnStructMessage callback
func (s *Subscriber) OnStructMessage(c websocket.Speaker, msg *websocket.StructMessage) {
	if msg.Status != http.StatusOK {
		s.logger.Errorf("request error: %v", msg)
		return
	}

	origin := string(c.GetServiceType())
	if len(c.GetRemoteHost()) > 0 {
		origin += "." + c.GetRemoteHost()
	}

	msgType, obj, err := messages.UnmarshalMessage(msg)
	if err != nil {
		s.logger.Error("unable to parse websocket message: %s", err)
		return
	}

	s.g.Lock()
	defer s.g.Unlock()

	switch msgType {
	case messages.SyncMsgType, messages.SyncReplyMsgType:
		r := obj.(*messages.SyncMsg)

		s.g.DelNodes(graph.Metadata{"Origin": origin})

		for _, n := range r.Nodes {
			if s.g.GetNode(n.ID) == nil {
				if err := s.g.NodeAdded(n); err != nil {
					s.logger.Errorf("%s, %+v", err, n)
				}
			}
		}
		for _, e := range r.Edges {
			if s.g.GetEdge(e.ID) == nil {
				if err := s.g.EdgeAdded(e); err != nil {
					s.logger.Errorf("%s, %+v", err, e)
				}
			}
		}
	case messages.NodeUpdatedMsgType:
		err = s.g.NodeUpdated(obj.(*graph.Node))
	case messages.NodeDeletedMsgType:
		err = s.g.NodeDeleted(obj.(*graph.Node))
	case messages.NodeAddedMsgType:
		err = s.g.NodeAdded(obj.(*graph.Node))
	case messages.EdgeUpdatedMsgType:
		err = s.g.EdgeUpdated(obj.(*graph.Edge))
	case messages.EdgeDeletedMsgType:
		if err = s.g.EdgeDeleted(obj.(*graph.Edge)); err == graph.ErrEdgeNotFound {
			return
		}
	case messages.EdgeAddedMsgType:
		err = s.g.EdgeAdded(obj.(*graph.Edge))
	}

	if err != nil {
		s.logger.Errorf("%s, %+v", err, msg)
	}
}

func NewSubscriber(client *websocket.Client, g *graph.Graph, logger logging.Logger) *Subscriber {
	structSpeaker := client.UpgradeToStructSpeaker()
	subscriber := &Subscriber{
		StructSpeaker: structSpeaker,
		g:             g,
		logger:        logger,
	}
	structSpeaker.AddEventHandler(subscriber)
	subscriber.AddStructMessageHandler(subscriber, []string{messages.Namespace})
	return subscriber
}
