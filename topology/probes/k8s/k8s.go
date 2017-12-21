/*
 * Copyright 2016 IBM Corp.
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
}

type starter interface {
	Start()
	Stop()
}

func (probe *Probe) getSubProbes() []starter {
	return []starter{
		probe.podCache,
		probe.networkPolicyCache,
		probe.nodeCache,
		probe.containerCache,
	}
}

// Start k8s probe
func (probe *Probe) Start() {
	for _, sub := range probe.getSubProbes() {
		sub.Start()
	}
}

// Stop k8s probe
func (probe *Probe) Stop() {
	for _, sub := range probe.getSubProbes() {
		sub.Stop()
	}
}

// NewProbe create the Probe for tracking k8s events
func NewProbe(g *graph.Graph) (probe *Probe, err error) {
	client, err := newKubeClient()
	if err != nil {
		return nil, err
	}

	podCache := newPodCache(client, g)
	networkPolicyCache := newNetworkPolicyCache(client, g, podCache)
	nodeCache := newNodeCache(client, g)
	containerCache := newContainerCache(client, g)

	return &Probe{
		graph:              g,
		client:             client,
		podCache:           podCache,
		networkPolicyCache: networkPolicyCache,
		nodeCache:          nodeCache,
		containerCache:     containerCache,
	}, nil
}
