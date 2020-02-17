//go:generate sh -c "go run github.com/gomatic/renderizer --name=edge --resource=edge --type=Edge --title=Edge --article=a swagger_operations.tmpl > edge_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name=edge --resource=edge --type=Edge --title=Edge swagger_definitions.tmpl > edge_swagger.json"

/*
 * Copyright (C) 2020 T3B, Inc.
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
	"time"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/http"
)

// EdgeResourceHandler aims to creates and manage a new Alert.
type EdgeResourceHandler struct {
	ResourceHandler
}

// EdgeAPIHandler aims to exposes the Alert API.
type EdgeAPIHandler struct {
	g *graph.Graph
}

// New creates a new edge
func (h *EdgeAPIHandler) New() types.Resource {
	return &types.Edge{}
}

// Name returns resource name "edge"
func (h *EdgeAPIHandler) Name() string {
	return "edge"
}

// Index returns the list of existing edges
func (h *EdgeAPIHandler) Index() map[string]types.Resource {
	edges := h.g.GetEdges(nil)
	edgeMap := make(map[string]types.Resource, len(edges))
	for _, edge := range edges {
		n := types.Edge(*edge)
		edgeMap[string(edge.ID)] = &n
	}
	return edgeMap
}

// Get returns a edge with the specified id
func (h *EdgeAPIHandler) Get(id string) (types.Resource, bool) {
	n := h.g.GetEdge(graph.Identifier(id))
	if n == nil {
		return nil, false
	}
	edge := types.Edge(*n)
	return &edge, true
}

// Decorate the specified edge
func (h *EdgeAPIHandler) Decorate(resource types.Resource) {
}

// Create adds the specified edge to the graph
func (h *EdgeAPIHandler) Create(resource types.Resource, createOpts *CreateOptions) error {
	edge := resource.(*types.Edge)
	graphEdge := graph.Edge(*edge)
	if graphEdge.CreatedAt.IsZero() {
		graphEdge.CreatedAt = graph.Time(time.Now())
	}
	if graphEdge.UpdatedAt.IsZero() {
		graphEdge.UpdatedAt = graphEdge.CreatedAt
	}
	if graphEdge.Origin == "" {
		graphEdge.Origin = h.g.Origin()
	}
	if graphEdge.Metadata == nil {
		graphEdge.Metadata = graph.Metadata{}
	}
	return h.g.AddEdge(&graphEdge)
}

// Delete the edge with the specified id from the graph
func (h *EdgeAPIHandler) Delete(id string) error {
	edge := h.g.GetEdge(graph.Identifier(id))
	if edge == nil {
		return common.ErrNotFound
	}
	return h.g.DelEdge(edge)
}

// RegisterEdgeAPI registers the edge API
func RegisterEdgeAPI(apiServer *Server, g *graph.Graph, authBackend shttp.AuthenticationBackend) (*EdgeAPIHandler, error) {
	edgeAPIHandler := &EdgeAPIHandler{
		g: g,
	}
	if err := apiServer.RegisterAPIHandler(edgeAPIHandler, authBackend); err != nil {
		return nil, err
	}
	return edgeAPIHandler, nil
}
