// +build !linux !opencontrail

/*
 * Copyright (C) 2018 Orange, Inc.
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

package opencontrail

import (
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/topology/graph"
)

// OpenContrailProbe describes a probe that reads OpenContrail database and updates the graph
type OpenContrailProbe struct {
}

// Start the probe
func (mapper *OpenContrailProbe) Start() {
}

// Stop the probe
func (mapper *OpenContrailProbe) Stop() {
}

// NewOpenContrailProbeFromConfig creates a new OpenContrail probe based on configuration
func NewOpenContrailProbeFromConfig(g *graph.Graph, r *graph.Node) (*OpenContrailProbe, error) {
	return nil, common.ErrNotImplemented
}
