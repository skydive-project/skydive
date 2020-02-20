//go:generate sh -c "go run github.com/gomatic/renderizer --name=capture --resource=capture --type=Capture --title=Capture --article=a swagger_operations.tmpl > capture_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name=capture --resource=capture --type=Capture --title=Capture --article=a swagger_definitions.tmpl > capture_swagger.json"

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

package server

import (
	"fmt"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

// CaptureResourceHandler describes a capture ressouce handler
type CaptureResourceHandler struct {
	rest.ResourceHandler
}

// CaptureAPIHandler based on BasicAPIHandler
type CaptureAPIHandler struct {
	rest.BasicAPIHandler
	Graph *graph.Graph
}

// Name returns "capture"
func (c *CaptureResourceHandler) Name() string {
	return "capture"
}

// New creates a new capture resource
func (c *CaptureResourceHandler) New() rest.Resource {
	return &types.Capture{
		LayerKeyMode: flow.DefaultLayerKeyModeName(),
	}
}

// Decorate populates the capture resource
func (c *CaptureAPIHandler) Decorate(resource rest.Resource) {
	capture := resource.(*types.Capture)

	count := 0

	c.Graph.RLock()
	defer c.Graph.RUnlock()

	res, err := ge.TopologyGremlinQuery(c.Graph, capture.GremlinQuery)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin error: %s", err)
		return
	}

	countCaptures := func(n *graph.Node) (count int) {
		if field, err := n.GetField("Captures"); err == nil {
			if captures, ok := field.(*probes.Captures); ok {
				for _, capture := range *captures {
					if capture.State == "active" {
						count++
					}
				}

			}
		}
		return
	}

	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			n := value.(*graph.Node)
			count += countCaptures(n)
		case []*graph.Node:
			for _, n := range value.([]*graph.Node) {
				count += countCaptures(n)
			}
		default:
			count = 0
		}
	}

	capture.Count = count
}

// Create tests that resource GremlinQuery does not exists already
func (c *CaptureAPIHandler) Create(r rest.Resource, opts *rest.CreateOptions) error {
	capture := r.(*types.Capture)

	// check capabilities
	if capture.Type != "" {
		if capture.BPFFilter != "" {
			if !probes.CheckProbeCapabilities(capture.Type, probes.BPFCapability) {
				return fmt.Errorf("%s capture doesn't support BPF filtering", capture.Type)
			}
		}
		if capture.RawPacketLimit != 0 {
			if !probes.CheckProbeCapabilities(capture.Type, probes.RawPacketsCapability) {
				return fmt.Errorf("%s capture doesn't support raw packet capture", capture.Type)
			}
		}
		if capture.ExtraTCPMetric {
			if !probes.CheckProbeCapabilities(capture.Type, probes.ExtraTCPMetricCapability) {
				return fmt.Errorf("%s capture doesn't support extra TCP metrics capture", capture.Type)
			}
		}
	}

	resources := c.Index()
	for _, resource := range resources {
		resource := resource.(*types.Capture)

		sameGremlin := resource.GremlinQuery == capture.GremlinQuery
		sameBPFFilter := resource.BPFFilter == capture.BPFFilter
		sameCaptureType := resource.Type == capture.Type
		supportsMulti := capture.Type == "" || probes.CheckProbeCapabilities(capture.Type, probes.MultipleOnSameNodeCapability)

		if sameCaptureType && sameGremlin {
			if !supportsMulti || sameBPFFilter {
				return rest.ErrDuplicatedResource
			}
		}
	}

	return c.BasicAPIHandler.Create(r, opts)
}

// RegisterCaptureAPI registers an new resource, capture
func RegisterCaptureAPI(apiServer *api.Server, g *graph.Graph, authBackend shttp.AuthenticationBackend) (*CaptureAPIHandler, error) {
	captureAPIHandler := &CaptureAPIHandler{
		BasicAPIHandler: rest.BasicAPIHandler{
			ResourceHandler: &CaptureResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
		Graph: g,
	}
	if err := apiServer.RegisterAPIHandler(captureAPIHandler, authBackend); err != nil {
		return nil, err
	}
	return captureAPIHandler, nil
}
