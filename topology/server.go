/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package topology

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"text/template"

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/rpc"
	"github.com/redhat-cip/skydive/statics"
	"github.com/redhat-cip/skydive/topology/graph"
)

type Server struct {
	Graph  *graph.Graph
	Router *mux.Router
	Port   int
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	html, err := statics.Asset("statics/topology.html")
	if err != nil {
		logging.GetLogger().Panic("Unable to find the topology asset")
	}

	t := template.New("topology template")

	t, err = t.Parse(string(html))
	if err != nil {
		panic(err)
	}

	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	var data = &struct {
		Hostname string
		Port     int
	}{
		Hostname: host,
		Port:     s.Port,
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	t.Execute(w, data)
}

func (s *Server) TopologyIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(s.Graph); err != nil {
		panic(err)
	}
}

func (s *Server) TopologyShow(w http.ResponseWriter, r *http.Request) {
	/*vars := mux.Vars(r)
	host := vars["host"]

	if topo, ok := s.MultiNodeTopology.Get(host); ok {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(topo); err != nil {
			panic(err)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}*/
}

func (s *Server) RegisterStaticEndpoints() {

	// static routes
	s.Router.HandleFunc("/static/topology", s.serveIndex)
}

func (s *Server) RegisterRpcEndpoints() {
	routes := []rpc.Route{
		{
			"TopologiesIndex",
			"GET",
			"/rpc/topology",
			s.TopologyIndex,
		},
		{
			"TopologyShow",
			"GET",
			"/rpc/topology/{host}",
			s.TopologyShow,
		},
	}

	for _, route := range routes {
		s.Router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}
}

func (s *Server) ListenAndServe() {
	http.ListenAndServe(":"+strconv.FormatInt(int64(s.Port), 10), s.Router)
}

func NewServer(g *graph.Graph, p int, router *mux.Router) *Server {
	return &Server{
		Graph:  g,
		Router: router,
		Port:   p,
	}
}
