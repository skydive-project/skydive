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
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"

	"github.com/skydive-project/skydive/common"
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

type ConnectionType int

const (
	// TCP connection
	TCP ConnectionType = 1 + iota
	// TLS secure connection
	TLS
)

type Server struct {
	http.Server
	Host        string
	ServiceType common.ServiceType
	Router      *mux.Router
	Addr        string
	Port        int
	Auth        AuthenticationBackend
	lock        sync.Mutex
	listener    net.Listener
	CnxType     ConnectionType
	wg          sync.WaitGroup
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

	s.CnxType = TCP
	listenAddrPort := fmt.Sprintf("%s:%d", s.Addr, s.Port)
	var err error
	ln, err := net.Listen("tcp", listenAddrPort)
	if err != nil {
		logging.GetLogger().Fatalf("Failed to listen on %s:%d: %s", s.Addr, s.Port, err.Error())
	}
	s.listener = ln

	if config.IsTLSenabled() == true {
		s.CnxType = TLS
		certPEM := config.GetConfig().GetString("analyzer.X509_cert")
		keyPEM := config.GetConfig().GetString("analyzer.X509_key")
		agentCertPEM := config.GetConfig().GetString("agent.X509_cert")
		s.TLSConfig = common.SetupTLSServerConfig(certPEM, keyPEM)
		s.TLSConfig.ClientCAs = common.SetupTLSLoadCertificate(agentCertPEM)
		s.listener = tls.NewListener(ln.(*net.TCPListener), s.TLSConfig)
	}

	socketType := "TCP"
	if s.CnxType == TLS {
		socketType = "TLS"
	}
	logging.GetLogger().Infof("Listening on %s socket %s:%d", socketType, s.Addr, s.Port)

	s.Handler = s.Router
	err = s.Serve(s.listener)
	if err != nil {
		logging.GetLogger().Errorf("Failed to Serve on %s:%d: %s", s.Addr, s.Port, err.Error())
	}
}

func (s *Server) Stop() {
	s.listener.Close()
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

	setTLSHeader(w, r)
	w.Header().Set("Content-Type", ct+"; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	html, err := statics.Asset("statics/index.html")
	if err != nil {
		logging.GetLogger().Error("Unable to find the asset index.html")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	setTLSHeader(w, r)
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write(html)
}

func (s *Server) serveLogin(w http.ResponseWriter, r *http.Request) {
	setTLSHeader(w, r)
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
				w.WriteHeader(http.StatusOK)
			} else {
				unauthorized(w, r)
			}
		} else {
			unauthorized(w, r)
		}
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("401 Unauthorized\n"))
}

func (s *Server) HandleFunc(path string, f auth.AuthenticatedHandlerFunc) {
	s.Router.HandleFunc(path, s.Auth.Wrap(f))
}

func NewServer(host string, serviceType common.ServiceType, addr string, port int, auth AuthenticationBackend) *Server {
	router := mux.NewRouter().StrictSlash(true)
	router.Headers("X-Host-ID", host, "X-Service-Type", serviceType.String())

	router.PathPrefix("/statics").HandlerFunc(serveStatics)

	server := &Server{
		Host:        host,
		ServiceType: serviceType,
		Router:      router,
		Addr:        addr,
		Port:        port,
		Auth:        auth,
	}

	router.HandleFunc("/login", server.serveLogin)
	router.HandleFunc("/", server.serveIndex)

	return server
}

func NewServerFromConfig(serviceType common.ServiceType) (*Server, error) {
	auth, err := NewAuthenticationBackendFromConfig()
	if err != nil {
		return nil, err
	}

	sa, err := common.ServiceAddressFromString(config.GetConfig().GetString(serviceType.String() + ".listen"))
	if err != nil {
		return nil, errors.New("Configuration error: " + err.Error())
	}

	host := config.GetConfig().GetString("host_id")

	return NewServer(host, serviceType, sa.Addr, sa.Port, auth), nil
}
