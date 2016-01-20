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

package analyzer

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/rpc"
	"github.com/redhat-cip/skydive/storage"
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
)

type Server struct {
	Port                int
	Router              *mux.Router
	TopoServer          *topology.Server
	GraphServer         *graph.Server
	FlowMappingPipeline *mappings.FlowMappingPipeline
	Storage             storage.Storage
}

func (s *Server) AnalyzeFlows(flows []*flow.Flow) {
	s.FlowMappingPipeline.Enhance(flows)

	if s.Storage != nil {
		s.Storage.StoreFlows(flows)
	}

	logging.GetLogger().Debug("%d flows stored", len(flows))
}

func (s *Server) handleUDPFlowPacket(conn *net.UDPConn) {
	data := make([]byte, 4096)

	for {
		n, _, err := conn.ReadFromUDP(data)
		if err != nil {
			logging.GetLogger().Error("Error while reading: %s", err.Error())
			return
		}

		f, err := flow.FromData(data[0:n])
		if err != nil {
			logging.GetLogger().Error("Error while parsing flow: %s", err.Error())
		}

		s.AnalyzeFlows([]*flow.Flow{f})
	}
}

func (s *Server) ListenAndServe() {
	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		s.TopoServer.ListenAndServe()
	}()

	go func() {
		defer wg.Done()
		s.GraphServer.ListenAndServe()
	}()

	go func() {
		defer wg.Done()

		addr, err := net.ResolveUDPAddr("udp", ":"+strconv.FormatInt(int64(s.Port), 10))
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		s.handleUDPFlowPacket(conn)
	}()

	wg.Wait()
}

func (s *Server) FlowSearch(w http.ResponseWriter, r *http.Request) {
	filters := make(storage.Filters)
	for k, v := range r.URL.Query() {
		filters[k] = v[0]
	}

	flows, err := s.Storage.SearchFlows(filters)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(flows); err != nil {
		panic(err)
	}
}

func (s *Server) RegisterRpcEndpoints() {
	routes := []rpc.Route{
		rpc.Route{
			"FlowSearch",
			"GET",
			"/rpc/flows",
			s.FlowSearch,
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

func (s *Server) SetStorage(st storage.Storage) {
	s.Storage = st
}

func NewServer(port int, router *mux.Router) (*Server, error) {
	backend, err := graph.NewMemoryBackend()
	if err != nil {
		return nil, err
	}

	g, err := graph.NewGraph(backend)
	if err != nil {
		return nil, err
	}

	tserver := topology.NewServer(g, port, router)
	tserver.RegisterStaticEndpoints()
	tserver.RegisterRpcEndpoints()

	alertmgr := graph.NewAlert(g, port, router)
	alertmgr.RegisterStaticEndpoints()
	alertmgr.RegisterRpcEndpoints()

	gserver := graph.NewServer(g, alertmgr, router)

	gfe, err := mappings.NewGraphFlowEnhancer(g)
	if err != nil {
		return nil, err
	}

	pipeline := mappings.NewFlowMappingPipeline([]mappings.FlowEnhancer{gfe})

	server := &Server{
		Port:                port,
		Router:              router,
		TopoServer:          tserver,
		GraphServer:         gserver,
		FlowMappingPipeline: pipeline,
	}
	server.RegisterRpcEndpoints()

	return server, nil
}
