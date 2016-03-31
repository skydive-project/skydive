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
	"net/http"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"
	"github.com/redhat-cip/skydive/flow"
	shttp "github.com/redhat-cip/skydive/http"
	"github.com/redhat-cip/skydive/storage"
)

type FlowApi struct {
	Service   string
	FlowTable *flow.FlowTable
	Storage   storage.Storage
}

func (f *FlowApi) FlowSearch(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	filters := make(storage.Filters)
	for k, v := range r.URL.Query() {
		filters[k] = v[0]
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if f.Storage == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	flows, err := f.Storage.SearchFlows(filters)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(flows); err != nil {
		panic(err)
	}
}

func (f *FlowApi) serveDataIndex(w http.ResponseWriter, r *auth.AuthenticatedRequest, message string) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}

func (f *FlowApi) ConversationLayer(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	vars := mux.Vars(&r.Request)
	layer := vars["layer"]

	ltype := flow.FlowEndpointType_ETHERNET
	switch layer {
	case "ethernet":
		ltype = flow.FlowEndpointType_ETHERNET
	case "ipv4":
		ltype = flow.FlowEndpointType_IPV4
	case "tcp":
		ltype = flow.FlowEndpointType_TCPPORT
	case "udp":
		ltype = flow.FlowEndpointType_UDPPORT
	case "sctp":
		ltype = flow.FlowEndpointType_SCTPPORT
	}
	f.serveDataIndex(w, r, f.FlowTable.JSONFlowConversationEthernetPath(ltype))
}

func (f *FlowApi) DiscoveryType(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	vars := mux.Vars(&r.Request)
	discoType := vars["type"]
	dtype := flow.BYTES
	switch discoType {
	case "bytes":
		dtype = flow.BYTES
	case "packets":
		dtype = flow.PACKETS
	}
	f.serveDataIndex(w, r, f.FlowTable.JSONFlowDiscovery(dtype))
}
func (f *FlowApi) registerEndpoints(r *shttp.Server) {
	routes := []shttp.Route{
		{
			"FlowSearch",
			"GET",
			"/api/flow/search",
			f.FlowSearch,
		},
		{
			"ConversationLayer",
			"GET",
			"/api/flow/conversation/{layer}",
			f.ConversationLayer,
		},
		{
			"Discovery",
			"GET",
			"/api/flow/discovery/{type}",
			f.DiscoveryType,
		},
	}

	r.RegisterRoutes(routes)
}

func RegisterFlowApi(s string, f *flow.FlowTable, st storage.Storage, r *shttp.Server) {
	fa := &FlowApi{
		Service:   s,
		FlowTable: f,
		Storage:   st,
	}

	fa.registerEndpoints(r)
}
