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
	"html/template"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/hydrogen18/stoppableListener"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/statics"
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
	Auth    AuthenticationBackend
	lock    sync.Mutex
	sl      *stoppableListener.StoppableListener
	wg      sync.WaitGroup
}

func copyRequestVars(old, new *http.Request) {
	kv := context.GetAll(old)
	for k, v := range kv {
		context.Set(new, k, v)
	}
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

func serveStatics(w http.ResponseWriter, r *http.Request) {
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
	s.renderTemplate(w, "topology.html")
}

func (s *Server) serveLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
		loginForm, passwordForm := r.Form["username"], r.Form["password"]
		if len(loginForm) != 0 && len(passwordForm) != 0 {
			login, password := loginForm[0], passwordForm[0]
			if token, err := s.Auth.Authenticate(login, password); err == nil {
				if token != "" {
					cookie := &http.Cookie{
						Name:  "authtok",
						Value: token,
					}
					http.SetCookie(w, cookie)
				}
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}
		unauthorized(w, r)
	} else {
		s.renderTemplate(w, "login.html")
	}
}

func (s *Server) getTemplateData() interface{} {
	return &struct {
		Service string
	}{
		Service: s.Service,
	}
}

func (s *Server) renderTemplate(w http.ResponseWriter, page string) {
	html, err := statics.Asset("statics/" + page)
	if err != nil {
		logging.GetLogger().Error("Unable to find the asset " + page)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	t := template.New(page)
	t, err = t.Parse(string(html))
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err = t.Execute(w, s.getTemplateData()); err != nil {
		logging.GetLogger().Errorf("Unable to render template: %s", err.Error())
	}
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	if acceptHeader, ok := r.Header["Accept"]; ok {
		for _, contentType := range strings.Split(acceptHeader[0], ",") {
			if contentType == "text/html" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
		}
	}

	w.WriteHeader(401)
	w.Write([]byte("401 Unauthorized\n"))
}

func (s *Server) HandleFunc(path string, f auth.AuthenticatedHandlerFunc) {
	s.Router.HandleFunc(path, s.Auth.Wrap(f))
}

func NewServer(s string, a string, p int, auth AuthenticationBackend) *Server {
	router := mux.NewRouter().StrictSlash(true)

	router.PathPrefix("/statics").HandlerFunc(serveStatics)

	server := &Server{
		Service: s,
		Router:  router,
		Addr:    a,
		Port:    p,
		Auth:    auth,
	}

	router.HandleFunc("/login", server.serveLogin)
	router.HandleFunc("/", auth.Wrap(server.serveIndex))

	return server
}

func NewServerFromConfig(s string) (*Server, error) {
	auth, err := NewAuthenticationBackendFromConfig()
	if err != nil {
		return nil, err
	}

	addr, port, err := config.GetHostPortAttributes(s, "listen")
	if err != nil {
		return nil, errors.New("Configuration error: " + err.Error())
	}

	return NewServer(s, addr, port, auth), nil
}
