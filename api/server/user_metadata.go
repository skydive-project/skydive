/*
 * Copyright (C) 2017 Red Hat, Inc.
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

	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/topology/graph"
)

//UserMetadataResourceHandler describes a user metadata resource handler
type UserMetadataResourceHandler struct {
}

//UserMetadataAPIHandler based on BasicAPIHandler
type UserMetadataAPIHandler struct {
	BasicAPIHandler
	Graph *graph.Graph
}

//New creates a new user metadata resource
func (m *UserMetadataResourceHandler) New() types.Resource {
	id, _ := uuid.NewV4()

	return &types.UserMetadata{
		UUID: id.String(),
	}
}

//Name return "usermetadata"
func (m *UserMetadataResourceHandler) Name() string {
	return "usermetadata"
}

//Create tests that whether the resource is duplicate or unique
func (m *UserMetadataAPIHandler) Create(r types.Resource) error {
	umd := r.(*types.UserMetadata)
	resources := m.BasicAPIHandler.Index()
	for _, resource := range resources {
		u := resource.(*types.UserMetadata)
		if u.GremlinQuery == umd.GremlinQuery && umd.Key == u.Key && umd.Value == u.Value {
			return fmt.Errorf("Duplicate user metadata, uuid=%s", u.UUID)
		} else if u.GremlinQuery == umd.GremlinQuery && umd.Key == u.Key && umd.Value != u.Value {
			u.Value = umd.Value
			return m.BasicAPIHandler.Create(u)
		}
	}

	return m.BasicAPIHandler.Create(r)
}

//RegisterUserMetadataAPI registers a new user metadata api handler
func RegisterUserMetadataAPI(apiServer *Server, g *graph.Graph) (*UserMetadataAPIHandler, error) {
	userMetadataAPIHandler := &UserMetadataAPIHandler{
		BasicAPIHandler: BasicAPIHandler{
			ResourceHandler: &UserMetadataResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
		Graph: g,
	}
	if err := apiServer.RegisterAPIHandler(userMetadataAPIHandler); err != nil {
		return nil, err
	}
	return userMetadataAPIHandler, nil
}
