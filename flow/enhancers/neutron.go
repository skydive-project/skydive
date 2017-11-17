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

package enhancers

import (
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/packet"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// NeutronFlowEnhancer describes a neutron graph enhancer
type NeutronFlowEnhancer struct {
	graph.DefaultGraphListener
	Graph    *graph.Graph
	tidCache *graph.MetadataIndexer
}

// Name return the Neutron enahancer name
func (nfe *NeutronFlowEnhancer) Name() string {
	return "Neutron"
}

func (nfe *NeutronFlowEnhancer) getNodeTID(mac string) (tid string) {
	if packet.IsBroadcastMac(mac) || packet.IsMulticastMac(mac) {
		return "*"
	}

	nfe.Graph.RLock()
	defer nfe.Graph.RUnlock()

	nodes, _ := nfe.tidCache.Get(mac)
	if intfs := len(nodes); intfs > 1 {
		logging.GetLogger().Infof("NeutronFlowEnhancer found more than one interface with the PeerIntfMAC: %s", mac)
	} else if intfs == 1 {
		tid, _ = nodes[0].GetFieldString("TID")
	}

	return
}

// Enhance A and B node TID based on neutron database
func (nfe *NeutronFlowEnhancer) Enhance(f *flow.Flow) {
	if f.Link == nil {
		return
	}
	if f.ANodeTID == "" {
		f.ANodeTID = nfe.getNodeTID(f.Link.A)
	}
	if f.BNodeTID == "" {
		f.BNodeTID = nfe.getNodeTID(f.Link.B)
	}
}

// Start the neutron flow enhancer
func (nfe *NeutronFlowEnhancer) Start() error {
	nfe.tidCache.Start()
	return nil
}

// Stop the neutron flow enhancer
func (nfe *NeutronFlowEnhancer) Stop() {
	nfe.tidCache.Stop()
}

// NewNeutronFlowEnhancer creates a new flow enhancer that will enhance A and B flow nodes TIDs
func NewNeutronFlowEnhancer(g *graph.Graph) *NeutronFlowEnhancer {
	nodeFilter := graph.NewGraphElementFilter(filters.NewAndFilter(
		filters.NewNotNullFilter("TID"),
		filters.NewNotNullFilter("PeerIntfMAC"),
		filters.NewTermStringFilter("Manager", "neutron"),
	))

	fe := &NeutronFlowEnhancer{
		Graph:    g,
		tidCache: graph.NewMetadataIndexer(g, nodeFilter, "PeerIntfMAC"),
	}
	g.AddEventListener(fe)

	return fe
}
