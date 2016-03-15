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
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/hydrogen18/stoppableListener"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/rpc"
	"github.com/redhat-cip/skydive/statics"
	"github.com/redhat-cip/skydive/topology/graph"
)

type Server struct {
	Service string
	Graph   *graph.Graph
	Router  *mux.Router
	Addr    string
	Port    int
	lock    sync.Mutex
	sl      *stoppableListener.StoppableListener
	wg      sync.WaitGroup
}

type StaticHandler struct {
}

func StaticServer() http.Handler {
	return &StaticHandler{}
}

func (s *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if strings.HasPrefix(upath, "/") {
		upath = strings.TrimPrefix(upath, "/")
	}

	content, err := statics.Asset(upath)
	if err != nil {
		logging.GetLogger().Errorf("Unable to find the asset: %s", upath)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ext := filepath.Ext(upath)
	ct := mime.TypeByExtension(ext)

	w.Header().Set("Content-Type", ct+"; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	html, err := statics.Asset("statics/topology.html")
	if err != nil {
		logging.GetLogger().Error("Unable to find the topology asset")
		w.WriteHeader(http.StatusNotFound)
		return
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

	data := &struct {
		Service  string
		Hostname string
		Port     int
	}{
		Service:  s.Service,
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

func (s *Server) RegisterStaticEndpoints() {
	s.Router.HandleFunc("/", s.serveIndex)
	s.Router.PathPrefix("/statics").Handler(StaticServer())
}

func (s *Server) RegisterRPCEndpoints() {
	routes := []rpc.Route{
		{
			"TopologiesIndex",
			"GET",
			"/rpc/topology",
			s.TopologyIndex,
		},
	}

	rpc.RegisterRoutes(s.Router, routes)
}

func (s *Server) ListenAndServe() {
	defer s.wg.Done()
	s.wg.Add(1)

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Addr, s.Port))
	if err != nil {
		logging.GetLogger().Fatalf("Failed to listen on %s:%d: %s", s.Addr, s.Port, err.Error())
	}

	s.lock.Lock()
	s.sl, err = stoppableListener.New(listener)
	if err != nil {
		s.lock.Unlock()
		logging.GetLogger().Fatalf("Failed to create stoppable listener: %s", err.Error())
	}
	s.lock.Unlock()

	http.Serve(s.sl, s.Router)
}

func (s *Server) Stop() {
	s.lock.Lock()
	s.sl.Stop()
	s.lock.Unlock()

	s.wg.Wait()
}

func NewServer(s string, g *graph.Graph, a string, p int, router *mux.Router) *Server {
	return &Server{
		Service: s,
		Graph:   g,
		Router:  router,
		Addr:    a,
		Port:    p,
	}
}

func NewServerFromConfig(s string, g *graph.Graph, router *mux.Router) (*Server, error) {
	addr, port, err := config.GetHostPortAttributes(s, "listen")
	if err != nil {
		return nil, errors.New("Configuration error: " + err.Error())
	}

	return NewServer(s, g, addr, port, router), nil
}
