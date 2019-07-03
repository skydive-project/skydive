/*
 * Copyright (C) 2017 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package k8s

import (
	"fmt"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type nodeHandler struct {
}

func (h *nodeHandler) Dump(obj interface{}) string {
	node := obj.(*v1.Node)
	return fmt.Sprintf("node{Name: %s}", node.Name)
}

func (h *nodeHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	node := obj.(*v1.Node)

	m := NewMetadataFields(&node.ObjectMeta)
	for _, a := range node.Status.Addresses {
		if a.Type == "Hostname" || a.Type == "InternalIP" || a.Type == "ExternalIP" {
			m.SetField(string(a.Type), a.Address)
		}
	}
	m.SetField("Arch", node.Status.NodeInfo.Architecture)
	m.SetField("Kernel", node.Status.NodeInfo.KernelVersion)
	m.SetField("OS", node.Status.NodeInfo.OperatingSystem)

	return graph.Identifier(node.GetUID()), NewMetadata(Manager, "node", m, node, node.Name)
}

func newNodeProbe(client interface{}, g *graph.Graph) Subprobe {
	return NewResourceCache(client.(*kubernetes.Clientset).CoreV1().RESTClient(), &v1.Node{}, "nodes", g, &nodeHandler{})
}

func newHostNodeLinker(g *graph.Graph) probe.Handler {
	nodeProbe := GetSubprobe(Manager, "node")
	if nodeProbe == nil {
		return nil
	}

	hostIndexer := graph.NewMetadataIndexer(g, g, graph.Metadata{"Type": "host"}, "Hostname")
	hostIndexer.Start()

	nodeIndexer := graph.NewMetadataIndexer(g, nodeProbe, graph.Metadata{"Type": "node"}, MetadataField("Name"))
	nodeIndexer.Start()

	ml := graph.NewMetadataIndexerLinker(g, hostIndexer, nodeIndexer, NewEdgeMetadata(Manager, "node"))

	linker := &Linker{
		ResourceLinker: ml.ResourceLinker,
	}
	ml.AddEventListener(linker)

	return linker
}

func newNodePodLinker(g *graph.Graph) probe.Handler {
	nodeIndexer := newResourceIndexer(g, Manager, "node", MetadataFields("Name"))
	podIndexer := newResourceIndexer(g, Manager, "pod", MetadataFields("Node"))
	return newResourceLinker(g, nodeIndexer, podIndexer, NewEdgeMetadata(Manager, "node"))
}
