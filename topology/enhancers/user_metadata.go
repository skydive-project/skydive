/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package metadata

import (
	"sync"

	"github.com/skydive-project/skydive/api/server"
	api "github.com/skydive-project/skydive/api/types"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

//UserMetadataManager describes user metadata manager
type UserMetadataManager struct {
	sync.RWMutex
	graph.DefaultGraphListener
	graph           *graph.Graph
	metadataHandler *server.UserMetadataAPIHandler
	metadata        map[string]*api.UserMetadata
	watcher         server.StoppableWatcher
}

//OnNodeAdded event
func (u *UserMetadataManager) OnNodeAdded(n *graph.Node) {
	u.nodeEvent()
}

//OnNodeUpdated event
func (u *UserMetadataManager) OnNodeUpdated(n *graph.Node) {
	u.nodeEvent()
}

func (u *UserMetadataManager) nodeEvent() {
	for _, m := range u.metadata {
		u.addUserMetadata(m)
	}
}

func (u *UserMetadataManager) applyGremlinExpr(query string) []interface{} {
	res, err := ge.TopologyGremlinQuery(u.graph, query)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin error: %s", err.Error())
		return nil
	}
	return res.Values()
}

func (u *UserMetadataManager) addUserMetadata(umd *api.UserMetadata) {
	u.Lock()
	u.metadata[umd.UUID] = umd
	u.Unlock()

	nodes := u.applyGremlinExpr(umd.GremlinQuery)
	for _, node := range nodes {
		switch node.(type) {
		case *graph.Node:
			u.graph.AddMetadata(node, "UserMetadata."+umd.Key, umd.Value)
		default:
			continue
		}
	}
}

func (u *UserMetadataManager) deleteUserMetadata(umd *api.UserMetadata) {
	u.Lock()
	delete(u.metadata, umd.UUID)
	u.Unlock()

	nodes := u.applyGremlinExpr(umd.GremlinQuery)
	for _, node := range nodes {
		switch node := node.(type) {
		case *graph.Node:
			u.graph.DelMetadata(node, "UserMetadata."+umd.Key)
		default:
			continue
		}
	}
}

func (u *UserMetadataManager) onAPIWatcherEvent(action string, id string, resource api.Resource) {
	metadata := resource.(*api.UserMetadata)
	switch action {
	case "init", "create", "set", "update":
		u.graph.Lock()
		u.addUserMetadata(metadata)
		u.graph.Unlock()
	case "delete":
		u.graph.Lock()
		u.deleteUserMetadata(metadata)
		u.graph.Unlock()

	}
}

//Start starts user metadata manager
func (u *UserMetadataManager) Start() {
	u.watcher = u.metadataHandler.AsyncWatch(u.onAPIWatcherEvent)
	u.graph.AddEventListener(u)
}

//Stop stops user metadata manager
func (u *UserMetadataManager) Stop() {
	u.watcher.Stop()
	u.graph.RemoveEventListener(u)
}

//NewUserMetadataManager creates a new user metadata manager
func NewUserMetadataManager(g *graph.Graph, u *server.UserMetadataAPIHandler) *UserMetadataManager {
	resources := u.Index()
	metadata := make(map[string]*api.UserMetadata)
	for _, resource := range resources {
		metadata[resource.ID()] = resource.(*api.UserMetadata)
	}

	umm := &UserMetadataManager{
		graph:           g,
		metadataHandler: u,
		metadata:        metadata,
	}
	return umm
}
