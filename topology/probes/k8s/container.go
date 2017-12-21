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
	"sync"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
)

type containerCache struct {
	sync.RWMutex
	defaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*kubeCache
	graph            *graph.Graph
	hostIndexer      *graph.MetadataIndexer
	containerIndexer *graph.MetadataIndexer
}

// commonly accessed docker specific fields
const (
	DockerNameField         = "Docker.ContainerName"
	DockerPodNamespaceField = "Docker.Labels.io.kubernetes.pod.namespace"
	DockerPodNameField      = "Docker.Labels.io.kubernetes.pod.name"
)

func addNotNullFilter(m graph.Metadata, field string) {
	m[field] = filters.NewNotFilter(filters.NewNullFilter(field))
}

func newContainerIndexer(g *graph.Graph, manager string) *graph.MetadataIndexer {
	m := graph.Metadata{"Type": "container"}
	if manager != "" {
		m["Manager"] = manager
	}
	addNotNullFilter(m, DockerPodNamespaceField)
	addNotNullFilter(m, DockerPodNameField)

	return graph.NewMetadataIndexer(g, m, DockerPodNamespaceField, DockerPodNameField)
}

func (c *containerCache) getMetadata(pod *v1.Pod, container *v1.Container) graph.Metadata {
	m := graph.Metadata{
		"Type":    "container",
		"Name":    container.Name,
		"Manager": "k8s",
	}

	common.SetField(m, DockerNameField, container.Name)
	common.SetField(m, DockerPodNamespaceField, pod.GetNamespace())
	common.SetField(m, DockerPodNameField, pod.GetName())
	return m
}

func makeContainerUID(podUID, containerName string) string {
	return podUID + "-" + containerName
}

func (c *containerCache) OnAdd(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		c.Lock()
		defer c.Unlock()

		c.graph.Lock()
		defer c.graph.Unlock()

		for _, container := range pod.Spec.Containers {
			uid := makeContainerUID(string(pod.GetUID()), container.Name)
			hostNodes := c.hostIndexer.Get(pod.Spec.NodeName)
			if len(hostNodes) == 0 {
				continue
			}

			logging.GetLogger().Debugf("Creating node of k8s container %s", uid)
			containerNode := c.graph.NewNode(graph.Identifier(uid), c.getMetadata(pod, &container))
			topology.AddOwnershipLink(c.graph, hostNodes[0], containerNode, nil)
		}
	}
}

func (c *containerCache) OnUpdate(oldObj, newObj interface{}) {
}

func (c *containerCache) OnDelete(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		c.graph.Lock()
		defer c.graph.Unlock()

		containerNodes := c.containerIndexer.Get(pod.Namespace, pod.Name)
		for _, containerNode := range containerNodes {
			c.graph.DelNode(containerNode)
		}
	}
}

func (c *containerCache) Start() {
	c.containerIndexer.AddEventListener(c)
	c.hostIndexer.AddEventListener(c)
	c.kubeCache.Start()
}

func (c *containerCache) Stop() {
	c.containerIndexer.RemoveEventListener(c)
	c.hostIndexer.RemoveEventListener(c)
	c.kubeCache.Stop()
}

func newContainerCache(client *kubeClient, g *graph.Graph) *containerCache {
	c := &containerCache{
		graph:            g,
		hostIndexer:      newHostIndexer(g, "k8s"),
		containerIndexer: newContainerIndexer(g, "k8s"),
	}
	c.kubeCache = client.getCacheFor(client.Core().RESTClient(), &v1.Pod{}, "pods", c)
	return c
}
