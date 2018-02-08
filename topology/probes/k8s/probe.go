/*
 * Copyright 2017 IBM Corp.
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

package k8s

import (
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

// Probe for tracking k8s events
type Probe struct {
	bundle *probe.ProbeBundle
}

func makeProbeBundle(g *graph.Graph) *probe.ProbeBundle {
	configProbes := config.GetStringSlice("k8s.probes")
	logging.GetLogger().Infof("K8s probes: %v", configProbes)
	probes := make(map[string]probe.Probe)
	for name, i := range configProbes {
		switch i {
		case "pod":
			probes[i] = newPodProbe(g)
		case "networkpolicy":
			probes[i] = newNetworkPolicyProbe(g)
		case "container":
			probes[i] = newContainerProbe(g)
		case "node":
			probes[i] = newNodeProbe(g)
		case "namespace":
			probes[i] = newNamespaceProbe(g)
		default:
			logging.GetLogger().Errorf("skipping unsupported K8s probe %v", name)
		}
	}
	return probe.NewProbeBundle(probes)
}

// Start k8s probe
func (p *Probe) Start() {
	p.bundle.Start()
}

// Stop k8s probe
func (p *Probe) Stop() {
	p.bundle.Stop()
}

// NewProbe create the Probe for tracking k8s events
func NewProbe(g *graph.Graph) (*Probe, error) {
	err := initClientset()
	if err != nil {
		return nil, err
	}

	return &Probe{
		bundle: makeProbeBundle(g),
	}, nil
}
