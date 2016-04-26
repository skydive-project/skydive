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

package mappings

import (
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/flow/packet"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
)

type OvsFlowEnhancer struct {
	Graph *graph.Graph
}

func (gfe *OvsFlowEnhancer) getPath(mac string) string {
	if packet.IsBroadcastMac(mac) || packet.IsMulticastMac(mac) {
		return "*"
	}

	gfe.Graph.Lock()
	defer gfe.Graph.Unlock()

	intfs := gfe.Graph.LookupNodes(graph.Metadata{"ExtID.attached-mac": mac})
	if len(intfs) > 1 {
		logging.GetLogger().Infof("OvsFlowEnhancer found more than one interface for the mac: %s", mac)
	} else if len(intfs) == 1 {
		nodes := gfe.Graph.LookupShortestPath(intfs[0], graph.Metadata{"Type": "host"}, topology.IsOwnershipEdge)
		if len(nodes) > 0 {
			return topology.NodePath{Nodes: nodes}.Marshal()
		}
	}
	return ""
}

func (gfe *OvsFlowEnhancer) Enhance(f *flow.Flow) {
	var eth *flow.FlowEndpointsStatistics
	if f.IfSrcGraphPath == "" || f.IfDstGraphPath == "" {
		eth = f.GetStatistics().GetEndpointsType(flow.FlowEndpointType_ETHERNET)
		if eth == nil {
			return
		}
	}
	if f.IfSrcGraphPath == "" {
		f.IfSrcGraphPath = gfe.getPath(eth.AB.Value)
	}
	if f.IfDstGraphPath == "" {
		f.IfDstGraphPath = gfe.getPath(eth.BA.Value)
	}
}

func NewOvsFlowEnhancer(g *graph.Graph) *OvsFlowEnhancer {
	return &OvsFlowEnhancer{
		Graph: g,
	}
}
