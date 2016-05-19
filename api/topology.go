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
	"net/http"
	"strings"

	"github.com/abbot/go-http-auth"
	shttp "github.com/redhat-cip/skydive/http"
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
)

type TopologyApi struct {
	Service string
	Graph   *graph.Graph
}

type Topology struct {
	GremlinQuery string `json:"GremlinQuery,omitempty"`
}

func (t *TopologyApi) topologyIndex(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	resource := Topology{}
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if resource.GremlinQuery != "" {
		tr := graph.NewGremlinTraversalParser(strings.NewReader(resource.GremlinQuery), t.Graph)
		tr.AddTraversalExtension(topology.NewTopologyTraversalExtension())

		ts, err := tr.Parse()
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
		if err := json.NewEncoder(w).Encode(res.Values()); err != nil {
			panic(err)
		}
	} else {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(t.Graph); err != nil {
			panic(err)
		}
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
	}

	r.RegisterRoutes(routes)
}

func RegisterTopologyApi(s string, g *graph.Graph, r *shttp.Server) {
	t := &TopologyApi{
		Service: s,
		Graph:   g,
	}

	t.registerEndpoints(r)
}
