/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package server Skydive API
//
// The Skydive REST API allows to communicate with a Skydive analyzer.
//
//     Schemes: http, https
//     Host: localhost:8082
//     BasePath: /api
//     Version: 0.26.0
//     License: Apache http://opensource.org/licenses/Apache-2.0
//     Contact: Skydive mailing list <skydive-dev@redhat.com>
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//     - text/plain
//
// swagger:meta
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	auth "github.com/abbot/go-http-auth"
	etcd "github.com/coreos/etcd/client"
	yaml "gopkg.in/yaml.v2"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/rbac"
	"github.com/skydive-project/skydive/validator"
)

// Server defines an API server
type Server struct {
	HTTPServer *shttp.Server
	EtcdKeyAPI etcd.KeysAPI
	handlers   map[string]rest.Handler
}

// Info for each host describes his API version and service
// swagger:model
type Info struct {
	// Server host ID
	Host string
	// API version
	Version string
	// Service type
	Service string
}

// RegisterAPIHandler registers a new handler for an API
func (a *Server) RegisterAPIHandler(handler rest.Handler, authBackend shttp.AuthenticationBackend) error {
	name := handler.Name()
	title := strings.Title(name)

	routes := []shttp.Route{
		{
			Name:   title + "Index",
			Method: "GET",
			Path:   "/api/" + name,
			HandlerFunc: func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
				if !rbac.Enforce(r.Username, name, "read") {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}

				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusOK)

				resources := handler.Index()
				for _, resource := range resources {
					handler.Decorate(resource)
				}

				if err := json.NewEncoder(w).Encode(resources); err != nil {
					logging.GetLogger().Criticalf("Failed to display %s: %s", name, err)
				}
			},
		},
		{
			Name:   title + "Show",
			Method: "GET",
			Path:   shttp.PathPrefix(fmt.Sprintf("/api/%s/", name)),
			HandlerFunc: func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
				if !rbac.Enforce(r.Username, name, "read") {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}

				id := r.URL.Path[len(fmt.Sprintf("/api/%s/", name)):]
				if id == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				resource, ok := handler.Get(id)
				if !ok {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusOK)
				handler.Decorate(resource)
				if err := json.NewEncoder(w).Encode(resource); err != nil {
					logging.GetLogger().Criticalf("Failed to display %s: %s", name, err)
				}
			},
		},
		{
			Name:   title + "Insert",
			Method: "POST",
			Path:   "/api/" + name,
			HandlerFunc: func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
				if !rbac.Enforce(r.Username, name, "write") {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}

				resource := handler.New()

				var err error
				if contentType := r.Header.Get("Content-Type"); contentType == "application/yaml" {
					if content, e := ioutil.ReadAll(r.Body); e == nil {
						err = yaml.Unmarshal(content, resource)
					} else {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
				} else {
					decoder := json.NewDecoder(r.Body)
					err = decoder.Decode(&resource)
				}
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				if err := validator.Validate(resource); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				var createOpts rest.CreateOptions
				if ttlHeader := r.Header.Get("X-Resource-TTL"); ttlHeader != "" {
					if createOpts.TTL, err = time.ParseDuration(ttlHeader); err != nil {
						http.Error(w, fmt.Sprintf("invalid ttl: %s", err), http.StatusBadRequest)
						return
					}
				}

				if err := handler.Create(resource, &createOpts); err == rest.ErrDuplicatedResource {
					http.Error(w, err.Error(), http.StatusConflict)
					return
				} else if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				data, err := json.Marshal(&resource)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusCreated)
				if _, err := w.Write(data); err != nil {
					logging.GetLogger().Criticalf("Failed to create %s: %s", name, err)
				}
			},
		},
		{
			Name:   title + "Delete",
			Method: "DELETE",
			Path:   shttp.PathPrefix(fmt.Sprintf("/api/%s/", name)),
			HandlerFunc: func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
				if !rbac.Enforce(r.Username, name, "write") {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}

				id := r.URL.Path[len(fmt.Sprintf("/api/%s/", name)):]
				if id == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				if err := handler.Delete(id); err != nil {
					if err, ok := err.(etcd.Error); ok && err.Code == etcd.ErrorCodeKeyNotFound {
						http.Error(w, err.Error(), http.StatusNotFound)
					} else {
						http.Error(w, err.Error(), http.StatusBadRequest)
					}
					return
				}

				w.WriteHeader(http.StatusOK)
			},
		},
	}

	a.HTTPServer.RegisterRoutes(routes, authBackend)

	a.handlers[handler.Name()] = handler

	return nil
}

func (a *Server) addAPIRootRoute(version string, service common.Service, authBackend shttp.AuthenticationBackend) {
	// swagger:operation GET / getApi
	//
	// Get API version
	//
	// ---
	// summary: Get API info
	//
	// tags:
	// - API Info
	//
	// consumes:
	// - application/json
	//
	// produces:
	// - application/json
	//
	// schemes:
	// - http
	// - https
	//
	// responses:
	//   200:
	//     description: API info
	//     schema:
	//       $ref: '#/definitions/Info'

	info := Info{
		Version: version,
		Service: string(service.Type),
		Host:    service.ID,
	}

	routes := []shttp.Route{
		{
			Name:   "Skydive API",
			Method: "GET",
			Path:   "/api",
			HandlerFunc: func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusOK)

				if err := json.NewEncoder(w).Encode(&info); err != nil {
					logging.GetLogger().Criticalf("Failed to display /api: %s", err)
				}
			},
		}}

	a.HTTPServer.RegisterRoutes(routes, authBackend)
}

// GetHandler returns the hander named hname
func (a *Server) GetHandler(hname string) rest.Handler {
	return a.handlers[hname]
}

func (a *Server) serveLogin(w http.ResponseWriter, r *http.Request, authBackend shttp.AuthenticationBackend) {
	shttp.SetTLSHeader(w, r)
	if r.Method == "POST" {
		r.ParseForm()
		loginForm, passwordForm := r.Form["username"], r.Form["password"]
		if len(loginForm) != 0 && len(passwordForm) != 0 {
			username, password := loginForm[0], passwordForm[0]

			token, permissions, err := shttp.Authenticate(authBackend, w, username, password)
			if err == nil {
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")

				w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.WriteHeader(http.StatusOK)

				body := struct {
					Token       string
					Permissions []rbac.Permission
				}{
					Token:       token,
					Permissions: permissions,
				}

				bytes, _ := json.Marshal(body)
				w.Write(bytes)

				roles := rbac.GetUserRoles(username)
				logging.GetLogger().Infof("User %s authenticated with %s backend with roles %s", username, authBackend.Name(), roles)
				return
			}

			shttp.Unauthorized(w, r, err)
		} else {
			shttp.Unauthorized(w, r, errors.New("No credentials provided"))
		}
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func (a *Server) serveLoginHandlerFunc(authBackend shttp.AuthenticationBackend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.serveLogin(w, r, authBackend)
	}
}

func (a *Server) addLoginRoute(authBackend shttp.AuthenticationBackend) {
	// swagger:operation POST /login login
	//
	// Login
	//
	// ---
	// summary: Login
	//
	// tags:
	// - Login
	//
	// consumes:
	// - application/x-www-form-urlencoded
	//
	// schemes:
	// - http
	// - https
	//
	// security: []
	//
	// parameters:
	// - name: username
	//   in: formData
	//   required: true
	//   type: string
	// - name: password
	//   in: formData
	//   required: true
	//   type: string
	//
	// responses:
	//   200:
	//     description: Authentication successful
	//   401:
	//     description: Unauthorized

	a.HTTPServer.Router.HandleFunc("/login", a.serveLoginHandlerFunc(authBackend))
}

// NewAPI creates a new API server based on http
func NewAPI(server *shttp.Server, kapi etcd.KeysAPI, version string, service common.Service, authBackend shttp.AuthenticationBackend) (*Server, error) {
	if version == "" {
		version = "unknown"
	}

	apiServer := &Server{
		HTTPServer: server,
		EtcdKeyAPI: kapi,
		handlers:   make(map[string]rest.Handler),
	}

	apiServer.addAPIRootRoute(version, service, authBackend)
	apiServer.addLoginRoute(authBackend)

	return apiServer, nil
}
