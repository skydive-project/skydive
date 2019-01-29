// +build !linux !opencontrail

/*
 * Copyright (C) 2018 Orange, Inc.
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

package opencontrail

import (
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// Probe describes a probe that reads OpenContrail database and updates the graph
type Probe struct {
}

// Start the probe
func (mapper *Probe) Start() {
}

// Stop the probe
func (mapper *Probe) Stop() {
}

// NewProbeFromConfig creates a new OpenContrail probe based on configuration
func NewProbeFromConfig(g *graph.Graph, r *graph.Node) (*Probe, error) {
	return nil, common.ErrNotImplemented
}
