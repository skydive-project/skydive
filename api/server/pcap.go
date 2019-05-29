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
	Storage storage.Storage
}

func (p *PcapAPI) flowExpireUpdate(flowArray *flow.FlowArray) {
	if p.Storage != nil && len(flowArray.Flows) > 0 {
		p.Storage.StoreFlows(flowArray.Flows)
		logging.GetLogger().Debugf("%d flows stored", len(flowArray.Flows))
	}
}

func (p *PcapAPI) injectPcap(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	update := config.GetInt("flow.update")
	expire := config.GetInt("flow.expire")

	if !rbac.Enforce(r.Username, "pcap", "write") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	updateHandler := flow.NewFlowHandler(p.flowExpireUpdate, time.Second*time.Duration(update))
	expireHandler := flow.NewFlowHandler(p.flowExpireUpdate, time.Second*time.Duration(expire))

	flowtable := flow.NewTable(updateHandler, expireHandler, "", flow.TableOpts{})
	packetSeqChan, _, _ := flowtable.Start()

	feeder, err := flow.NewPcapTableFeeder(r.Body, packetSeqChan, false, "")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	feeder.Start()
	feeder.Wait()

	// stop/flush flowtable
	flowtable.Stop()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
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
	p := &PcapAPI{
		Storage: store,
	}

	p.registerEndpoints(r, authBackend)
}
