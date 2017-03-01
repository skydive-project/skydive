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
	"github.com/skydive-project/skydive/topology/graph"
)

type GraphFlowEnhancer struct {
	Graph    *graph.Graph
	tidCache *tidCache
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
		gfe.tidCache.set(ce, mac, tid)
	}

	return tid
}

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

func NewGraphFlowEnhancer(g *graph.Graph, cache *cache.Cache) *GraphFlowEnhancer {
	fe := &GraphFlowEnhancer{
		Graph: g,
	}
	if cache != nil {
		fe.tidCache = &tidCache{cache}
	}

	return fe
}
