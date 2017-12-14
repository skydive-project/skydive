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
	"fmt"

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/extensions/v1beta1"
)

// NetworkPolicyCache for maintaining state of network policy objects
type NetworkPolicyCache struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func (c *NetworkPolicyCache) getMetadata(np *v1beta1.NetworkPolicy) graph.Metadata {
	return graph.Metadata{
		"Type":       "k8s::networkpolicy",
		"Name":       np.GetName(),
		"UID":        np.GetUID(),
		"ObjectMeta": np.ObjectMeta,
		"Spec":       np.Spec,
	}
}

func (c *NetworkPolicyCache) getLabel(np *v1beta1.NetworkPolicy) string {
	return fmt.Sprintf("k8s::networkpolicy::%s::%s", np.GetUID(), np.GetName())
}

func (c *NetworkPolicyCache) doUpdate(np *v1beta1.NetworkPolicy) {
	c.graph.NewNode(graph.Identifier(np.GetUID()), c.getMetadata(np))
	// TODO: create links between pod and pods
}

// OnAdd a network policy
func (c *NetworkPolicyCache) OnAdd(obj interface{}) {
	if np, ok := obj.(*v1beta1.NetworkPolicy); ok {
		logging.GetLogger().Debugf("Adding %s", c.getLabel(np))
		c.doUpdate(np)
	}
}

// OnUpdate a network policy
func (c *NetworkPolicyCache) OnUpdate(oldObj, newObj interface{}) {
	if np, ok := newObj.(*v1beta1.NetworkPolicy); ok {
		logging.GetLogger().Debugf("Updating %s", c.getLabel(np))
		c.doUpdate(np)
	}
}

// OnDelete a network policy
func (c *NetworkPolicyCache) OnDelete(obj interface{}) {
	if np, ok := obj.(*v1beta1.NetworkPolicy); ok {
		if node := c.graph.GetNode(graph.Identifier(np.GetUID())); node != nil {
			logging.GetLogger().Debugf("Deleting %s", c.getLabel(np))
			c.graph.DelNode(node)
		}
	}
}

func newNetworkPolicyCache(client *kubeClient, g *graph.Graph) *NetworkPolicyCache {
	c := &NetworkPolicyCache{graph: g}
	c.kubeCache = client.getCacheFor(
		client.ExtensionsV1beta1().RESTClient(),
		&v1beta1.NetworkPolicy{},
		"networkpolicies",
		c)
	return c
}
