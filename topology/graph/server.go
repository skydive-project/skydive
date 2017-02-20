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

const (
	Namespace = "Graph"
)

type GraphServer struct {
	shttp.DefaultWSServerEventHandler
	WSServer *shttp.WSServer
	Graph    *Graph
}

func (s *GraphServer) OnMessage(c *shttp.WSClient, msg shttp.WSMessage) {
	if msg.Namespace != Namespace {
		return
	}

	s.Graph.Lock()
	defer s.Graph.Unlock()

	msgType, obj, err := UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

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
}

func (s *GraphServer) OnNodeUpdated(n *Node) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, NodeUpdatedMsgType, n))
}

func (s *GraphServer) OnNodeAdded(n *Node) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, NodeAddedMsgType, n))
}

func (s *GraphServer) OnNodeDeleted(n *Node) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, NodeDeletedMsgType, n))
}

func (s *GraphServer) OnEdgeUpdated(e *Edge) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, EdgeUpdatedMsgType, e))
}

func (s *GraphServer) OnEdgeAdded(e *Edge) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, EdgeAddedMsgType, e))
}

func (s *GraphServer) OnEdgeDeleted(e *Edge) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, EdgeDeletedMsgType, e))
}

func NewServer(g *Graph, server *shttp.WSServer) *GraphServer {
	s := &GraphServer{
		Graph:    g,
		WSServer: server,
	}
	s.Graph.AddEventListener(s)
	server.AddEventHandler(s)

	return s
}
