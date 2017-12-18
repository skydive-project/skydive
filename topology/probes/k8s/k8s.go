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
	PodToContainerLink = "pod2container"
	PolicyToPodLink    = "policy2pod"
)

var (
	PodToContainerMetadata = graph.Metadata{"RelationType": PodToContainerLink}
	PolicyToPodMetadata    = graph.Metadata{"RelationType": PolicyToPodLink}
)

type probe struct {
	graph              *graph.Graph
	client             *kubeClient
	podCache           *podCache
	networkPolicyCache *networkPolicyCache
}

func (k8s *probe) Start() {
	k8s.networkPolicyCache.Start()
	k8s.podCache.Start()
}

func (k8s *probe) Stop() {
	k8s.networkPolicyCache.Stop()
	k8s.podCache.Stop()
}

func NewProbe(g *graph.Graph) (k8s *probe, err error) {
	client, err := newKubeClient()
	if err != nil {
		return nil, err
	}

	podCache := newPodCache(client, g)
	networkPolicyCache := newNetworkPolicyCache(client, g, podCache)

	return &probe{
		graph:              g,
		client:             client,
		podCache:           podCache,
		networkPolicyCache: networkPolicyCache,
	}, nil
}
