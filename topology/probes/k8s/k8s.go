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

const (
	podToContainerLink = "pod2container"
	policyToPodLink    = "policy2pod"
)

var (
	podToContainerMetadata = graph.Metadata{"RelationType": podToContainerLink}
	policyToPodMetadata    = graph.Metadata{"RelationType": policyToPodLink}
)

// Probe for tracking k8s events
type Probe struct {
	graph              *graph.Graph
	client             *kubeClient
	podCache           *podCache
	networkPolicyCache *networkPolicyCache
	nodeCache          *nodeCache
	containerCache     *containerCache
	bundle             *probe.ProbeBundle
}

func (p *Probe) makeProbeBundle() *probe.ProbeBundle {
	subprobes := config.GetConfig().GetStringSlice("k8s.subprobes")
	logging.GetLogger().Infof("K8s subprobes: %v", subprobes)
	probes := make(map[string]probe.Probe)
	for _, i := range subprobes {
		switch i {
		case "pod":
			probes[i] = p.podCache
		case "networkpolicy":
			probes[i] = p.networkPolicyCache
		case "container":
			probes[i] = p.containerCache
		case "node":
			probes[i] = p.nodeCache
		default:
			logging.GetLogger().Errorf("skipping unsupported K8s subprobe %v", i)
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
	client, err := newKubeClient()
	if err != nil {
		return nil, err
	}

	p := &Probe{
		graph:  g,
		client: client,
	}

	p.podCache = newPodCache(client, g)
	p.networkPolicyCache = newNetworkPolicyCache(client, g, p.podCache)
	p.containerCache = newContainerCache(client, g)
	p.nodeCache = newNodeCache(client, g)
	p.bundle = p.makeProbeBundle()

	return p, nil
}
