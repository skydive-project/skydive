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

package enhancers

import (
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/flow/packet"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// GraphFlowEnhancer describes an cache node enhancer
type GraphFlowEnhancer struct {
	graph.DefaultGraphListener
	Graph    *graph.Graph
	tidCache *graph.MetadataIndexer
}

// Name return the Graph enahancer name
func (gfe *GraphFlowEnhancer) Name() string {
	return "Graph"
}

func (gfe *GraphFlowEnhancer) getNodeTID(mac string) (tid string) {
	if packet.IsBroadcastMac(mac) || packet.IsMulticastMac(mac) {
		return "*"
	}

	gfe.Graph.RLock()
	defer gfe.Graph.RUnlock()

	nodes, _ := gfe.tidCache.Get(mac)
	if intfs := len(nodes); intfs > 1 {
		logging.GetLogger().Infof("GraphFlowEnhancer found more than one interface with the MAC: %s", mac)
	} else if intfs == 1 {
		tid, _ = nodes[0].GetFieldString("TID")
	}

	return
}

// Start the graph flow enhancer
func (gfe *GraphFlowEnhancer) Start() error {
	gfe.tidCache.Start()
	return nil
}

// Stop the graph flow enhancer
func (gfe *GraphFlowEnhancer) Stop() {
	gfe.tidCache.Stop()
}

// NewGraphFlowEnhancer creates a new flow enhancer that will enhance A and B flow nodes TIDs
func NewGraphFlowEnhancer(g *graph.Graph) *GraphFlowEnhancer {
	nodeFilter := graph.NewGraphElementFilter(filters.NewAndFilter(
		filters.NewNotNullFilter("TID"),
		filters.NewNotNullFilter("MAC"),
	))

	fe := &GraphFlowEnhancer{
		Graph:    g,
		tidCache: graph.NewMetadataIndexer(g, nodeFilter, "MAC"),
	}
	g.AddEventListener(fe)
	return fe
}
