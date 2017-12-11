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

package k8s

import (
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"

	networking_v1 "k8s.io/api/extensions/v1beta1"
)

type networkPolicyCache struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func (n *networkPolicyCache) networkPolicyMetadata(policy *networking_v1.NetworkPolicy) graph.Metadata {
	return graph.Metadata{
		"Type": "networkpolicy",
		"Name": policy.GetName(),
	}
}

func (n *networkPolicyCache) OnAdd(obj interface{}) {
	if networkPolicy, ok := obj.(*networking_v1.NetworkPolicy); ok {
		n.handleNetworkPolicy(networkPolicy)
	}
}

func (n *networkPolicyCache) OnUpdate(oldObj, newObj interface{}) {
	if networkPolicy, ok := newObj.(*networking_v1.NetworkPolicy); ok {
		n.handleNetworkPolicy(networkPolicy)
	}
}

func (n *networkPolicyCache) OnDelete(obj interface{}) {
	if policy, ok := obj.(*networking_v1.NetworkPolicy); ok {
		if policyNode := n.graph.GetNode(graph.Identifier(policy.GetUID())); policyNode != nil {
			n.graph.DelNode(policyNode)
		}
	}
}

func (n *networkPolicyCache) handleNetworkPolicy(policy *networking_v1.NetworkPolicy) {
	logging.GetLogger().Debugf("Handling network policy %s:%s", policy.GetUID(), policy.ObjectMeta.Name)
	n.graph.NewNode(graph.Identifier(policy.GetUID()), n.networkPolicyMetadata(policy))
	// TODO: create links between network policy and pods
}

func newNetworkPolicyCache(client *kubeClient, g *graph.Graph) *networkPolicyCache {
	n := &networkPolicyCache{graph: g}
	n.kubeCache = client.getCacheFor(
		client.ExtensionsV1beta1().RESTClient(),
		&networking_v1.NetworkPolicy{},
		"networkpolicies",
		n)
	return n
}
