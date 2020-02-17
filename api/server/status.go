/*
 * Copyright (C) 2019 Red Hat, Inc.
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
	"encoding/json"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/rbac"
)

// StatusReporter is the interface to report the status of a service
type StatusReporter interface {
	GetStatus() interface{}
}

type statusAPI struct {
	reporter StatusReporter
}

func (s *statusAPI) statusGet(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	if !rbac.Enforce(r.Username, "status", "read") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	status := s.reporter.GetStatus()
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		logging.GetLogger().Warningf("Error while writing response: %s", err)
	}
}

func (s *statusAPI) registerEndpoints(r *shttp.Server, authBackend shttp.AuthenticationBackend) {
	// swagger:operation GET /status getStatus
	//
	// Get status
	//
	// ---
	// summary: Get status
	//
	// tags:
	// - Status
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
	//     description: Status
	//     content:
	//       application/json:
	//         schema:
	//           anyOf:
	//           - $ref: '#/definitions/AgentStatus'
	//           - $ref: '#/definitions/AnalyzerStatus'

	routes := []shttp.Route{
		{
			Name:        "StatusGet",
			Method:      "GET",
			Path:        "/api/status",
			HandlerFunc: s.statusGet,
		},
	}

	r.RegisterRoutes(routes, authBackend)
}

// RegisterStatusAPI registers the status API endpoint
func RegisterStatusAPI(s *shttp.Server, r StatusReporter, authBackend shttp.AuthenticationBackend) {
	a := &statusAPI{
		reporter: r,
	}

	a.registerEndpoints(s, authBackend)
}
