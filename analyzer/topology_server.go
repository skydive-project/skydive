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

package analyzer

import (
	"sync"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

type TopologyServer struct {
	sync.RWMutex
	shttp.DefaultWSServerEventHandler
	Graph       *graph.Graph
	GraphServer *graph.GraphServer
	cached      *graph.CachedBackend
	// map used to store agent which uses this analyzer as master
	// basically sending graph messages
	authors map[string]bool
}

func (t *TopologyServer) hostGraphDeleted(host string, mode int) {
	t.cached.SetMode(mode)
	defer t.cached.SetMode(graph.DEFAULT_MODE)

	t.Graph.DelHostGraph(host)
}

func (t *TopologyServer) OnUnregisterClient(c *shttp.WSClient) {
	if c.ClientType != common.AnalyzerService {
		t.hostGraphDeleted(c.Host, graph.CACHE_ONLY_MODE)

		t.RLock()
		_, ok := t.authors[c.Host]
		t.RUnlock()

		// it's an authors so already received a message meaning that the client chose this analyzer as master
		if ok {
			logging.GetLogger().Debugf("Authoritative client unregistered, delete resources %s", c.Host)
			t.hostGraphDeleted(c.Host, graph.PERSISTENT_ONLY_MODE)

			t.Lock()
			delete(t.authors, c.Host)
			t.Unlock()
		}
	}
}

func (t *TopologyServer) OnMessage(c *shttp.WSClient, msg shttp.WSMessage) {
	if msg.Namespace != graph.Namespace {
		return
	}

	t.Graph.Lock()
	defer t.Graph.Unlock()

	msgType, obj, err := graph.UnmarshalWSMessage(msg)
	if err != nil {
		logging.GetLogger().Errorf("Graph: Unable to parse the event %v: %s", msg, err.Error())
		return
	}

	if c.ClientType != common.AnalyzerService {
		t.Lock()
		t.authors[c.Host] = true
		t.Unlock()
	}

	// got HostGraphDeleted, so if not an analyzer we need to do two things:
	// force the deletion from the cache and force the delete from the persistent
	// backend. We need to use the persistent only to be use to retrieve nodes/edges
	// from the persistent backend otherwise the memory backend would be used.
	if msgType == "HostGraphDeleted" {
		host := obj.(string)

		logging.GetLogger().Debugf("Got HostGraphDeleted event for host %s", host)

		t.hostGraphDeleted(obj.(string), graph.CACHE_ONLY_MODE)
		if c.ClientType != common.AnalyzerService {
			t.hostGraphDeleted(obj.(string), graph.PERSISTENT_ONLY_MODE)
		}
	}

	// If the message comes from analyzer we need to apply it only on cache only
	// as it is a forwarded message.
	if c.ClientType == common.AnalyzerService {
		t.cached.SetMode(graph.CACHE_ONLY_MODE)
	}
	defer t.cached.SetMode(graph.DEFAULT_MODE)

	switch msgType {
	case "NodeUpdated":
		n := obj.(*graph.Node)
		if node := t.Graph.GetNode(n.ID); node != nil {
			t.Graph.SetMetadata(node, n.Metadata())
		}
	case "NodeDeleted":
		t.Graph.DelNode(obj.(*graph.Node))
	case "NodeAdded":
		n := obj.(*graph.Node)
		if t.Graph.GetNode(n.ID) == nil {
			t.Graph.AddNode(n)
		}
	case "EdgeUpdated":
		e := obj.(*graph.Edge)
		if edge := t.Graph.GetEdge(e.ID); edge != nil {
			t.Graph.SetMetadata(edge, e.Metadata())
		}
	case "EdgeDeleted":
		t.Graph.DelEdge(obj.(*graph.Edge))
	case "EdgeAdded":
		e := obj.(*graph.Edge)
		if t.Graph.GetEdge(e.ID) == nil {
			t.Graph.AddEdge(e)
		}
	}
}

func NewTopologyServer(host string, server *shttp.WSServer) *TopologyServer {
	persistent, err := graph.BackendFromConfig()
	if err != nil {
		logging.GetLogger().Error(err.Error())
		return nil
	}

	cached, err := graph.NewCachedBackend(persistent)
	if err != nil {
		logging.GetLogger().Error(err.Error())
		return nil
	}

	g := graph.NewGraphFromConfig(cached)

	t := &TopologyServer{
		Graph:       g,
		GraphServer: graph.NewServer(g, server),
		cached:      cached,
		authors:     make(map[string]bool),
	}
	server.AddEventHandler(t)

	return t
}

func NewTopologyServerFromConfig(server *shttp.WSServer) *TopologyServer {
	host := config.GetConfig().GetString("host_id")
	return NewTopologyServer(host, server)
}
