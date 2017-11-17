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
	"github.com/pmylund/go-cache"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/packet"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

// GraphFlowEnhancer describes an cache node enhancer
type GraphFlowEnhancer struct {
	graph.DefaultGraphListener
	Graph    *graph.Graph
	tidCache *tidCache
}

// Name return the Graph enahancer name
func (gfe *GraphFlowEnhancer) Name() string {
	return "Graph"
}

func (gfe *GraphFlowEnhancer) getNodeTID(mac string) string {
	if packet.IsBroadcastMac(mac) || packet.IsMulticastMac(mac) {
		return "*"
	}

	var ce *tidCacheEntry

	if gfe.tidCache != nil {
		var f bool
		if ce, f = gfe.tidCache.get(mac); f {
			return ce.tid
		}
	}

	gfe.Graph.RLock()
	defer gfe.Graph.RUnlock()

	var tid string

	intfs := gfe.Graph.GetNodes(graph.Metadata{"MAC": mac})
	if len(intfs) > 1 {
		logging.GetLogger().Infof("GraphFlowEnhancer found more than one interface for the mac: %s", mac)
	} else if len(intfs) == 1 {
		tid, _ = intfs[0].GetFieldString("TID")
	}

	if gfe.tidCache != nil {
		logging.GetLogger().Debugf("GraphFlowEnhancer set cache %s: %s", mac, tid)
		gfe.tidCache.set(ce, mac, tid)
	}

	return tid
}

// Enhance the graph with local TID node cache
func (gfe *GraphFlowEnhancer) Enhance(f *flow.Flow) {
	if f.Link == nil {
		return
	}
	if f.ANodeTID == "" {
		f.ANodeTID = gfe.getNodeTID(f.Link.A)
	}
	if f.BNodeTID == "" {
		f.BNodeTID = gfe.getNodeTID(f.Link.B)
	}
}

// OnNodeDeleted event
func (gfe *GraphFlowEnhancer) OnNodeDeleted(n *graph.Node) {
	if mac, _ := n.GetFieldString("MAC"); mac != "" {
		logging.GetLogger().Debugf("GraphFlowEnhancer node event del cache %s", n.String())
		gfe.tidCache.del(mac)
	}
}

// OnEdgeDeleted event
func (gfe *GraphFlowEnhancer) OnEdgeDeleted(e *graph.Edge) {
	// need to reset the entry as edge event of type RelationType means TID update
	if rt, _ := e.GetFieldString("RelationType"); rt == topology.OwnershipLink {
		_, children := gfe.Graph.GetEdgeNodes(e, nil, nil)
		for _, child := range children {
			if mac, _ := child.GetFieldString("MAC"); mac != "" {
				logging.GetLogger().Debugf("GraphFlowEnhancer edge event del cache %s", child.String())
				gfe.tidCache.del(mac)
			}
		}
	}
}

// Start the graph flow enhancer
func (gfe *GraphFlowEnhancer) Start() error {
	return nil
}

// Stop the graph flow enhancer
func (gfe *GraphFlowEnhancer) Stop() {
}

// NewGraphFlowEnhancer creates a new flow enhancer that will enhance A and B flow nodes TIDs
func NewGraphFlowEnhancer(g *graph.Graph, cache *cache.Cache) *GraphFlowEnhancer {
	fe := &GraphFlowEnhancer{
		Graph: g,
	}
	g.AddEventListener(fe)

	if cache != nil {
		fe.tidCache = &tidCache{cache}
	}

	return fe
}
