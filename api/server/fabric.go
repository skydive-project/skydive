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
	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/api/types"
)

//FabricProbeResourceHandler describes a packet injector resource handler
type FabricProbeResourceHandler struct {
	ResourceHandler
}

//FabricProbeAPI exposes the fabric probe API
type FabricProbeAPI struct {
	BasicAPIHandler
}

func (fprh *FabricProbeResourceHandler) Name() string {
	return "fabricprobe"
}

func (fprh *FabricProbeResourceHandler) New() types.Resource {
	id, _ := uuid.NewV4()

	return &types.FabricProbe{
		UUID: id.String(),
	}
}

//RegisterFabricAPI registers a new fabric probe resource in the API
func RegisterFabricAPI(apiServer *Server) (*FabricProbeAPI, error) {
	fba := &FabricProbeAPI{
		BasicAPIHandler: BasicAPIHandler{
			ResourceHandler: &FabricProbeResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
	}

	if err := apiServer.RegisterAPIHandler(fba); err != nil {
		return nil, err
	}

	return fba, nil

}
