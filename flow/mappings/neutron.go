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
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/packet"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

type NeutronFlowEnhancer struct {
	Graph *graph.Graph
}

func (nfe *NeutronFlowEnhancer) getNodeTID(mac string) string {
	if packet.IsBroadcastMac(mac) || packet.IsMulticastMac(mac) {
		return "*"
	}

	nfe.Graph.RLock()
	defer nfe.Graph.RUnlock()

	// use the PeerIntfMAC metadata provided by the neutron probe. The interface used is the
	// one attached to the VM interface.
	intfs := nfe.Graph.GetNodes(graph.Metadata{"PeerIntfMAC": mac, "Manager": "neutron"})
	if len(intfs) > 1 {
		logging.GetLogger().Infof("NeutronFlowEnhancer found more than one interface with the PeerIntfMAC: %s", mac)
	} else if len(intfs) == 1 {
		if t, ok := intfs[0].Metadata()["TID"]; ok {
			return t.(string)
		}
	}
	return ""
}

func (nfe *NeutronFlowEnhancer) Enhance(f *flow.Flow) {
	if f.ANodeTID == "" || f.BNodeTID == "" {
		if f.Link == nil {
			return
		}
	}
	if f.ANodeTID == "" {
		f.ANodeTID = nfe.getNodeTID(f.Link.A)
	}
	if f.BNodeTID == "" {
		f.BNodeTID = nfe.getNodeTID(f.Link.B)
	}
}

func NewNeutronFlowEnhancer(g *graph.Graph) *NeutronFlowEnhancer {
	return &NeutronFlowEnhancer{
		Graph: g,
	}
}
