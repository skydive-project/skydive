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

	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	nodeNameField = detailsField + ".Spec.NodeName"
)

type nodeHandler struct {
	DefaultResourceHandler
}

func (h *nodeHandler) IsTopLevel() bool {
	return true
}

func (h *nodeHandler) Dump(obj interface{}) string {
	node := obj.(*v1.Node)
	return fmt.Sprintf("node{Name: %s}", node.Name)
}

func (h *nodeHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	node := obj.(*v1.Node)

	m := NewMetadata(Manager, "node", node, node.Name)
	m.SetFieldAndNormalize("Labels", node.Labels)
	m.SetField("Cluster", node.ClusterName)
	for _, a := range node.Status.Addresses {
		if a.Type == "Hostname" || a.Type == "InternalIP" || a.Type == "ExternalIP" {
			m.SetField(string(a.Type), a.Address)
		}
	}
	m.SetField("Arch", node.Status.NodeInfo.Architecture)
	m.SetField("Kernel", node.Status.NodeInfo.KernelVersion)
	m.SetField("OS", node.Status.NodeInfo.OperatingSystem)

	return graph.Identifier(node.GetUID()), m
}

func newNodeProbe(clientset *kubernetes.Clientset, g *graph.Graph) Subprobe {
	return NewResourceCache(clientset.Core().RESTClient(), &v1.Node{}, "nodes", g, &nodeHandler{})
}

func newHostNodeLinker(g *graph.Graph, subprobes map[string]Subprobe) probe.Probe {
	nodeProbe := subprobes["node"]
	if nodeProbe == nil {
		return nil
	}

	hostIndexer := graph.NewMetadataIndexer(g, g, graph.Metadata{"Type": "host"}, "Hostname")
	hostIndexer.Start()

	nodeIndexer := graph.NewMetadataIndexer(g, nodeProbe, graph.Metadata{"Type": "node"}, "Hostname")
	nodeIndexer.Start()

	return graph.NewMetadataIndexerLinker(g, hostIndexer, nodeIndexer, newEdgeMetadata())
}

func newNodePodLinker(g *graph.Graph, subprobes map[string]Subprobe) probe.Probe {
	return newResourceLinker(g, subprobes, "node", []string{"Hostname"}, "pod", []string{nodeNameField}, newEdgeMetadata())
}
