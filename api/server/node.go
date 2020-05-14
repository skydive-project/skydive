//go:generate sh -c "go run github.com/gomatic/renderizer --name=node --resource=node --type=Node --title=Node --article=a swagger_operations.tmpl > node_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name=node --resource=node --type=Node --title=Node swagger_definitions.tmpl > node_swagger.json"

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

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/http"
)

// NodeResourceHandler aims to creates and manage a new Alert.
type NodeResourceHandler struct {
	rest.ResourceHandler
}

// NodeAPIHandler aims to exposes the Alert API.
type NodeAPIHandler struct {
	g *graph.Graph
}

// New creates a new node
func (h *NodeAPIHandler) New() rest.Resource {
	return &types.Node{}
}

// Name returns resource name "node"
func (h *NodeAPIHandler) Name() string {
	return "node"
}

// Index returns the list of existing nodes
func (h *NodeAPIHandler) Index() map[string]rest.Resource {
	nodes := h.g.GetNodes(nil)
	nodeMap := make(map[string]rest.Resource, len(nodes))
	for _, node := range nodes {
		n := types.Node(*node)
		nodeMap[string(node.ID)] = &n
	}
	return nodeMap
}

// Get returns a node with the specified id
func (h *NodeAPIHandler) Get(id string) (rest.Resource, bool) {
	n := h.g.GetNode(graph.Identifier(id))
	if n == nil {
		return nil, false
	}
	node := types.Node(*n)
	return &node, true
}

// Decorate the specified node
func (h *NodeAPIHandler) Decorate(resource rest.Resource) {
}

// Create adds the specified node to the graph
func (h *NodeAPIHandler) Create(resource rest.Resource, createOpts *rest.CreateOptions) error {
	node := resource.(*types.Node)
	graphNode := graph.Node(*node)
	if graphNode.CreatedAt.IsZero() {
		graphNode.CreatedAt = graph.Time(time.Now())
	}
	if graphNode.UpdatedAt.IsZero() {
		graphNode.UpdatedAt = graphNode.CreatedAt
	}
	if graphNode.Origin == "" {
		graphNode.Origin = h.g.Origin()
	}
	if graphNode.Metadata == nil {
		graphNode.Metadata = graph.Metadata{}
	}
	return h.g.AddNode(&graphNode)
}

// Delete the node with the specified id from the graph
func (h *NodeAPIHandler) Delete(id string) error {
	node := h.g.GetNode(graph.Identifier(id))
	if node == nil {
		return common.ErrNotFound
	}
	return h.g.DelNode(node)
}

// RegisterNodeAPI registers the node API
func RegisterNodeAPI(apiServer *api.Server, g *graph.Graph, authBackend shttp.AuthenticationBackend) (*NodeAPIHandler, error) {
	nodeAPIHandler := &NodeAPIHandler{
		g: g,
	}
	if err := apiServer.RegisterAPIHandler(nodeAPIHandler, authBackend); err != nil {
		return nil, err
	}
	return nodeAPIHandler, nil
}
