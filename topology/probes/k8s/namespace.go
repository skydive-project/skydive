/*
 * Copyright (C) 2018 IBM, Inc.
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
	"github.com/skydive-project/skydive/topology"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// MetadataInnerNamespace contains the type specific fields
// easyjson:json
type MetadataInnerNamespace struct {
	MetadataInner
	Status string `skydive:"string"`
}

// GetField implements Getter interface
func (inner *MetadataInnerNamespace) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInnerNamespace) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInnerNamespace) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInnerNamespace) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerNamespaceDecoder implements a json message raw decoder
func MetadataInnerNamespaceDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInnerNamespace
	return GenericMetadataDecoder(&inner, raw)
}

type namespaceHandler struct {
}

func (h *namespaceHandler) Dump(obj interface{}) string {
	ns := obj.(*v1.Namespace)
	return fmt.Sprintf("namespace{Name: %s}", ns.Name)
}

func (h *namespaceHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	ns := obj.(*v1.Namespace)

	inner := new(MetadataInnerNamespace)
	inner.MetadataInner.Setup(&ns.ObjectMeta, ns)
	inner.Status = string(ns.Status.Phase)

	return graph.Identifier(ns.GetUID()), NewMetadata(Manager, "namespace", inner.Name, inner)
}

func newNamespaceProbe(client interface{}, g *graph.Graph) Subprobe {
	RegisterNodeDecoder(MetadataInnerNamespaceDecoder, "namespace")
	return NewResourceCache(client.(*kubernetes.Clientset).Core().RESTClient(), &v1.Namespace{}, "namespaces", g, &namespaceHandler{})
}

func newNamespaceLinker(g *graph.Graph, manager string, types ...string) probe.Probe {
	namespaceFilter := newTypesFilter(Manager, "namespace")
	namespaceIndexer := newObjectIndexerFromFilter(g, GetSubprobe(Manager, "namespace"), namespaceFilter, MetadataFields("Name")...)
	namespaceIndexer.Start()

	objectFilter := newTypesFilter(manager, types...)
	objectIndexer := newObjectIndexerFromFilter(g, g, objectFilter, MetadataFields("Namespace")...)
	objectIndexer.Start()

	ml := graph.NewMetadataIndexerLinker(g, namespaceIndexer, objectIndexer, topology.OwnershipMetadata())

	linker := &Linker{
		ResourceLinker: ml.ResourceLinker,
	}
	ml.AddEventListener(linker)

	return linker
}
