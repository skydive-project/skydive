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
	"html/template"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"os"
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
	"github.com/skydive-project/skydive/rbac"
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
	extraAssets map[string]ExtraAsset
	globalVars  map[string]interface{}
}

func copyRequestVars(old, new *http.Request) {
	kv := gcontext.GetAll(old)
	for k, v := range kv {
		gcontext.Set(new, k, v)
	}
}

func getRequestParameter(r *http.Request, name string) string {
	param := r.Header.Get(name)
	if param == "" {
		param = r.URL.Query().Get(strings.ToLower(name))
	}
	return param
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

func (s *Server) RegisterLoginRoute(authBackend AuthenticationBackend) {
	s.Router.HandleFunc("/login", s.serveLoginHandlerFunc(authBackend))
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

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Server.Shutdown(ctx); err != nil {
		logging.GetLogger().Error("Shutdown error :", err)
	}
	s.listener.Close()
	s.wg.Wait()
}

func (s *Server) readStatics(upath string) (content []byte, err error) {
	if asset, ok := s.extraAssets[upath]; ok {
		logging.GetLogger().Debugf("Fetch disk asset: %s", upath)
		content = asset.Content
	} else if content, err = statics.Asset(upath); err != nil {
		logging.GetLogger().Debugf("Fetch embedded asset: %s", upath)
	}
	return
}

func (s *Server) serveStatics(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if strings.HasPrefix(upath, "/") {
		upath = strings.TrimPrefix(upath, "/")
	}

	content, err := s.readStatics(upath)

	if err != nil {
		logging.GetLogger().Errorf("Unable to find the asset %s", upath)
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

// ServeIndex servers the index page
func (s *Server) ServeIndex(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	html, err := s.readStatics("statics/index.html")
	if err != nil {
		logging.GetLogger().Error("Unable to find the asset index.html")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	username := r.Username
	if username == "" {
		username = "admin"
	}

	s.RLock()
	defer s.RUnlock()

	data := struct {
		ExtraAssets map[string]ExtraAsset
		GlobalVars  interface{}
		Permissions []rbac.Permission
	}{
		ExtraAssets: s.extraAssets,
		GlobalVars:  s.globalVars,
		Permissions: rbac.GetPermissionsForUser(username),
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	setTLSHeader(w, &r.Request)
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	tmpl := template.Must(template.New("index").Delims("<<", ">>").Parse(string(html)))
	if err := tmpl.Execute(w, data); err != nil {
		logging.GetLogger().Criticalf("Unable to execute index template: %s", err)
	}
}

func (s *Server) serveLogin(w http.ResponseWriter, r *http.Request, authBackend AuthenticationBackend) {
	setTLSHeader(w, r)
	if r.Method == "POST" {
		r.ParseForm()
		loginForm, passwordForm := r.Form["username"], r.Form["password"]
		if len(loginForm) != 0 && len(passwordForm) != 0 {
			username, password := loginForm[0], passwordForm[0]

			if _, err := authenticate(authBackend, w, username, password); err == nil {
				w.WriteHeader(http.StatusOK)

				roles := rbac.GetUserRoles(username)
				logging.GetLogger().Infof("User %s authenticated with %s backend with roles %s", username, authBackend.Name(), roles)
				return
			}

			unauthorized(w, r)
		} else {
			unauthorized(w, r)
		}
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func (s *Server) serveLoginHandlerFunc(authBackend AuthenticationBackend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.serveLogin(w, r, authBackend)
	}
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("401 Unauthorized\n"))
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
		setTLSHeader(w, r)

		preAuthHandler(w, r)
	})
}

func (s *Server) loadExtraAssets(folder, prefix string) {
	files := []string{}

	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		files = append(files, strings.TrimPrefix(path, folder))
		return nil
	})

	if err != nil {
		logging.GetLogger().Errorf("Unable to load extra assets from %s: %s", folder, err)
		return
	}

	for _, file := range files {
		path := filepath.Join(folder, file)

		data, err := ioutil.ReadFile(path)
		if err != nil {
			logging.GetLogger().Errorf("Unable to load extra asset %s: %s", path, err)
			return
		}

		ext := filepath.Ext(path)

		key := strings.TrimPrefix(filepath.Join(prefix, file), "/")
		logging.GetLogger().Debugf("Added extra static assert: %s", key)
		s.extraAssets[key] = ExtraAsset{
			Filename: filepath.Join(prefix, file),
			Ext:      ext,
			Content:  data,
		}
	}
}

func (s *Server) AddGlobalVar(key string, v interface{}) {
	s.Lock()
	s.globalVars[key] = v
	s.Unlock()
}

func NewServer(host string, serviceType common.ServiceType, addr string, port int, assetsFolder string) *Server {
	router := mux.NewRouter().StrictSlash(true)
	router.Headers("X-Host-ID", host, "X-Service-Type", serviceType.String())

	server := &Server{
		Host:        host,
		ServiceType: serviceType,
		Router:      router,
		Addr:        addr,
		Port:        port,
		extraAssets: make(map[string]ExtraAsset),
		globalVars:  make(map[string]interface{}),
	}

	if assetsFolder != "" {
		server.loadExtraAssets(assetsFolder, ExtraAssetPrefix)
	}

	router.PathPrefix("/statics").HandlerFunc(server.serveStatics)
	router.PathPrefix(ExtraAssetPrefix).HandlerFunc(server.serveStatics)
	router.HandleFunc("/", NoAuthenticationWrap(server.ServeIndex))

	return server
}

func NewServerFromConfig(serviceType common.ServiceType) (*Server, error) {
	sa, err := common.ServiceAddressFromString(config.GetString(serviceType.String() + ".listen"))
	if err != nil {
		return nil, fmt.Errorf("Configuration error: %s", err)
	}

	host := config.GetString("host_id")
	assets := config.GetString("ui.extra_assets")

	return NewServer(host, serviceType, sa.Addr, sa.Port, assets), nil
}
