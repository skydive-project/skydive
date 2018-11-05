/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package server

import (
	"fmt"

	"gopkg.in/yaml.v2"

	"github.com/skydive-project/skydive/api/types"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/statics"
)

const workflowAssetDir = "statics/workflows"

// WorkflowResourceHandler describes a workflow resource handler
type WorkflowResourceHandler struct {
}

// WorkflowAPIHandler based on BasicAPIHandler
type WorkflowAPIHandler struct {
	BasicAPIHandler
}

// New creates a new workflow resource
func (w *WorkflowResourceHandler) New() types.Resource {
	return &types.Workflow{}
}

// Name return "workflow"
func (w *WorkflowResourceHandler) Name() string {
	return "workflow"
}

// Create tests whether the resource is a duplicate or is unique
func (w *WorkflowAPIHandler) Create(r types.Resource) error {
	workflow := r.(*types.Workflow)
	for _, resource := range w.Index() {
		w := resource.(*types.Workflow)
		if w.Name == workflow.Name {
			return fmt.Errorf("Duplicate workflow, name=%s", w.Name)
		}
	}

	return w.BasicAPIHandler.Create(workflow)
}

func (w *WorkflowAPIHandler) loadWorkflowAsset(name string) (*types.Workflow, error) {
	yml, err := statics.Asset(name)
	if err != nil {
		return nil, err
	}

	var workflow types.Workflow
	if err := yaml.Unmarshal([]byte(yml), &workflow); err != nil {
		return nil, err
	}

	return &workflow, nil
}

// Get retrieves a workflow based on its id
func (w *WorkflowAPIHandler) Get(id string) (types.Resource, bool) {
	workflows := w.Index()
	workflow, found := workflows[id]
	if !found {
		return nil, false
	}
	return workflow.(*types.Workflow), true
}

// Index returns a map of workflows indexed by id
func (w *WorkflowAPIHandler) Index() map[string]types.Resource {
	resources := w.BasicAPIHandler.Index()
	assets, err := statics.AssetDir(workflowAssetDir)
	if err == nil {
		for _, asset := range assets {
			workflow, err := w.loadWorkflowAsset(workflowAssetDir + "/" + asset)
			if err != nil {
				logging.GetLogger().Errorf("Failed to load worklow asset %s: %s", asset, err)
				continue
			}
			resources[workflow.ID()] = workflow
		}
	}
	return resources
}

// RegisterWorkflowAPI registers a new workflow api handler
func RegisterWorkflowAPI(apiServer *Server, authBackend shttp.AuthenticationBackend) (*WorkflowAPIHandler, error) {
	workflowAPIHandler := &WorkflowAPIHandler{
		BasicAPIHandler: BasicAPIHandler{
			ResourceHandler: &WorkflowResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
	}
	if err := apiServer.RegisterAPIHandler(workflowAPIHandler, authBackend); err != nil {
		return nil, err
	}
	return workflowAPIHandler, nil
}
