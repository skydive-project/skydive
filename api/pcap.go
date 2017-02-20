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
	"net/http"

	"github.com/abbot/go-http-auth"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
)

type PcapAPI struct {
	packetsChan chan *flow.FlowPackets
}

func (p *PcapAPI) injectPcap(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	writer, err := flow.NewPcapWriter(r.Body, p.packetsChan, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writer.Start()
	writer.Wait()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
}

func (p *PcapAPI) registerEndpoints(r *shttp.Server) {
	routes := []shttp.Route{
		{
			Name:        "PCAP",
			Method:      "POST",
			Path:        "/api/pcap",
			HandlerFunc: p.injectPcap,
		},
	}

	r.RegisterRoutes(routes)
}

func RegisterPcapAPI(r *shttp.Server, packetsChan chan *flow.FlowPackets) {
	p := &PcapAPI{packetsChan: packetsChan}

	p.registerEndpoints(r)
}
