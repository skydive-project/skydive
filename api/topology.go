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

package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/abbot/go-http-auth"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/storage"
	ftraversal "github.com/skydive-project/skydive/flow/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	"github.com/skydive-project/skydive/validator"
)

type TopologyApi struct {
	Service     string
	Graph       *graph.Graph
	TableClient *flow.TableClient
	Storage     storage.Storage
}

type Topology struct {
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
}

func (t *TopologyApi) topologyIndex(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(t.Graph); err != nil {
		panic(err)
	}
}

func (t *TopologyApi) topologySearch(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	resource := Topology{}

	data, _ := ioutil.ReadAll(r.Body)
	if len(data) != 0 {
		if err := json.Unmarshal(data, &resource); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		if err := validator.Validate(resource); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
	}

	if resource.GremlinQuery == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	tr := traversal.NewGremlinTraversalParser(t.Graph)
	tr.AddTraversalExtension(topology.NewTopologyTraversalExtension())
	if t.TableClient != nil {
		tr.AddTraversalExtension(ftraversal.NewFlowTraversalExtension(t.TableClient, t.Storage))
	}

	ts, err := tr.Parse(strings.NewReader(resource.GremlinQuery))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	res, err := ts.Exec()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		panic(err)
	}
}

func (t *TopologyApi) registerEndpoints(r *shttp.Server) {
	routes := []shttp.Route{
		{
			"TopologiesIndex",
			"GET",
			"/api/topology",
			t.topologyIndex,
		},
		{
			"TopologiesSearch",
			"POST",
			"/api/topology",
			t.topologySearch,
		},
	}

	r.RegisterRoutes(routes)
}

func RegisterTopologyApi(s string, g *graph.Graph, r *shttp.Server, tc *flow.TableClient, st storage.Storage) {
	t := &TopologyApi{
		Service:     s,
		Graph:       g,
		TableClient: tc,
		Storage:     st,
	}

	t.registerEndpoints(r)
}
