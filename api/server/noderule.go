//go:generate sh -c "go run github.com/gomatic/renderizer --name='node rule' --resource=noderule --type=NodeRule --title='Node rule' --article=a swagger_operations.tmpl > noderule_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name='node rule' --resource=noderule --type=NodeRule --title='Node rule' swagger_definitions.tmpl > noderule_swagger.json"

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
	"github.com/skydive-project/skydive/graffiti/api/rest"
	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/http"
)

// NodeRuleResourceHandler describes a node resource handler
type NodeRuleResourceHandler struct {
	rest.ResourceHandler
}

// NodeRuleAPI based on BasicAPIHandler
type NodeRuleAPI struct {
	rest.BasicAPIHandler
	Graph *graph.Graph
}

// Name returns resource name "noderule"
func (nrh *NodeRuleResourceHandler) Name() string {
	return "noderule"
}

// New creates a new node rule
func (nrh *NodeRuleResourceHandler) New() rest.Resource {
	return &types.NodeRule{}
}

// RegisterNodeRuleAPI register a new node rule api handler
func RegisterNodeRuleAPI(apiServer *api.Server, g *graph.Graph, authBackend shttp.AuthenticationBackend) (*NodeRuleAPI, error) {
	nra := &NodeRuleAPI{
		BasicAPIHandler: rest.BasicAPIHandler{
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
