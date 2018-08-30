/*
 * Copyright 2018 IBM Corp.
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

package istio

import (
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/k8s"
)

const (
	detailsField = "Istio"
	Manager      = "istio"
)

func newMetadata(ty, namespace, name string, details interface{}) graph.Metadata {
	return k8s.NewMetadata(Manager, ty, namespace, name, details)
}

func newEdgeMetadata() graph.Metadata {
	return k8s.NewEdgeMetadata(Manager)
}

func addOwnershipLink(g *graph.Graph, parent, child *graph.Node) *graph.Edge {
	return k8s.AddOwnershipLink(Manager, g, parent, child)
}

func newObjectIndexerByNamespace(g *graph.Graph, ty string) *graph.MetadataIndexer {
	return k8s.NewObjectIndexerByNamespace(Manager, g, ty)
}

func newObjectIndexerByNamespaceAndName(g *graph.Graph, ty string) *graph.MetadataIndexer {
	return k8s.NewObjectIndexerByNamespaceAndName(Manager, g, ty)
}

func newObjectIndexerByName(g *graph.Graph, ty string) *graph.MetadataIndexer {
	return k8s.NewObjectIndexerByName(Manager, g, ty)
}
