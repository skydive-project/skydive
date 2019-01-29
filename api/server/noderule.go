/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/http"
)

// NodeRuleResourceHandler describes a node resource handler
type NodeRuleResourceHandler struct {
	ResourceHandler
}

// NodeRuleAPI based on BasicAPIHandler
type NodeRuleAPI struct {
	BasicAPIHandler
	Graph *graph.Graph
}

// Name returns resource name "noderule"
func (nrh *NodeRuleResourceHandler) Name() string {
	return "noderule"
}

// New creates a new node rule
func (nrh *NodeRuleResourceHandler) New() types.Resource {
	return &types.NodeRule{}
}

// Create a new node rule
func (nra *NodeRuleAPI) Create(r types.Resource) error {
	return nra.BasicAPIHandler.Create(r)
}

// RegisterNodeRuleAPI register a new node rule api handler
func RegisterNodeRuleAPI(apiServer *Server, g *graph.Graph, authBackend shttp.AuthenticationBackend) (*NodeRuleAPI, error) {
	nra := &NodeRuleAPI{
		BasicAPIHandler: BasicAPIHandler{
			ResourceHandler: &NodeRuleResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
		Graph: g,
	}
	if err := apiServer.RegisterAPIHandler(nra, authBackend); err != nil {
		return nil, err
	}

	return nra, nil
}
