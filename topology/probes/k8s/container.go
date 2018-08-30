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

	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
)

type containerProbe struct {
	DefaultKubeCacheEventHandler
	graph.DefaultGraphListener
	*KubeCache
	graph            *graph.Graph
	podIndexer       *graph.MetadataIndexer
	containerIndexer *graph.MetadataIndexer
	dockerIndexer    *graph.MetadataIndexer
}

// commonly accessed docker specific fields
const (
	DockerNameField         = "Docker.ContainerName"
	DockerPodNameField      = "Docker.Labels.io.kubernetes.pod.name"
	DockerPodNamespaceField = "Docker.Labels.io.kubernetes.pod.namespace"
)

func newDockerIndexer(g *graph.Graph) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", "docker"),
		filters.NewTermStringFilter("Type", "container"),
		filters.NewNotNullFilter(DockerPodNamespaceField),
		filters.NewNotNullFilter(DockerPodNameField),
		filters.NewNotNullFilter(DockerNameField))
	m := graph.NewGraphElementFilter(filter)

	return graph.NewMetadataIndexer(g, m, DockerPodNamespaceField, DockerPodNameField, DockerNameField)
}

func newContainerIndexer(g *graph.Graph) *graph.MetadataIndexer {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", "k8s"),
		filters.NewTermStringFilter("Type", "container"),
		filters.NewNotNullFilter("Namespace"),
		filters.NewNotNullFilter("Pod"))
	m := graph.NewGraphElementFilter(filter)

	return graph.NewMetadataIndexer(g, m, "Namespace", "Pod")
}

func (c *containerProbe) newMetadata(pod *v1.Pod, container *v1.Container) graph.Metadata {
	m := NewMetadata(Manager, "container", pod.Namespace, container.Name, container)
	m.SetField("Pod", pod.Name)
	m.SetFieldAndNormalize("Labels", pod.Labels)
	m.SetField("Image", container.Image)
	return m
}

func containerUID(pod *v1.Pod, containerName string) graph.Identifier {
	return graph.GenIDNameBased(string(pod.GetUID()), containerName)
}

func dumpContainer2(pod *v1.Pod, containerName string) string {
	return fmt.Sprintf("container{podNamespace: %s, podName: %s, containerName: %s}", pod.GetNamespace(), pod.GetName(), containerName)
}

func dumpContainer(pod *v1.Pod, container *v1.Container) string {
	return dumpContainer2(pod, container.Name)
}

func (c *containerProbe) linkContainerToPod(pod *v1.Pod, container *v1.Container, containerNode *graph.Node) {
	podNodes, _ := c.podIndexer.Get(pod.GetNamespace(), pod.GetName())

	if len(podNodes) == 0 {
		logging.GetLogger().Debugf("Can't find %s", dumpPod(pod))
		return
	}

	logging.GetLogger().Debugf("Linking %s to %s", dumpContainer(pod, container), dumpPod(pod))
	AddOwnershipLink(Manager, c.graph, podNodes[0], containerNode)
}

func (c *containerProbe) linkContainerToDocker(pod *v1.Pod, container *v1.Container, containerNode *graph.Node) {
	dockerNodes, _ := c.dockerIndexer.Get(pod.GetNamespace(), pod.GetName(), container.Name)

	if len(dockerNodes) == 0 {
		logging.GetLogger().Debugf("Can't find docker associated with %s", dumpContainer(pod, container))
		return
	}

	AddLinkTry(c.graph, containerNode, dockerNodes[0], NewEdgeMetadata(Manager))
}

func (c *containerProbe) onContainerAdd(pod *v1.Pod, container *v1.Container) {
	c.graph.Lock()
	defer c.graph.Unlock()

	uid := containerUID(pod, container.Name)
	containerNode := c.graph.GetNode(uid)
	if containerNode == nil {
		logging.GetLogger().Debugf("Adding %s", dumpContainer(pod, container))
		containerNode = NewNode(c.graph, uid, c.newMetadata(pod, container))
	} else {
		logging.GetLogger().Debugf("Updating %s (as it already exists)", dumpContainer(pod, container))
		AddMetadata(c.graph, containerNode, container)
	}

	c.linkContainerToPod(pod, container, containerNode)
	c.linkContainerToDocker(pod, container, containerNode)
}

func (c *containerProbe) onPodAdd(pod *v1.Pod) {
	for _, container := range pod.Spec.Containers {
		c.onContainerAdd(pod, &container)
	}
}

func (c *containerProbe) OnAdd(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}

	logging.GetLogger().Debugf("Creating container nodes for %s", dumpPod(pod))
	c.onPodAdd(pod)
}

func (c *containerProbe) OnUpdate(oldObj, newObj interface{}) {
	pod, ok := newObj.(*v1.Pod)
	if !ok {
		return
	}
	logging.GetLogger().Debugf("Updating container nodes of %s", dumpPod(pod))
	c.onPodAdd(pod)
}

func (c *containerProbe) OnDelete(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		c.graph.Lock()
		defer c.graph.Unlock()

		containerNodes, _ := c.containerIndexer.Get(pod.Namespace, pod.Name)
		logging.GetLogger().Debugf("Deleting container nodes of %s", dumpPod(pod))
		for _, containerNode := range containerNodes {
			containerName, _ := containerNode.GetFieldString(DockerNameField)
			logging.GetLogger().Debugf("Deleting %s", dumpContainer2(pod, containerName))
			c.graph.DelNode(containerNode)
		}
	}
}

func (c *containerProbe) Start() {
	c.containerIndexer.AddEventListener(c)
	c.containerIndexer.Start()
	c.dockerIndexer.AddEventListener(c)
	c.dockerIndexer.Start()
	c.podIndexer.AddEventListener(c)
	c.podIndexer.Start()
	c.KubeCache.Start()
}

func (c *containerProbe) Stop() {
	c.containerIndexer.RemoveEventListener(c)
	c.containerIndexer.Stop()
	c.dockerIndexer.RemoveEventListener(c)
	c.dockerIndexer.Stop()
	c.podIndexer.RemoveEventListener(c)
	c.podIndexer.Stop()
	c.KubeCache.Stop()
}

func newContainerProbe(g *graph.Graph) probe.Probe {
	c := &containerProbe{
		graph:            g,
		podIndexer:       newPodIndexerByName(g),
		containerIndexer: newContainerIndexer(g),
		dockerIndexer:    newDockerIndexer(g),
	}
	c.KubeCache = newPodKubeCache(c)
	return c
}
