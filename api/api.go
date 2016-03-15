/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	etcd "github.com/coreos/etcd/client"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/rpc"
	"github.com/redhat-cip/skydive/topology/graph"
)

type ApiServer struct {
	Router     *mux.Router
	etcdKeyAPI etcd.KeysAPI
}

type ApiResource struct {
	UUID *graph.UUID
}

type HandlerFunc func(w http.ResponseWriter, r *http.Request)

type apiResourceCRUD interface {
	Name() string
	New() interface{}
	Index() map[string]interface{}
	Get(id string) (interface{}, bool)
	Create(resource interface{}) error
	Delete(id string) error
}

func (a *ApiServer) RegisterResource(handler apiResourceCRUD) error {
	name := handler.Name()
	title := strings.Title(name)

	routes := []rpc.Route{
		{
			title + "Index",
			"GET",
			"/rpc/" + name,
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusOK)

				resources := handler.Index()
				if err := json.NewEncoder(w).Encode(resources); err != nil {
					logging.GetLogger().Criticalf("Failed to display %s: %s", name, err.Error())
				}
			},
		},
		{
			title + "Show",
			"GET",
			rpc.PathPrefix(fmt.Sprintf("/rpc/%s/", name)),
			func(w http.ResponseWriter, r *http.Request) {
				id := r.URL.Path[len(fmt.Sprintf("/rpc/%s/", name)):]
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
				if err := json.NewEncoder(w).Encode(resource); err != nil {
					logging.GetLogger().Criticalf("Failed to display %s: %s", name, err.Error())
				}
			},
		},
		{
			title + "Insert",
			"POST",
			"/rpc/" + name,
			func(w http.ResponseWriter, r *http.Request) {
				resource := handler.New()
				data, _ := ioutil.ReadAll(r.Body)
				if err := json.Unmarshal(data, &resource); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				if err := handler.Create(resource); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				data, err := json.Marshal(&resource)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write(data); err != nil {
					logging.GetLogger().Criticalf("Failed to create %s: %s", name, err.Error())
				}
			},
		},
		{
			title + "Delete",
			"DELETE",
			rpc.PathPrefix(fmt.Sprintf("/rpc/%s/", name)),
			func(w http.ResponseWriter, r *http.Request) {
				id := r.URL.Path[len(fmt.Sprintf("/rpc/%s/", name)):]
				if id == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				if err := handler.Delete(id); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	rpc.RegisterRoutes(a.Router, routes)

	_, err := a.etcdKeyAPI.Set(context.Background(), "/"+name, "", &etcd.SetOptions{Dir: true})
	return err
}

func NewApi(router *mux.Router, kapi etcd.KeysAPI) (*ApiServer, error) {
	server := &ApiServer{
		Router:     router,
		etcdKeyAPI: kapi,
	}

	server.RegisterResource(&CaptureHandler{etcdKeyAPI: kapi})
	return server, nil
}
