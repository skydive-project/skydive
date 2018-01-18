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
	podIndexer       *graph.MetadataIndexer
	containerIndexer *graph.MetadataIndexer
}

// commonly accessed docker specific fields
const (
	DockerNameField         = "Docker.ContainerName"
	DockerPodNamespaceField = "Docker.Labels.io.kubernetes.pod.namespace"
	DockerPodNameField      = "Docker.Labels.io.kubernetes.pod.name"
)

func newContainerIndexer(g *graph.Graph) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Type", "container"),
		filters.NewNotFilter(filters.NewNullFilter(DockerPodNamespaceField)),
		filters.NewNotFilter(filters.NewNullFilter(DockerPodNameField)))
	m := graph.NewGraphElementFilter(filter)

	return graph.NewMetadataIndexer(g, m, DockerPodNamespaceField, DockerPodNameField)
}

func (c *containerCache) newMetadata(pod *v1.Pod, container *v1.Container) graph.Metadata {
	m := newMetadata("container", container.Name, container)
	m.SetField(DockerNameField, container.Name)
	m.SetField(DockerPodNamespaceField, pod.GetNamespace())
	m.SetField(DockerPodNameField, pod.GetName())
	return m
}

func containerUID(pod *v1.Pod, containerName string) graph.Identifier {
	return graph.GenIDNameBased(string(pod.GetUID()), containerName)
}

func (c *containerCache) linkContainerToPod(pod *v1.Pod, container *v1.Container, containerNode *graph.Node) {
	podNodes := c.podIndexer.Get(pod.GetName())

	if len(podNodes) == 0 {
		logging.GetLogger().Warningf("Can't find pod{%s}", container.Name, pod.GetName())
		return
	}

	logging.GetLogger().Infof("Linking container{%s} to pod{%s}", container.Name, pod.GetName())
	topology.AddOwnershipLink(c.graph, podNodes[0], containerNode, nil)
}

func (c *containerCache) onContainerAdd(pod *v1.Pod, container *v1.Container) {
	c.Lock()
	defer c.Unlock()

	c.graph.Lock()
	defer c.graph.Unlock()

	uid := containerUID(pod, container.Name)
	containerNode := c.graph.GetNode(uid)
	if containerNode == nil {
		logging.GetLogger().Infof("Creating container{%s}", container.Name)
		containerNode = c.graph.NewNode(uid, c.newMetadata(pod, container))
	} else {
		logging.GetLogger().Infof("container{%s} already exists", container.Name)
		addMetadata(c.graph, containerNode, container)
	}

	c.linkContainerToPod(pod, container, containerNode)
}

func (c *containerCache) onPodAdd(pod *v1.Pod) {
	for _, container := range pod.Spec.Containers {
		c.onContainerAdd(pod, &container)
	}
}

func (c *containerCache) OnAdd(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	logging.GetLogger().Infof("Creating containers for pod{%s}", pod.GetName())
	c.onPodAdd(pod)
}

func (c *containerCache) OnUpdate(oldObj, newObj interface{}) {
	pod, ok := newObj.(*v1.Pod)
	if !ok {
		return
	}
	logging.GetLogger().Infof("Updating containers for pod{%s}", pod.GetName())
	c.onPodAdd(pod)
}

func (c *containerCache) OnDelete(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		c.graph.Lock()
		defer c.graph.Unlock()

		containerNodes := c.containerIndexer.Get(pod.Namespace, pod.Name)
		logging.GetLogger().Infof("Deleting containers for pod{%s}", pod.GetName())
		for _, containerNode := range containerNodes {
			containerName, _ := containerNode.GetFieldString(DockerNameField)
			logging.GetLogger().Infof("Deleting container{%s}", containerName)
			c.graph.DelNode(containerNode)
		}
	}
}

func (c *containerCache) Start() {
	c.containerIndexer.AddEventListener(c)
	c.podIndexer.AddEventListener(c)
	c.kubeCache.Start()
}

func (c *containerCache) Stop() {
	c.containerIndexer.RemoveEventListener(c)
	c.podIndexer.RemoveEventListener(c)
	c.kubeCache.Stop()
}

func newContainerCache(client *kubeClient, g *graph.Graph) *containerCache {
	c := &containerCache{
		graph:            g,
		podIndexer:       newPodIndexerByName(g),
		containerIndexer: newContainerIndexer(g),
	}
	c.kubeCache = client.getCacheFor(client.Core().RESTClient(), &v1.Pod{}, "pods", c)
	return c
}
