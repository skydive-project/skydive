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
	shttp "github.com/redhat-cip/skydive/http"
	"github.com/redhat-cip/skydive/logging"
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

	msg, err := UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %s: %s", msg, err.Error())
		return
	}

	switch msg.Type {
	case "SyncRequest":
		reply := shttp.WSMessage{
			Namespace: Namespace,
			Type:      "SyncReply",
			Obj:       s.Graph,
		}

		c.SendWSMessage(reply)

	case "SubGraphDeleted":
		n := msg.Obj.(*Node)

		logging.GetLogger().Debugf("Got SubGraphDeleted event from the node %s", n.ID)

		node := s.Graph.GetNode(n.ID)
		if node != nil {
			s.Graph.DelSubGraph(node)
		}
	case "NodeUpdated":
		n := msg.Obj.(*Node)
		node := s.Graph.GetNode(n.ID)
		if node != nil {
			s.Graph.SetMetadata(node, n.metadata)
		}
	case "NodeDeleted":
		s.Graph.DelNode(msg.Obj.(*Node))
	case "NodeAdded":
		n := msg.Obj.(*Node)
		if s.Graph.GetNode(n.ID) == nil {
			s.Graph.AddNode(n)
		}
	case "EdgeUpdated":
		e := msg.Obj.(*Edge)
		edge := s.Graph.GetEdge(e.ID)
		if edge != nil {
			s.Graph.SetMetadata(edge, e.metadata)
		}
	case "EdgeDeleted":
		s.Graph.DelEdge(msg.Obj.(*Edge))
	case "EdgeAdded":
		e := msg.Obj.(*Edge)
		if s.Graph.GetEdge(e.ID) == nil {
			s.Graph.AddEdge(e)
		}
	}
}

func (s *GraphServer) OnNodeUpdated(n *Node) {
	s.WSServer.BroadcastWSMessage(shttp.WSMessage{
		Namespace: Namespace,
		Type:      "NodeUpdated",
		Obj:       n,
	})
}

func (s *GraphServer) OnNodeAdded(n *Node) {
	s.WSServer.BroadcastWSMessage(shttp.WSMessage{
		Namespace: Namespace,
		Type:      "NodeAdded",
		Obj:       n,
	})
}

func (s *GraphServer) OnNodeDeleted(n *Node) {
	s.WSServer.BroadcastWSMessage(shttp.WSMessage{
		Namespace: Namespace,
		Type:      "NodeDeleted",
		Obj:       n,
	})
}

func (s *GraphServer) OnEdgeUpdated(e *Edge) {
	s.WSServer.BroadcastWSMessage(shttp.WSMessage{
		Namespace: Namespace,
		Type:      "EdgeUpdated",
		Obj:       e,
	})
}

func (s *GraphServer) OnEdgeAdded(e *Edge) {
	s.WSServer.BroadcastWSMessage(shttp.WSMessage{
		Namespace: Namespace,
		Type:      "EdgeAdded",
		Obj:       e,
	})
}

func (s *GraphServer) OnEdgeDeleted(e *Edge) {
	s.WSServer.BroadcastWSMessage(shttp.WSMessage{
		Namespace: Namespace,
		Type:      "EdgeDeleted",
		Obj:       e,
	})
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
