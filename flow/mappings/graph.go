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

package mappings

import (
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/packet"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

type GraphFlowEnhancer struct {
	Graph *graph.Graph
}

func (gfe *GraphFlowEnhancer) getNodeTID(mac string) string {
	if packet.IsBroadcastMac(mac) || packet.IsMulticastMac(mac) {
		return "*"
	}

	gfe.Graph.RLock()
	defer gfe.Graph.RUnlock()

	intfs := gfe.Graph.GetNodes(graph.Metadata{"MAC": mac})
	if len(intfs) > 1 {
		logging.GetLogger().Infof("GraphFlowEnhancer found more than one interface for the mac: %s", mac)
	} else if len(intfs) == 1 {
		if t, ok := intfs[0].Metadata()["TID"]; ok {
			return t.(string)
		}
	}
	return ""
}

func (gfe *GraphFlowEnhancer) Enhance(f *flow.Flow) {
	if f.ANodeTID == "" || f.BNodeTID == "" {
		if f.Link == nil {
			return
		}
	}
	if f.ANodeTID == "" {
		f.ANodeTID = gfe.getNodeTID(f.Link.A)
	}
	if f.BNodeTID == "" {
		f.BNodeTID = gfe.getNodeTID(f.Link.B)
	}
}

func NewGraphFlowEnhancer(g *graph.Graph) *GraphFlowEnhancer {
	return &GraphFlowEnhancer{
		Graph: g,
	}
}
