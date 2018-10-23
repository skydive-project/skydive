/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	dockerContainerNameField = "Docker.Labels.io.kubernetes.container.name"
	dockerPodNameField       = "Docker.Labels.io.kubernetes.pod.name"
	dockerPodNamespaceField  = "Docker.Labels.io.kubernetes.pod.namespace"
)

type containerProbe struct {
	*graph.EventHandler
	*KubeCache
	graph            *graph.Graph
	containerIndexer *graph.MetadataIndexer
}

func (c *containerProbe) newMetadata(pod *v1.Pod, container *v1.Container) graph.Metadata {
	m := NewMetadata(Manager, "container", container, container.Name, pod.Namespace)
	m.SetField("Pod", pod.Name)
	m.SetField("Image", container.Image)
	return m
}

func (c *containerProbe) dump(pod *v1.Pod, name string) string {
	return fmt.Sprintf("container{Namespace: %s, Pod: %s, Name: %s}", pod.GetNamespace(), pod.GetName(), name)
}

// OnAdd is called when a new Kubernetes resource has been created
func (c *containerProbe) OnAdd(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		c.graph.Lock()
		defer c.graph.Unlock()

		wasUpdated := make(map[string]bool)

		for _, container := range pod.Spec.Containers {
			uid := graph.GenID(string(pod.GetUID()), container.Name)
			m := c.newMetadata(pod, &container)
			if node := c.graph.GetNode(uid); node == nil {
				node = c.graph.NewNode(uid, m)
				c.NotifyEvent(graph.NodeAdded, node)
				logging.GetLogger().Debugf("Added %s", c.dump(pod, container.Name))
			} else {
				c.graph.SetMetadata(node, m)
				c.NotifyEvent(graph.NodeUpdated, node)
				logging.GetLogger().Debugf("Updated %s", c.dump(pod, container.Name))
			}
			wasUpdated[container.Name] = true
		}

		nodes, _ := c.containerIndexer.Get(pod.Namespace, pod.Name)
		for _, node := range nodes {
			name, _ := node.GetFieldString("Name")
			if !wasUpdated[name] {
				c.graph.DelNode(node)
				c.NotifyEvent(graph.NodeDeleted, node)
				logging.GetLogger().Debugf("Deleted %s", c.dump(pod, name))
			}
		}
	}
}

// OnUpdate is called when a Kubernetes resource has been updated
func (c *containerProbe) OnUpdate(oldObj, newObj interface{}) {
	c.OnAdd(newObj)
}

// OnDelete is called when a Kubernetes resource has been deleted
func (c *containerProbe) OnDelete(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		c.graph.Lock()
		defer c.graph.Unlock()

		containerNodes, _ := c.containerIndexer.Get(pod.Namespace, pod.Name)
		for _, containerNode := range containerNodes {
			name, _ := containerNode.GetFieldString("Name")
			c.graph.DelNode(containerNode)
			c.NotifyEvent(graph.NodeDeleted, containerNode)
			logging.GetLogger().Debugf("Deleted %s", c.dump(pod, name))
		}
	}
}

func newContainerProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	c := &containerProbe{
		EventHandler: graph.NewEventHandler(100),
		graph:        g,
	}
	c.containerIndexer = newObjectIndexer(g, c.EventHandler, "container", "Namespace", "Pod")
	c.KubeCache = NewKubeCache(clientset.CoreV1().RESTClient(), &v1.Pod{}, "pods", c)
	return c
}

func newPodContainerLinker(g *graph.Graph, subprobes map[string]Subprobe) probe.Probe {
	return newResourceLinker(g, subprobes, "pod", []string{"Namespace", "Name"}, "container", []string{"Namespace", "Pod"}, topology.OwnershipMetadata())
}

func newDockerIndexer(g *graph.Graph) *graph.MetadataIndexer {
	m := graph.NewElementFilter(filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", "docker"),
		filters.NewTermStringFilter("Type", "container"),
		filters.NewNotNullFilter(dockerPodNamespaceField),
		filters.NewNotNullFilter(dockerPodNameField),
	))

	return graph.NewMetadataIndexer(g, g, m, dockerPodNamespaceField, dockerPodNameField, dockerContainerNameField)
}

func newContainerDockerLinker(g *graph.Graph, subprobes map[string]Subprobe) probe.Probe {
	containerProbe := subprobes["container"]
	if containerProbe == nil {
		return nil
	}

	containerIndexer := newObjectIndexer(g, containerProbe, "container", "Namespace", "Pod", "Name")
	containerIndexer.Start()

	dockerIndexer := newDockerIndexer(g)
	dockerIndexer.Start()

	return graph.NewMetadataIndexerLinker(g, containerIndexer, dockerIndexer, newEdgeMetadata())
}
