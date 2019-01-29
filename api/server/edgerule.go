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

// EdgeRuleResourceHandler describes a edge rule resource handler
type EdgeRuleResourceHandler struct {
	ResourceHandler
}

// EdgeRuleAPI based on BasicAPIHandler
type EdgeRuleAPI struct {
	BasicAPIHandler
	Graph *graph.Graph
}

// Name returns resource name "edgerule"
func (erh *EdgeRuleResourceHandler) Name() string {
	return "edgerule"
}

// New creates a new EdgeRule
func (erh *EdgeRuleResourceHandler) New() types.Resource {
	return &types.EdgeRule{}
}

// RegisterEdgeRuleAPI registers an EdgeRule's API to a designated API Server
func RegisterEdgeRuleAPI(apiServer *Server, g *graph.Graph, authBackend shttp.AuthenticationBackend) (*EdgeRuleAPI, error) {
	era := &EdgeRuleAPI{
		BasicAPIHandler: BasicAPIHandler{
			ResourceHandler: &EdgeRuleResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
		Graph: g,
	}
	if err := apiServer.RegisterAPIHandler(era, authBackend); err != nil {
		return nil, err
	}

	return era, nil
}
