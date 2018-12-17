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

package k8s

import (
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AreLinkedCallback return true if (a, b) should be linked
type AreLinked func(a, b interface{}) bool

// ABLinker basis for a simple A to B linker
type ABLinker struct {
	Manager   string
	Type      string
	Graph     *graph.Graph
	ACache    *ResourceCache
	BCache    *ResourceCache
	AreLinked AreLinked
}

type ABLinkerInterface interface {
	GetABLinker() *ABLinker
}

// GetABLinker returns inner object
func (l *ABLinker) GetABLinker() *ABLinker {
	return l
}

// NewEdgeMetadata create a new edge metadata
func (l *ABLinker) NewEdgeMetadata() graph.Metadata {
	return NewEdgeMetadata(l.Manager, l.Type)
}

// GenEdgeID generate a new ID for edge
func (l *ABLinker) GenEdgeID(ANode, BNode *graph.Node) graph.Identifier {
	return graph.GenID(string(ANode.ID), string(BNode.ID), "RelationType", l.Type)
}

// AppendNewEdge create and append a new edge to list of edges
func (l *ABLinker) AppendNewEdge(inputEdges []*graph.Edge, ANode, BNode *graph.Node) []*graph.Edge {
	edge, err := l.Graph.NewEdge(l.GenEdgeID(ANode, BNode), ANode, BNode, l.NewEdgeMetadata(), "")
	if err != nil {
		logging.GetLogger().Error(err)
		return inputEdges
	}

	return append(inputEdges, edge)
}

// GetABLinks implementing graph.Linker
func (l *ABLinker) GetABLinks(aNode *graph.Node) (edges []*graph.Edge) {
	namespace, _ := aNode.GetFieldString(MetadataField("Namespace"))
	if a := l.ACache.GetByNode(aNode); a != nil {
		for _, b := range l.BCache.getByNamespace(namespace) {
			uid := b.(metav1.Object).GetUID()
			if bNode := l.Graph.GetNode(graph.Identifier(uid)); bNode != nil {
				if l.AreLinked(a, b) {
					edges = l.AppendNewEdge(edges, aNode, bNode)
				}
			}
		}
	}
	return
}

// GetBALinks implementing graph.Linker
func (l *ABLinker) GetBALinks(bNode *graph.Node) (edges []*graph.Edge) {
	namespace, _ := bNode.GetFieldString(MetadataField("Namespace"))
	if b := l.BCache.GetByNode(bNode); b != nil {
		for _, a := range l.ACache.getByNamespace(namespace) {
			uid := a.(metav1.Object).GetUID()
			if aNode := l.Graph.GetNode(graph.Identifier(uid)); aNode != nil {
				if l.AreLinked(a, b) {
					edges = l.AppendNewEdge(edges, aNode, bNode)
				}
			}
		}
	}
	return
}

// NewABLinker create and initialize an ABLinker based linker
func NewABLinker(g *graph.Graph, aManager, aType, bManager, bType string, outerLinker graph.Linker, areLinked AreLinked) probe.Probe {
	aProbe := GetSubprobe(aManager, aType)
	bProbe := GetSubprobe(bManager, bType)

	if aProbe == nil || bProbe == nil {
		return nil
	}

	innerLinker := outerLinker.(ABLinkerInterface).GetABLinker()
	innerLinker.Manager = aManager
	innerLinker.Type = aType
	innerLinker.Graph = g
	innerLinker.ACache = aProbe.(*ResourceCache)
	innerLinker.BCache = bProbe.(*ResourceCache)
	innerLinker.AreLinked = areLinked

	rl := graph.NewResourceLinker(
		g,
		[]graph.ListenerHandler{aProbe},
		[]graph.ListenerHandler{bProbe},
		outerLinker,
		graph.Metadata{"RelationType": aType},
	)

	linker := &Linker{
		ResourceLinker: rl,
	}
	rl.AddEventListener(linker)

	return linker
}
