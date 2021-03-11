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
package ovnk8s

import (
	"github.com/skydive-project/skydive/probe"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// Probe describes an OVN-kubernetes probe
type Probe struct {
	graph.DefaultGraphListener
	graph  *graph.Graph
	bundle *probe.Bundle
}

func (p *Probe) Start() error {
	return p.bundle.Start()
}

func (p *Probe) Stop() {
	p.bundle.Stop()
}

// NewProbe creates a new graph OVN-kubernetes probe
func NewProbe(g *graph.Graph) (probe.Handler, error) {
	p := &Probe{
		graph:  g,
		bundle: &probe.Bundle{},
	}
	logging.GetLogger().Info("OVN-kubenetes probe created")
	return p, nil
}
