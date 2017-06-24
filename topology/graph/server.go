/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"net/http"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

// Namespace websocket message : Graph
const (
	Namespace = "Graph"
)

// ServerEventHandler interface event
type ServerEventHandler interface {
	OnGraphMessage(c *shttp.WSClient, m shttp.WSMessage, msgType string, obj interface{})
}

// Server describes a graph server based on websocket
type Server struct {
	shttp.DefaultWSServerEventHandler
	WSServer      *shttp.WSServer
	Graph         *Graph
	eventHandlers []ServerEventHandler
}

// OnMessage event
func (s *Server) OnMessage(c *shttp.WSClient, msg shttp.WSMessage) {
	msgType, obj, err := UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

	s.Graph.RLock()
	switch msgType {
	case SyncRequestMsgType:
		status := http.StatusOK
		graph, err := s.Graph.WithContext(obj.(GraphContext))
		if err != nil {
			logging.GetLogger().Errorf("Graph: unable to get a graph with context %+v: %s", obj.(GraphContext), err.Error())
			graph, status = nil, http.StatusBadRequest
		}
		reply := msg.Reply(graph, SyncReplyMsgType, status)
		c.SendWSMessage(reply)
	}
	s.Graph.RUnlock()

	for _, h := range s.eventHandlers {
		h.OnGraphMessage(c, msg, msgType, obj)
	}
}

// OnNodeUpdated event
func (s *Server) OnNodeUpdated(n *Node) {
	s.WSServer.QueueBroadcastWSMessage(shttp.NewWSMessage(Namespace, NodeUpdatedMsgType, n))
}

// OnNodeAdded event
func (s *Server) OnNodeAdded(n *Node) {
	s.WSServer.QueueBroadcastWSMessage(shttp.NewWSMessage(Namespace, NodeAddedMsgType, n))
}

// OnNodeDeleted event
func (s *Server) OnNodeDeleted(n *Node) {
	s.WSServer.QueueBroadcastWSMessage(shttp.NewWSMessage(Namespace, NodeDeletedMsgType, n))
}

// OnEdgeUpdated event
func (s *Server) OnEdgeUpdated(e *Edge) {
	s.WSServer.QueueBroadcastWSMessage(shttp.NewWSMessage(Namespace, EdgeUpdatedMsgType, e))
}

// OnEdgeAdded event
func (s *Server) OnEdgeAdded(e *Edge) {
	s.WSServer.QueueBroadcastWSMessage(shttp.NewWSMessage(Namespace, EdgeAddedMsgType, e))
}

// OnEdgeDeleted event
func (s *Server) OnEdgeDeleted(e *Edge) {
	s.WSServer.QueueBroadcastWSMessage(shttp.NewWSMessage(Namespace, EdgeDeletedMsgType, e))
}

// AddEventHandler subscribe a new graph server event handler
func (s *Server) AddEventHandler(h ServerEventHandler) {
	s.eventHandlers = append(s.eventHandlers, h)
}

// NewServer creates a new graph server based on a websocket server
func NewServer(g *Graph, server *shttp.WSServer) *Server {
	s := &Server{
		Graph:    g,
		WSServer: server,
	}

	s.Graph.AddEventListener(s)
	server.AddEventHandler(s, []string{Namespace})

	return s
}
