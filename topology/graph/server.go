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
	"bytes"
	"encoding/json"
	"net/http"
	"time"

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

func (s *GraphServer) OnUnregisterClient(c *shttp.WSClient) {
	s.Graph.Lock()
	defer s.Graph.Unlock()
	if host, kind := c.GetHostInfo(); kind == "skydive-agent" {
		s.Graph.DelHostGraph(host)
	}
}

func UnmarshalWSMessage(msg shttp.WSMessage) (string, interface{}, error) {
	if msg.Type == "SyncRequest" {
		var obj map[string]interface{}
		if msg.Obj != nil {
			decoder := json.NewDecoder(bytes.NewReader([]byte(*msg.Obj)))
			decoder.UseNumber()

			if err := decoder.Decode(&obj); err != nil {
				return "", msg, err
			}
		}

		var context GraphContext
		switch v := obj["Time"].(type) {
		case json.Number:
			i, err := v.Int64()
			if err != nil {
				return "", msg, err
			}
			unix := time.Unix(i/1000, 0).UTC()
			context.Time = &unix
		}

		return msg.Type, context, nil
	}

	switch msg.Type {
	case "HostGraphDeleted":
		var obj interface{}
		if err := json.Unmarshal([]byte(*msg.Obj), &obj); err != nil {
			return "", msg, err
		}
		return msg.Type, obj, nil
	case "NodeUpdated", "NodeDeleted", "NodeAdded":
		var obj interface{}
		if err := json.Unmarshal([]byte(*msg.Obj), &obj); err != nil {
			return "", msg, err
		}

		var node Node
		if err := node.Decode(obj); err != nil {
			return "", msg, err
		}

		return msg.Type, &node, nil
	case "EdgeUpdated", "EdgeDeleted", "EdgeAdded":
		var obj interface{}
		err := json.Unmarshal([]byte(*msg.Obj), &obj)
		if err != nil {
			return "", msg, err
		}

		var edge Edge
		if err := edge.Decode(obj); err != nil {
			return "", msg, err
		}

		return msg.Type, &edge, nil
	}

	return "", msg, nil
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
	case "SyncRequest":
		status := http.StatusOK
		graph, err := s.Graph.WithContext(obj.(GraphContext))
		if err != nil {
			logging.GetLogger().Errorf("Graph: unable to get a graph with context %+v: %s", obj.(GraphContext), err.Error())
			graph, status = nil, http.StatusBadRequest
		}
		reply := msg.Reply(graph, "SyncReply", status)
		c.SendWSMessage(reply)
	case "HostGraphDeleted":
		host := obj.(string)

		logging.GetLogger().Debugf("Got HostGraphDeleted event for host %s", host)
		s.Graph.DelHostGraph(host)
	case "NodeUpdated":
		n := obj.(*Node)
		node := s.Graph.GetNode(n.ID)
		if node != nil {
			s.Graph.SetMetadata(node, n.metadata)
		}
	case "NodeDeleted":
		s.Graph.DelNode(obj.(*Node))
	case "NodeAdded":
		n := obj.(*Node)
		if s.Graph.GetNode(n.ID) == nil {
			s.Graph.AddNode(n)
		}
	case "EdgeUpdated":
		e := obj.(*Edge)
		edge := s.Graph.GetEdge(e.ID)
		if edge != nil {
			s.Graph.SetMetadata(edge, e.metadata)
		}
	case "EdgeDeleted":
		s.Graph.DelEdge(obj.(*Edge))
	case "EdgeAdded":
		e := obj.(*Edge)
		if s.Graph.GetEdge(e.ID) == nil {
			s.Graph.AddEdge(e)
		}
	}
}

func (s *GraphServer) OnNodeUpdated(n *Node) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, "NodeUpdated", n))
}

func (s *GraphServer) OnNodeAdded(n *Node) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, "NodeAdded", n))
}

func (s *GraphServer) OnNodeDeleted(n *Node) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, "NodeDeleted", n))
}

func (s *GraphServer) OnEdgeUpdated(e *Edge) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, "EdgeUpdated", e))
}

func (s *GraphServer) OnEdgeAdded(e *Edge) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, "EdgeAdded", e))
}

func (s *GraphServer) OnEdgeDeleted(e *Edge) {
	s.WSServer.BroadcastWSMessage(shttp.NewWSMessage(Namespace, "EdgeDeleted", e))
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
