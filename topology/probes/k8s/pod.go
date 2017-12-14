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

	"k8s.io/api/core/v1"
)

// PodCache for keeping state of pod objects
type PodCache struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func (c *PodCache) getMetadata(pod *v1.Pod) graph.Metadata {
	return graph.Metadata{
		"Type":       "k8s::pod",
		"Name":       pod.GetName(),
		"UID":        pod.GetUID(),
		"ObjectMeta": pod.ObjectMeta,
		"Spec":       pod.Spec,
	}
}

func (c *PodCache) getLabel(pod *v1.Pod) string {
	return fmt.Sprintf("k8s::pod::%s::%s", pod.GetUID(), pod.GetName())
}

func (c *PodCache) doUpdate(pod *v1.Pod) {
	c.graph.NewNode(graph.Identifier(pod.GetUID()), c.getMetadata(pod))
	// TODO: create links between pod and pods
}

// OnAdd of a pod object
func (c *PodCache) OnAdd(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		logging.GetLogger().Debugf("Adding %s", c.getLabel(pod))
		c.doUpdate(pod)
	}
}

// OnUpdate of a pod object
func (c *PodCache) OnUpdate(oldObj, newObj interface{}) {
	if pod, ok := newObj.(*v1.Pod); ok {
		logging.GetLogger().Debugf("Updating %s", c.getLabel(pod))
		c.doUpdate(pod)
	}
}

// OnDelete of a pod object
func (c *PodCache) OnDelete(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		logging.GetLogger().Debugf("Deleting %s", c.getLabel(pod))
		if podNode := c.graph.GetNode(graph.Identifier(pod.GetUID())); podNode != nil {
			c.graph.DelNode(podNode)
		}
	}
}

func newPodCache(client *kubeClient, g *graph.Graph) *PodCache {
	c := &PodCache{graph: g}
	c.kubeCache = client.getCacheFor(
		client.CoreV1().RESTClient(),
		&v1.Pod{},
		"pods",
		c)
	return c
}
