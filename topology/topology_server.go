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
	"net/http"

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/statics"
)

type TopologyServer struct {
	Topology *Topology
}

func (t *TopologyServer) htmlOutput(w http.ResponseWriter, r *http.Request) {
	data, err := statics.Asset("statics/topology.html")
	if err != nil {
		logging.GetLogger().Panic("Unable to find the topology asset")
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (t *TopologyServer) jsonOutput(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(t.Topology); err != nil {
		panic(err)
	}
}

func RegisterTopologyServerEndpoints(topo *Topology, router *mux.Router) {
	server := &TopologyServer{
		Topology: topo,
	}

	router.HandleFunc("/topology.json", server.jsonOutput)
	router.HandleFunc("/topology.html", server.htmlOutput)
}
