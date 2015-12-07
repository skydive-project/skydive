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
	"text/template"

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/statics"
)

type Service struct {
	GlobalTopology *GlobalTopology
}

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

func (s *Service) htmlOutput(w http.ResponseWriter, r *http.Request) {
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
	}{
		Hostname: host,
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	t.Execute(w, data)
}

func (s *Service) TopologiesIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(s.GlobalTopology); err != nil {
		panic(err)
	}
}

func (s *Service) TopologyShow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	host := vars["host"]

	if topo, ok := s.GlobalTopology.Get(host); ok {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(topo); err != nil {
			panic(err)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Service) TopologyInsert(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	host := vars["host"]

	topology := NewTopology(host)

	err := json.NewDecoder(r.Body).Decode(topology)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.GlobalTopology.Add(topology)

	if topo, ok := s.GlobalTopology.Get(host); ok {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(topo); err != nil {
			panic(err)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}

	logging.GetLogger().Debug(topology.String())
}

func RegisterStaticEndpoints(global *GlobalTopology, router *mux.Router) {
	service := &Service{
		GlobalTopology: global,
	}

	// static routes
	router.HandleFunc("/static/topology.html", service.htmlOutput)
}

func RegisterRpcEndpoints(global *GlobalTopology, router *mux.Router) {
	service := &Service{
		GlobalTopology: global,
	}

	routes := []Route{
		Route{
			"TopologiesIndex",
			"GET",
			"/rpc/topology",
			service.TopologiesIndex,
		},
		Route{
			"TopologyShow",
			"GET",
			"/rpc/topology/{host}",
			service.TopologyShow,
		},
		Route{
			"TopologyInsert",
			"POST",
			"/rpc/topology/{host}",
			service.TopologyInsert,
		},
	}

	for _, route := range routes {
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}
}
