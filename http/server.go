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
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/abbot/go-http-auth"
	gcontext "github.com/gorilla/context"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/rbac"
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
	sync.RWMutex
	http.Server
	Host        string
	ServiceType common.ServiceType
	Router      *mux.Router
	Addr        string
	Port        int
	AuthBackend AuthenticationBackend
	lock        sync.Mutex
	listener    net.Listener
	CnxType     ConnectionType
	wg          sync.WaitGroup
}

func copyRequestVars(old, new *http.Request) {
	kv := gcontext.GetAll(old)
	for k, v := range kv {
		gcontext.Set(new, k, v)
	}
}

func (s *Server) RegisterRoutes(routes []Route, auth AuthenticationBackend) {
	for _, route := range routes {
		r := s.Router.
			Methods(route.Method).
			Name(route.Name).
			Handler(auth.Wrap(route.HandlerFunc))
		switch p := route.Path.(type) {
		case string:
			r.Path(p)
		case PathPrefix:
			r.PathPrefix(string(p))
		}
	}
}

func (s *Server) Listen() error {
	listenAddrPort := fmt.Sprintf("%s:%d", s.Addr, s.Port)
	socketType := "TCP"
	ln, err := net.Listen("tcp", listenAddrPort)
	if err != nil {
		return fmt.Errorf("Failed to listen on %s:%d: %s", s.Addr, s.Port, err)
	}
	s.listener = ln

	if config.IsTLSenabled() == true {
		socketType = "TLS"
		certPEM := config.GetString("analyzer.X509_cert")
		keyPEM := config.GetString("analyzer.X509_key")
		agentCertPEM := config.GetString("agent.X509_cert")
		tlsConfig, err := common.SetupTLSServerConfig(certPEM, keyPEM)
		if err != nil {
			return err
		}
		tlsConfig.ClientCAs, err = common.SetupTLSLoadCertificate(agentCertPEM)
		if err != nil {
			return err
		}
		s.listener = tls.NewListener(ln.(*net.TCPListener), tlsConfig)
	}

	logging.GetLogger().Infof("Listening on %s socket %s:%d", socketType, s.Addr, s.Port)
	return nil
}

func (s *Server) ListenAndServe() {
	if err := s.Listen(); err != nil {
		logging.GetLogger().Critical(err)
	}

	go s.Serve()
}

func (s *Server) Serve() {
	defer s.wg.Done()
	s.wg.Add(1)

	s.Handler = handlers.CompressHandler(s.Router)
	if err := s.Server.Serve(s.listener); err != nil {
		if err == http.ErrServerClosed {
			return
		}
		logging.GetLogger().Errorf("Failed to serve on %s:%d: %s", s.Addr, s.Port, err)
	}
}

func Unauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("401 Unauthorized\n"))
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Server.Shutdown(ctx); err != nil {
		logging.GetLogger().Error("Shutdown error :", err)
	}
	s.listener.Close()
	s.wg.Wait()
}

// HandleFunc specifies the handler function and the authentication backend used for a given path
func (s *Server) HandleFunc(path string, f auth.AuthenticatedHandlerFunc, authBackend AuthenticationBackend) {
	postAuthHandler := func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
		// re-add user to its group
		if roles := rbac.GetUserRoles(r.Username); len(roles) == 0 {
			rbac.AddRoleForUser(r.Username, authBackend.DefaultUserRole(r.Username))
		}

		// re-send the permissions
		setPermissionsCookie(w, r.Username)

		f(w, r)
	}

	preAuthHandler := authBackend.Wrap(postAuthHandler)

	s.Router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		// set tls headers first
		SetTLSHeader(w, r)

		preAuthHandler(w, r)
	})
}

func NewServer(host string, serviceType common.ServiceType, addr string, port int) *Server {
	router := mux.NewRouter().StrictSlash(true)
	router.Headers("X-Host-ID", host, "X-Service-Type", serviceType.String())

	return &Server{
		Host:        host,
		ServiceType: serviceType,
		Router:      router,
		Addr:        addr,
		Port:        port,
	}
}

func NewServerFromConfig(serviceType common.ServiceType) (*Server, error) {
	sa, err := common.ServiceAddressFromString(config.GetString(serviceType.String() + ".listen"))
	if err != nil {
		return nil, fmt.Errorf("Configuration error: %s", err)
	}

	host := config.GetString("host_id")

	return NewServer(host, serviceType, sa.Addr, sa.Port), nil
}
