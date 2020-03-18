/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package server

import (
	"net/http"
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/storage"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/rbac"
)

// PcapAPI exposes the pcap injector API
type PcapAPI struct {
	storage storage.Storage
}

// SendFlows implements the flow Sender interface
func (p *PcapAPI) SendFlows(flows []*flow.Flow) {
	if p.storage != nil && len(flows) > 0 {
		p.storage.StoreFlows(flows)
		logging.GetLogger().Debugf("%d flows stored", len(flows))
	}
}

// SendStats implements the flow Sender interface
func (p *PcapAPI) SendStats(status flow.Stats) {
}

func (p *PcapAPI) injectPcap(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	updateEvery := time.Duration(config.GetInt("flow.update")) * time.Second
	expireAfter := time.Duration(config.GetInt("flow.expire")) * time.Second

	if !rbac.Enforce(r.Username, "pcap", "write") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	flowtable := flow.NewTable(updateEvery, expireAfter, p, flow.UUIDs{}, flow.TableOpts{})
	packetSeqChan, _, _ := flowtable.Start(nil)

	feeder, err := flow.NewPcapTableFeeder(r.Body, packetSeqChan, false, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	feeder.Start()
	feeder.Wait()

	// stop/flush flowtable
	flowtable.Stop()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusAccepted)
}

func (p *PcapAPI) registerEndpoints(r *shttp.Server, authBackend shttp.AuthenticationBackend) {
	routes := []shttp.Route{
		{
			Name:        "PCAP",
			Method:      "POST",
			Path:        "/api/pcap",
			HandlerFunc: p.injectPcap,
		},
	}

	r.RegisterRoutes(routes, authBackend)
}

// RegisterPcapAPI registers a new pcap injector API
func RegisterPcapAPI(r *shttp.Server, store storage.Storage, authBackend shttp.AuthenticationBackend) {
	// swagger:operation POST /pcap injectPCAP
	//
	// Inject PCAP
	//
	// ---
	// summary: Inject PCAP
	//
	// tags:
	// - PCAP
	//
	// consumes:
	// - application/octet-stream
	//
	// schemes:
	// - http
	// - https
	//
	// parameters:
	//   - in: body
	//     name: status
	//     required: true
	//     schema:
	//       type: string
	//       format: binary
	//
	// responses:
	//   202:
	//     description: request accepted
	//   400:
	//     description: invalid PCAP

	p := &PcapAPI{
		storage: store,
	}

	p.registerEndpoints(r, authBackend)
}
