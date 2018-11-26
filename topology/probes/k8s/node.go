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
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerNode contains the type specific fields
// easyjson:json
type MetadataInnerNode struct {
	MetadataInner
	Hostname   string `skydive:"string" json:"Hostname,omitempty"`
	InternalIP string `skydive:"string" json:"InternalIP,omitempty"`
	ExternalIP string `skydive:"string" json:"ExternalIP,omitempty"`
	Arch       string `skydive:"string"`
	Kernel     string `skydive:"string"`
	OS         string `skydive:"string"`
}

// GetField implements Getter interface
func (inner *MetadataInnerNode) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerNode) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerNode) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerNode) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerNodeDecoder implements a json message raw decoder
func MetadataInnerNodeDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerNode
	return GenericMetadataDecoder(&inner, raw)
}

type nodeHandler struct {
}

func (h *nodeHandler) Dump(obj interface{}) string {
	node := obj.(*v1.Node)
	return fmt.Sprintf("node{Name: %s}", node.Name)
}

func (h *nodeHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	node := obj.(*v1.Node)

	inner := new(MetadataInnerNode)
	inner.MetadataInner.Setup(&node.ObjectMeta, node)
	for _, a := range node.Status.Addresses {
		switch a.Type {
		case "Hostname":
			inner.Hostname = a.Address
		case "InternalIP":
			inner.InternalIP = a.Address
		case "ExternalIP":
			inner.ExternalIP = a.Address
		}
	}
	inner.Arch = node.Status.NodeInfo.Architecture
	inner.Kernel = node.Status.NodeInfo.KernelVersion
	inner.OS = node.Status.NodeInfo.OperatingSystem

	return graph.Identifier(node.GetUID()), NewMetadata(Manager, "node", inner.Name, inner)
}

func newNodeProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerNodeDecoder, "node")
	return NewResourceCache(client.(*kubernetes.Clientset).Core().RESTClient(), &v1.Node{}, "nodes", g, &nodeHandler{})
}

func newHostNodeLinker(g *graph.Graph) probe.Probe {
	nodeProbe := GetSubprobe(Manager, "node")
	if nodeProbe == nil {
		return nil
	}

	hostIndexer := graph.NewMetadataIndexer(g, g, graph.Metadata{"Type": "host"}, "Hostname")
	hostIndexer.Start()

	nodeIndexer := graph.NewMetadataIndexer(g, nodeProbe, graph.Metadata{"Type": "node"}, MetadataField("Name"))
	nodeIndexer.Start()

	ml := graph.NewMetadataIndexerLinker(g, hostIndexer, nodeIndexer, NewEdgeMetadata(Manager, "association"))

	linker := &Linker{
		ResourceLinker: ml.ResourceLinker,
	}
	ml.AddEventListener(linker)

	return linker
}

func newNodePodLinker(g *graph.Graph) probe.Probe {
	return newResourceLinker(g, GetSubprobesMap(Manager), "node", MetadataFields("Name"), "pod", MetadataFields("Node"), NewEdgeMetadata(Manager, "association"))
}
