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
	"fmt"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"

	"github.com/skydive-project/skydive/api/types"
	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/js"
	"github.com/skydive-project/skydive/rbac"
)

// WorkflowCallAPIHandler based on BasicAPIHandler
type WorkflowCallAPIHandler struct {
	apiServer *api.Server
	graph     *graph.Graph
	parser    *traversal.GremlinTraversalParser
	runtime   *js.Runtime
}

func (wc *WorkflowCallAPIHandler) executeWorkflow(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	if !rbac.Enforce(r.Username, "workflow.call", "write") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	decoder := json.NewDecoder(r.Body)
	var wfCall types.WorkflowCall
	if err := decoder.Decode(&wfCall); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	vars := mux.Vars(&r.Request)

	workflow, err := wc.getWorkflow(vars["ID"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ottoResult, err := wc.runtime.ExecFunction(workflow.Source, wfCall.Params...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := ottoResult.Export()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		panic(err)
	}
}

func (wc *WorkflowCallAPIHandler) getWorkflow(id string) (*types.Workflow, error) {
	handler := wc.apiServer.GetHandler("workflow")
	workflow, ok := handler.Get(id)
	if !ok {
		return nil, fmt.Errorf("No workflow found with ID: %s", id)
	}
	return workflow.(*types.Workflow), nil
}

func (wc *WorkflowCallAPIHandler) registerEndPoints(s *shttp.Server, authBackend shttp.AuthenticationBackend) {
	// swagger:operation POST /workflow/{id}/call callWorkflow
	//
	// Call workflow
	//
	// ---
	// summary: Call workflow
	//
	// tags:
	// - Workflows
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
	// parameters:
	//   - name: id
	//     in: path
	//     required: true
	//     type: string
	//
	//   - name: params
	//     in: body
	//     required: true
	//     schema:
	//       $ref: '#/definitions/WorkflowCall'
	//
	// responses:
	//   202:
	//     description: Request accepted
	//     schema:
	//       $ref: '#/definitions/AnyValue'
	//
	//   400:
	//     description: Invalid PCAP

	routes := []shttp.Route{
		{
			Name:        "WorkflowCall",
			Method:      "POST",
			Path:        "/api/workflow/{ID}/call",
			HandlerFunc: wc.executeWorkflow,
		},
	}

	s.RegisterRoutes(routes, authBackend)
}

// RegisterWorkflowCallAPI registers a new workflow  call api handler
func RegisterWorkflowCallAPI(s *shttp.Server, authBackend shttp.AuthenticationBackend, apiServer *api.Server, g *graph.Graph, tr *traversal.GremlinTraversalParser) error {
	runtime, err := NewWorkflowRuntime(g, tr, apiServer)
	if err != nil {
		return err
	}

	workflowCallAPIHandler := &WorkflowCallAPIHandler{
		apiServer: apiServer,
		graph:     g,
		parser:    tr,
		runtime:   runtime,
	}
	workflowCallAPIHandler.registerEndPoints(s, authBackend)

	return nil
}
