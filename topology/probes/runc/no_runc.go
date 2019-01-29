// +build !linux

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

package runc

import (
	"github.com/skydive-project/skydive/common"
	ns "github.com/skydive-project/skydive/topology/probes/netns"
)

// Probe describes a list NetLink NameSpace probe to enhance the graph
type Probe struct {
}

// Start the probe
func (probe *Probe) Start() {
}

// Stop the probe
func (probe *Probe) Stop() {
}

// NewProbe creates a new runc probe
func NewProbe(nsProbe *ns.Probe) (*Probe, error) {
	return nil, common.ErrNotImplemented
}
