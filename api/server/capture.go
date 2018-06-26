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

package server

import (
	"fmt"

	uuid "github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// CaptureResourceHandler describes a capture ressouce handler
type CaptureResourceHandler struct {
	ResourceHandler
}

// CaptureAPIHandler based on BasicAPIHandler
type CaptureAPIHandler struct {
	BasicAPIHandler
	Graph *graph.Graph
}

// Name returns "capture"
func (c *CaptureResourceHandler) Name() string {
	return "capture"
}

// New creates a new capture resource
func (c *CaptureResourceHandler) New() types.Resource {
	id, _ := uuid.NewV4()

	return &types.Capture{
		UUID:         id.String(),
		LayerKeyMode: flow.DefaultLayerKeyModeName(),
	}
}

// Decorate populates the capture resource
func (c *CaptureAPIHandler) Decorate(resource types.Resource) {
	capture := resource.(*types.Capture)

	count := 0
	pcapSocket := ""

	c.Graph.RLock()
	defer c.Graph.RUnlock()

	res, err := ge.TopologyGremlinQuery(c.Graph, capture.GremlinQuery)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin error: %s", err.Error())
		return
	}

	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			n := value.(*graph.Node)
			if cuuid, _ := n.GetFieldString("Capture.ID"); cuuid != "" {
				count++
			}
			if p, _ := n.GetFieldString("Capture.PCAPSocket"); p != "" {
				pcapSocket = p
			}
		case []*graph.Node:
			for _, n := range value.([]*graph.Node) {
				if cuuid, _ := n.GetFieldString("Capture.ID"); cuuid != "" {
					count++
				}
				if p, _ := n.GetFieldString("Capture.PCAPSocket"); p != "" {
					pcapSocket = p
				}
			}
		default:
			count = 0
		}
	}

	capture.Count = count
	capture.PCAPSocket = pcapSocket
}

// Create tests that resource GremlinQuery does not exists already
func (c *CaptureAPIHandler) Create(r types.Resource) error {
	capture := r.(*types.Capture)

	// check capabilities
	if capture.Type != "" {
		if capture.BPFFilter != "" {
			if !common.CheckProbeCapabilities(capture.Type, common.BPFCapability) {
				return fmt.Errorf("%s capture doesn't support BPF filtering", capture.Type)
			}
		}
		if capture.RawPacketLimit != 0 {
			if !common.CheckProbeCapabilities(capture.Type, common.RawPacketsCapability) {
				return fmt.Errorf("%s capture doesn't support raw packet capture", capture.Type)
			}
		}
		if capture.ExtraTCPMetric {
			if !common.CheckProbeCapabilities(capture.Type, common.ExtraTCPMetricCapability) {
				return fmt.Errorf("%s capture doesn't support extra TCP metrics capture", capture.Type)
			}
		}
	}

	resources := c.BasicAPIHandler.Index()
	for _, resource := range resources {
		resource := resource.(*types.Capture)
		if resource.GremlinQuery == capture.GremlinQuery {
			return fmt.Errorf("Duplicate capture, uuid=%s", capture.UUID)
		}
	}

	return c.BasicAPIHandler.Create(r)
}

// RegisterCaptureAPI registers an new resource, capture
func RegisterCaptureAPI(apiServer *Server, g *graph.Graph) (*CaptureAPIHandler, error) {
	captureAPIHandler := &CaptureAPIHandler{
		BasicAPIHandler: BasicAPIHandler{
			ResourceHandler: &CaptureResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
		Graph: g,
	}
	if err := apiServer.RegisterAPIHandler(captureAPIHandler); err != nil {
		return nil, err
	}
	return captureAPIHandler, nil
}
