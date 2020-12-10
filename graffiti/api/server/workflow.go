//go:generate sh -c "go run github.com/gomatic/renderizer --name=workflow --resource=workflow --type=Workflow --title=Workflow --article=a ../../../api/server/swagger_operations.tmpl > workflow_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name=workflow --resource=workflow --type=Workflow --title=Workflow --article=a ../../../api/server/swagger_definitions.tmpl > workflow_swagger.json"

/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/api/types"
	"github.com/skydive-project/skydive/graffiti/assets"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/js"
	"github.com/skydive-project/skydive/graffiti/rbac"
)

// StaticWorkflows holds a list of hardcoded workflows
var StaticWorkflows []*types.Workflow

// WorkflowResourceHandler describes a workflow resource handler
type WorkflowResourceHandler struct {
}

// WorkflowAPIHandler based on BasicAPIHandler
type WorkflowAPIHandler struct {
	rest.BasicAPIHandler
	apiServer *Server
	graph     *graph.Graph
	parser    *traversal.GremlinTraversalParser
	runtime   *js.Runtime
}

// New creates a new workflow resource
func (w *WorkflowResourceHandler) New() rest.Resource {
	return &types.Workflow{}
}

// Name return "workflow"
func (w *WorkflowResourceHandler) Name() string {
	return "workflow"
}

// Create tests whether the resource is a duplicate or is unique
func (w *WorkflowAPIHandler) Create(r rest.Resource, opts *rest.CreateOptions) error {
	workflow := r.(*types.Workflow)

	for _, resource := range w.Index() {
		w := resource.(*types.Workflow)
		if w.Name == workflow.Name {
			return fmt.Errorf("Duplicate workflow, name=%s", w.Name)
		}
	}

	return w.BasicAPIHandler.Create(workflow, opts)
}

// Get retrieves a workflow based on its id
func (w *WorkflowAPIHandler) Get(id string) (rest.Resource, bool) {
	workflows := w.Index()
	workflow, found := workflows[id]
	if !found {
		return nil, false
	}
	return workflow.(*types.Workflow), true
}

// Index returns a map of workflows indexed by id
func (w *WorkflowAPIHandler) Index() map[string]rest.Resource {
	resources := w.BasicAPIHandler.Index()
	for _, workflow := range StaticWorkflows {
		resources[workflow.GetID()] = workflow
	}
	return resources
}

func (w *WorkflowAPIHandler) getWorkflow(id string) (*types.Workflow, error) {
	handler := w.apiServer.GetHandler("workflow")
	workflow, ok := handler.Get(id)
	if !ok {
		return nil, fmt.Errorf("No workflow found with ID: %s", id)
	}
	return workflow.(*types.Workflow), nil
}

func (w *WorkflowAPIHandler) executeWorkflow(resp http.ResponseWriter, r *auth.AuthenticatedRequest) {
	if !rbac.Enforce(r.Username, "workflow.call", "write") {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	decoder := json.NewDecoder(r.Body)
	var wfCall types.WorkflowCall
	if err := decoder.Decode(&wfCall); err != nil {
		http.Error(resp, err.Error(), http.StatusBadRequest)
		return
	}

	vars := mux.Vars(&r.Request)

	workflow, err := w.getWorkflow(vars["ID"])
	if err != nil {
		http.Error(resp, err.Error(), http.StatusBadRequest)
		return
	}

	ottoResult, err := w.runtime.ExecFunction(workflow.Source, wfCall.Params...)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := ottoResult.Export()
	if err != nil {
		http.Error(resp, err.Error(), http.StatusBadRequest)
		return
	}
	resp.Header().Set("Content-Type", "application/json; charset=UTF-8")
	resp.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(resp).Encode(result); err != nil {
		panic(err)
	}
}

func (w *WorkflowAPIHandler) registerEndPoints(s *shttp.Server, authBackend shttp.AuthenticationBackend) {
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
			HandlerFunc: w.executeWorkflow,
		},
	}

	s.RegisterRoutes(routes, authBackend)
}

// RegisterWorkflowAPI registers a new workflow api handler
func RegisterWorkflowAPI(apiServer *Server, g *graph.Graph, parser *traversal.GremlinTraversalParser, assets assets.Assets, authBackend shttp.AuthenticationBackend) (*WorkflowAPIHandler, error) {
	runtime, err := NewWorkflowRuntime(g, parser, apiServer, assets)
	if err != nil {
		return nil, err
	}

	workflowAPIHandler := &WorkflowAPIHandler{
		BasicAPIHandler: rest.BasicAPIHandler{
			ResourceHandler: &WorkflowResourceHandler{},
			EtcdClient:      apiServer.EtcdClient,
		},
		apiServer: apiServer,
		graph:     g,
		parser:    parser,
		runtime:   runtime,
	}

	apiServer.RegisterAPIHandler(workflowAPIHandler, authBackend)
	workflowAPIHandler.registerEndPoints(apiServer.HTTPServer, authBackend)

	return workflowAPIHandler, nil
}
