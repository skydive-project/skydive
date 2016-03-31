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

package http

import (
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"
	"github.com/hydrogen18/stoppableListener"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/statics"
)

type PathPrefix string

type Route struct {
	Name        string
	Method      string
	Path        interface{}
	HandlerFunc auth.AuthenticatedHandlerFunc
}

type Server struct {
	Service string
	Router  *mux.Router
	Addr    string
	Port    int
	Auth    AuthRouteHandlerWrapper
	lock    sync.Mutex
	sl      *stoppableListener.StoppableListener
	wg      sync.WaitGroup
}

func (s *Server) RegisterRoutes(routes []Route) {
	for _, route := range routes {
		r := s.Router.
			Methods(route.Method).
			Name(route.Name).
			Handler(s.Auth.Wrap(route.HandlerFunc))
		switch p := route.Path.(type) {
		case string:
			r.Path(p)
		case PathPrefix:
			r.PathPrefix(string(p))
		}
	}
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

func serveStatics(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
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

func (s *Server) serveIndex(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
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

	data := &struct {
		Service string
	}{
		Service: s.Service,
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	t.Execute(w, data)
}

func (s *Server) HandleFunc(path string, f auth.AuthenticatedHandlerFunc) {
	s.Router.HandleFunc(path, s.Auth.Wrap(f))
}

func NewServer(s string, a string, p int, auth AuthRouteHandlerWrapper) *Server {
	router := mux.NewRouter().StrictSlash(true)

	router.PathPrefix("/statics").Handler(auth.Wrap(serveStatics))

	server := &Server{
		Service: s,
		Router:  router,
		Addr:    a,
		Port:    p,
		Auth:    auth,
	}

	router.HandleFunc("/", auth.Wrap(server.serveIndex))

	return server
}

func NewServerFromConfig(s string) (*Server, error) {
	auth, err := NewAuthRouteHandlerFromConfig()
	if err != nil {
		return nil, err
	}

	addr, port, err := config.GetHostPortAttributes(s, "listen")
	if err != nil {
		return nil, errors.New("Configuration error: " + err.Error())
	}

	return NewServer(s, addr, port, auth), nil
}
