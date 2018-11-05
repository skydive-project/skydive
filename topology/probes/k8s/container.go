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
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/client-go/kubernetes"
)

const (
	dockerContainerNameField = "Docker.Labels.io.kubernetes.container.name"
	dockerPodNameField       = "Docker.Labels.io.kubernetes.pod.name"
	dockerPodNamespaceField  = "Docker.Labels.io.kubernetes.pod.namespace"
)

type containerProbe struct {
	graph.EventHandler
	graph.DefaultGraphListener
	graph *graph.Graph
}

func (p *containerProbe) getContainerMetadata(podNode *graph.Node, containerName string, container map[string]interface{}) graph.Metadata {
	podName, _ := podNode.GetFieldString("Name")
	podNamespace, _ := podNode.GetFieldString("Namespace")
	m := NewMetadata(Manager, "container", container, containerName, podNamespace)
	m.SetField("Pod", podName)
	m.SetField("Image", container["Image"])
	return m
}

func (p *containerProbe) handleContainers(podNode *graph.Node) map[graph.Identifier]*graph.Node {
	containers := make(map[graph.Identifier]*graph.Node)
	specContainers, _ := podNode.GetField(detailsField + ".Spec.Containers")
	if specContainers, ok := specContainers.([]interface{}); ok {
		for _, container := range specContainers {
			if container, ok := container.(map[string]interface{}); ok {
				containerName := container["Name"].(string)
				uid := graph.GenID(string(podNode.ID), containerName)
				m := p.getContainerMetadata(podNode, containerName, container)
				node := p.graph.GetNode(uid)
				if node == nil {
					node = p.graph.NewNode(uid, m)
					p.graph.NewEdge(graph.GenID(), podNode, node, topology.OwnershipMetadata(), "")
				} else {
					p.graph.SetMetadata(node, m)
				}
				containers[uid] = node
			}
		}
	}
	return containers
}

func (p *containerProbe) OnNodeAdded(node *graph.Node) {
	if nodeType, _ := node.GetFieldString("Type"); nodeType == "pod" {
		p.handleContainers(node)
	}
}

func (p *containerProbe) OnNodeUpdated(node *graph.Node) {
	if nodeType, _ := node.GetFieldString("Type"); nodeType == "pod" {
		previousContainers := p.graph.LookupChildren(node, graph.Metadata{"Type": "container"}, topology.OwnershipMetadata())
		containers := p.handleContainers(node)

		for _, container := range previousContainers {
			if _, found := containers[container.ID]; !found {
				p.graph.DelNode(container)
			}
		}
	}
}

func (p *containerProbe) OnNodeDeleted(node *graph.Node) {
	if nodeType, _ := node.GetFieldString("Type"); nodeType == "pod" {
		containers := p.graph.LookupChildren(node, graph.Metadata{"Type": "container"}, topology.OwnershipMetadata())
		for _, container := range containers {
			p.graph.DelNode(container)
		}
	}
}

func (p *containerProbe) Start() {
	p.graph.AddEventListener(p)
}

func (p *containerProbe) Stop() {
	p.graph.RemoveEventListener(p)
}

func newContainerProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return &containerProbe{graph: g}
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

func newContainerLinker(g *graph.Graph, subprobes map[string]Subprobe) probe.Probe {
	podProbe := subprobes["pod"]
	if podProbe == nil {
		return nil
	}

	k8sIndexer := newObjectIndexer(g, g, "container", "Namespace", "Pod", "Name")
	k8sIndexer.Start()

	dockerIndexer := newDockerIndexer(g)
	dockerIndexer.Start()

	return graph.NewMetadataIndexerLinker(g, k8sIndexer, dockerIndexer, newEdgeMetadata())
}
