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
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/abbot/go-http-auth"
	gcontext "github.com/gorilla/context"
	"github.com/gorilla/handlers"
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

	// ExtraAssetPrefix is used for extra assets
	ExtraAssetPrefix = "/extra-statics"
)

type ExtraAsset struct {
	Filename string
	Ext      string
	Content  []byte
}

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
	extraAssets map[string]ExtraAsset
}

func copyRequestVars(old, new *http.Request) {
	kv := gcontext.GetAll(old)
	for k, v := range kv {
		gcontext.Set(new, k, v)
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

func (s *Server) Listen() error {
	listenAddrPort := fmt.Sprintf("%s:%d", s.Addr, s.Port)
	socketType := "TCP"
	ln, err := net.Listen("tcp", listenAddrPort)
	if err != nil {
		return fmt.Errorf("Failed to listen on %s:%d: %s", s.Addr, s.Port, err.Error())
	}
	s.listener = ln

	if config.IsTLSenabled() == true {
		socketType = "TLS"
		certPEM := config.GetString("analyzer.X509_cert")
		keyPEM := config.GetString("analyzer.X509_key")
		agentCertPEM := config.GetString("agent.X509_cert")
		tlsConfig := common.SetupTLSServerConfig(certPEM, keyPEM)
		tlsConfig.ClientCAs = common.SetupTLSLoadCertificate(agentCertPEM)
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
		logging.GetLogger().Errorf("Failed to Serve on %s:%d: %s", s.Addr, s.Port, err.Error())
	}
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Server.Shutdown(ctx); err != nil {
		logging.GetLogger().Error("Shutdown error :", err.Error())
	}
	s.listener.Close()
	s.wg.Wait()
}

func (s *Server) serveStatics(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if strings.HasPrefix(upath, "/") {
		upath = strings.TrimPrefix(upath, "/")
	}

	var content []byte
	var err error

	if asset, ok := s.extraAssets[upath]; ok {
		content = asset.Content
	} else {
		content, err = statics.Asset(upath)
		if err != nil {
			logging.GetLogger().Errorf("Unable to find the asset: %s", upath)
			w.WriteHeader(http.StatusNotFound)
			return
		}
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

	data := struct {
		ExtraAssets map[string]ExtraAsset
	}{
		ExtraAssets: s.extraAssets,
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	setTLSHeader(w, r)
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	tmpl := template.Must(template.New("index").Delims("<<", ">>").Parse(string(html)))
	if err := tmpl.Execute(w, data); err != nil {
		logging.GetLogger().Criticalf("Unable to execute index template: %s", err)
	}
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

func (s *Server) loadExtraAssets(folder string) {
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		logging.GetLogger().Errorf("Unable to load extra assets from %s: %s", folder, err)
		return
	}

	for _, file := range files {
		path := filepath.Join(folder, file.Name())

		data, err := ioutil.ReadFile(path)
		if err != nil {
			logging.GetLogger().Errorf("Unable to load extra asset %s: %s", path, err)
			return
		}

		ext := filepath.Ext(path)

		key := strings.TrimPrefix(filepath.Join(ExtraAssetPrefix, file.Name()), "/")
		s.extraAssets[key] = ExtraAsset{
			Filename: filepath.Join(ExtraAssetPrefix, file.Name()),
			Ext:      ext,
			Content:  data,
		}
	}
}

func NewServer(host string, serviceType common.ServiceType, addr string, port int, auth AuthenticationBackend, assetsFolder string) *Server {
	router := mux.NewRouter().StrictSlash(true)
	router.Headers("X-Host-ID", host, "X-Service-Type", serviceType.String())

	server := &Server{
		Host:        host,
		ServiceType: serviceType,
		Router:      router,
		Addr:        addr,
		Port:        port,
		Auth:        auth,
		extraAssets: make(map[string]ExtraAsset),
	}

	if assetsFolder != "" {
		server.loadExtraAssets(assetsFolder)
	}

	router.PathPrefix("/statics").HandlerFunc(server.serveStatics)
	router.PathPrefix(ExtraAssetPrefix).HandlerFunc(server.serveStatics)
	router.HandleFunc("/login", server.serveLogin)
	router.PathPrefix("/topology").HandlerFunc(server.serveIndex)
	router.HandleFunc("/", server.serveIndex)

	return server
}

func NewServerFromConfig(serviceType common.ServiceType) (*Server, error) {
	auth, err := NewAuthenticationBackendFromConfig()
	if err != nil {
		return nil, err
	}

	sa, err := common.ServiceAddressFromString(config.GetString(serviceType.String() + ".listen"))
	if err != nil {
		return nil, errors.New("Configuration error: " + err.Error())
	}

	host := config.GetString("host_id")
	assets := config.GetString("ui.extra_assets")

	return NewServer(host, serviceType, sa.Addr, sa.Port, auth, assets), nil
}
