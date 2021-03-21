//go:generate sh -c "go run github.com/gomatic/renderizer --name=edge --resource=edge --type=Edge --title=Edge --article=a ../../../api/server/swagger_operations.tmpl > edge_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name=edge --resource=edge --type=Edge --title=Edge ../../../api/server/swagger_definitions.tmpl > edge_swagger.json"

/*
 * Copyright (C) 2020 Sylvain Baubeau
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

	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
)

// EdgeResourceHandler aims to creates and manage a new Alert.
type EdgeResourceHandler struct {
	rest.ResourceHandler
}

// EdgeAPIHandler aims to exposes the Alert API.
type EdgeAPIHandler struct {
	g *graph.Graph
}

// New creates a new edge
func (h *EdgeAPIHandler) New() rest.Resource {
	return &types.Edge{}
}

// Name returns resource name "edge"
func (h *EdgeAPIHandler) Name() string {
	return "edge"
}

// Index returns the list of existing edges
func (h *EdgeAPIHandler) Index() map[string]rest.Resource {
	h.g.RLock()
	edges := h.g.GetEdges(nil)
	edgeMap := make(map[string]rest.Resource, len(edges))
	for _, edge := range edges {
		n := types.Edge(*edge)
		edgeMap[string(edge.ID)] = &n
	}
	h.g.RUnlock()
	return edgeMap
}

// Get returns a edge with the specified id
func (h *EdgeAPIHandler) Get(id string) (rest.Resource, bool) {
	h.g.RLock()
	defer h.g.RUnlock()

	e := h.g.GetEdge(graph.Identifier(id))
	if e == nil {
		return nil, false
	}
	edge := (*types.Edge)(e)
	return edge, true
}

// Decorate the specified edge
func (h *EdgeAPIHandler) Decorate(resource rest.Resource) {
}

// Create adds the specified edge to the graph
func (h *EdgeAPIHandler) Create(resource rest.Resource, createOpts *rest.CreateOptions) error {
	edge := resource.(*types.Edge)
	graphEdge := graph.Edge(*edge)
	if graphEdge.CreatedAt.IsZero() {
		graphEdge.CreatedAt = graph.Time(time.Now())
	}
	if graphEdge.UpdatedAt.IsZero() {
		graphEdge.UpdatedAt = graphEdge.CreatedAt
	}
	if graphEdge.Origin == "" {
		graphEdge.Origin = graph.Origin(h.g.GetHost(), apiOrigin)
	}
	if graphEdge.Metadata == nil {
		graphEdge.Metadata = graph.Metadata{}
	}

	h.g.Lock()
	err := h.g.AddEdge(&graphEdge)
	h.g.Unlock()
	return err
}

// Delete the edge with the specified id from the graph
func (h *EdgeAPIHandler) Delete(id string) error {
	h.g.RLock()
	defer h.g.RUnlock()

	edge := h.g.GetEdge(graph.Identifier(id))
	if edge == nil {
		return rest.ErrNotFound
	}

	return h.g.DelEdge(edge)
}

// Update a edge metadata
func (h *EdgeAPIHandler) Update(id string, resource rest.Resource) (rest.Resource, bool, error) {
	h.g.Lock()
	defer h.g.Unlock()

	e := h.g.GetEdge(graph.Identifier(id))
	if e == nil {
		return nil, false, rest.ErrNotFound
	}

	// Edge containing the metadata updated
	updateData := resource.(*types.Edge)

	previousRevision := e.Revision
	if err := h.g.SetMetadata(e, updateData.Metadata); err != nil {
		return nil, false, err
	}

	return (*types.Edge)(e), e.Revision != previousRevision, nil
}

// RegisterEdgeAPI registers the edge API
func RegisterEdgeAPI(apiServer *Server, g *graph.Graph, authBackend shttp.AuthenticationBackend) *EdgeAPIHandler {
	edgeAPIHandler := &EdgeAPIHandler{
		g: g,
	}
	apiServer.RegisterAPIHandler(edgeAPIHandler, authBackend)
	return edgeAPIHandler
}
