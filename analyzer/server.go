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
	"os"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/rpc"
	"github.com/redhat-cip/skydive/statics"
	"github.com/redhat-cip/skydive/storage"
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
)

type Server struct {
	Addr                string
	Port                int
	Stopping            bool
	Router              *mux.Router
	TopoServer          *topology.Server
	GraphServer         *graph.Server
	FlowMappingPipeline *mappings.FlowMappingPipeline
	Storage             storage.Storage
	FlowTable           *flow.FlowTable
	Conn                *net.UDPConn
}

func (s *Server) flowExpire(f *flow.Flow) {
	/* Storge flow in the database */
}

func (s *Server) AnalyzeFlows(flows []*flow.Flow) {
	s.FlowTable.Update(flows)
	s.FlowMappingPipeline.Enhance(flows)

	if s.Storage != nil {
		s.Storage.StoreFlows(flows)
	}

	logging.GetLogger().Debug("%d flows stored", len(flows))
}

func (s *Server) handleUDPFlowPacket() {
	data := make([]byte, 4096)

	for {
		n, _, err := s.Conn.ReadFromUDP(data)
		if err != nil {
			if !s.Stopping {
				logging.GetLogger().Error("Error while reading: %s", err.Error())
			}
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

		addr, err := net.ResolveUDPAddr("udp", s.Addr+":"+strconv.FormatInt(int64(s.Port), 10))
		s.Conn, err = net.ListenUDP("udp", addr)
		if err != nil {
			panic(err)
		}
		defer s.Conn.Close()

		s.handleUDPFlowPacket()
	}()

	wg.Wait()
}

func (s *Server) Stop() {
	s.Stopping = true
	s.TopoServer.Stop()
	s.GraphServer.Stop()
	s.Conn.Close()
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

func (s *Server) serveDataIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/data/conversation.json" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(s.FlowTable.JSONFlowConversationEthernetPath()))
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	return
}

func (s *Server) serveStaticIndex(w http.ResponseWriter, r *http.Request) {
	html, err := statics.Asset("statics/conversation.html")
	if err != nil {
		logging.GetLogger().Panic("Unable to find the conversation asset")
	}

	t := template.New("conversation template")

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

func (s *Server) RegisterStaticEndpoints() {
	// static routes
	s.Router.HandleFunc("/static/conversation", s.serveStaticIndex)
	s.Router.HandleFunc("/data/conversation.json", s.serveDataIndex)
}

func (s *Server) RegisterRpcEndpoints() {
	routes := []rpc.Route{
		{
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

func NewServer(addr string, port int, router *mux.Router) (*Server, error) {
	backend, err := graph.BackendFromConfig()
	if err != nil {
		return nil, err
	}

	g, err := graph.NewGraph(backend)
	if err != nil {
		return nil, err
	}

	tserver := topology.NewServer(g, addr, port, router)
	tserver.RegisterStaticEndpoints()
	tserver.RegisterRpcEndpoints()

	alertmgr := graph.NewAlert(g, router)
	alertmgr.RegisterRpcEndpoints()

	gserver, err := graph.NewServerFromConfig(g, alertmgr, router)
	if err != nil {
		return nil, err
	}

	gfe, err := mappings.NewGraphFlowEnhancer(g)
	if err != nil {
		return nil, err
	}

	pipeline := mappings.NewFlowMappingPipeline([]mappings.FlowEnhancer{gfe})

	flowtable := flow.NewFlowTable()

	server := &Server{
		Addr:                addr,
		Port:                port,
		Router:              router,
		TopoServer:          tserver,
		GraphServer:         gserver,
		FlowMappingPipeline: pipeline,
		FlowTable:           flowtable,
	}
	server.RegisterStaticEndpoints()
	server.RegisterRpcEndpoints()
	cfgFlowtable_expire, err := config.GetConfig().Section("analyzer").Key("flowtable_expire").Int()
	if err != nil || cfgFlowtable_expire < 1 {
		logging.GetLogger().Error("Config flowTable_expire invalid value ", cfgFlowtable_expire, err.Error())
		return nil, err
	}
	go flowtable.AsyncExpire(server.flowExpire, time.Duration(cfgFlowtable_expire)*time.Minute)

	return server, nil
}

func NewServerFromConfig(router *mux.Router) (*Server, error) {
	addr, port, err := config.GetHostPortAttributes("analyzer", "listen")
	if err != nil {
		logging.GetLogger().Error("Configuration error: %s", err.Error())
	}

	return NewServer(addr, port, router)
}
