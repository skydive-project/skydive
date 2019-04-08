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
	"strings"

	"github.com/skydive-project/skydive/js"
	"github.com/skydive-project/skydive/rbac"

	auth "github.com/abbot/go-http-auth"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	shttp "github.com/skydive-project/skydive/http"
)

// WorkflowCallAPIHandler based on BasicAPIHandler
type WorkflowCallAPIHandler struct {
	apiServer *Server
	graph     *graph.Graph
	parser    *traversal.GremlinTraversalParser
}

func (wc *WorkflowCallAPIHandler) executeWorkflow(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	if !rbac.Enforce(r.Username, "workflow.call", "write") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	uriSegments := strings.Split(r.URL.Path, "/")
	decoder := json.NewDecoder(r.Body)
	var wfCall types.WorkflowCall
	if err := decoder.Decode(&wfCall); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	workflow, err := wc.getWorkflow(uriSegments[3])
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	runtime, err := js.NewRuntime()
	if err != nil {
		writeError(w, http.StatusFailedDependency, err)
		return
	}

	runtime.Start()
	RegisterAPIServer(runtime, wc.graph, wc.parser, wc.apiServer)
	ottoResult, err := runtime.ExecPromise(workflow.Source, wfCall.Params...)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result, err := ottoResult.Export()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
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
	routes := []shttp.Route{
		{
			Name:        "WorkflowCall",
			Method:      "POST",
			Path:        "/api/workflow/{workflowID}/call",
			HandlerFunc: wc.executeWorkflow,
		},
	}

	s.RegisterRoutes(routes, authBackend)
}

// RegisterWorkflowCallAPI registers a new workflow  call api handler
func RegisterWorkflowCallAPI(s *shttp.Server, authBackend shttp.AuthenticationBackend, apiServer *Server, g *graph.Graph, tr *traversal.GremlinTraversalParser) {
	workflowCallAPIHandler := &WorkflowCallAPIHandler{
		apiServer: apiServer,
		graph:     g,
		parser:    tr,
	}
	workflowCallAPIHandler.registerEndPoints(s, authBackend)
}
