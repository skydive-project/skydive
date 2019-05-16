/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package client

import (
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/flow/probes"

	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/gremlin"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/ondemand/client"
	ws "github.com/skydive-project/skydive/websocket"
)

type onDemandFlowHandler struct {
	graph         *graph.Graph
	nodeTypeQuery string
}

func (h *onDemandFlowHandler) DecodeMessage(msg json.RawMessage) (types.Resource, error) {
	var capture types.Capture
	if err := json.Unmarshal(msg, &capture); err != nil {
		return nil, fmt.Errorf("Unable to decode capture: %s", err)
	}
	return &capture, nil
}

func (h *onDemandFlowHandler) EncodeMessage(nodeID graph.Identifier, resource types.Resource) (json.RawMessage, error) {
	bytes, err := json.Marshal(resource)
	return json.RawMessage(bytes), err
}

func (h *onDemandFlowHandler) CheckState(node *graph.Node, resource types.Resource) bool {
	capture := resource.(*types.Capture)
	if captures, err := node.GetField("Captures"); err == nil {
		for _, c := range *captures.(*probes.Captures) {
			if c.ID == capture.UUID && c.State == "active" {
				return true
			}
		}
	}
	return false
}

func (h *onDemandFlowHandler) ResourceName() string {
	return "Capture"
}

func (h *onDemandFlowHandler) GetNodes(resource types.Resource) []interface{} {
	capture := resource.(*types.Capture)
	query := capture.GremlinQuery
	query += fmt.Sprintf(".Dedup().Has('Captures.ID', NEE('%s'))", resource.ID())
	if capture.Type != "" && !common.CheckProbeCapabilities(capture.Type, common.MultipleOnSameNodeCapability) {
		query += fmt.Sprintf(".Has('Captures.Type', NEE('%s'))", capture.Type)
	}
	return h.applyGremlinExpr(query)
}

func (h *onDemandFlowHandler) applyGremlinExpr(query string) []interface{} {
	res, err := ge.TopologyGremlinQuery(h.graph, query)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin %s error: %s", query, err)
		return nil
	}
	return res.Values()
}

// NewOnDemandFlowProbeClient creates a new ondemand probe client based on API, graph and websocket
func NewOnDemandFlowProbeClient(g *graph.Graph, ch api.Handler, agentPool ws.StructSpeakerPool, subscriberPool ws.StructSpeakerPool, etcdClient *etcd.Client) *client.OnDemandClient {
	nodeTypes := make([]interface{}, len(common.CaptureTypes))
	i := 0
	for nodeType := range common.CaptureTypes {
		nodeTypes[i] = nodeType
		i++
	}
	nodeTypeQuery := new(gremlin.QueryString).Has("Type", gremlin.Within(nodeTypes...)).String()
	return client.NewOnDemandClient(g, ch, agentPool, subscriberPool, etcdClient, &onDemandFlowHandler{graph: g, nodeTypeQuery: nodeTypeQuery})
}
