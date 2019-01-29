/*
 * Copyright 2018 IBM Corp.
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
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AreLinked return true if (a, b) should be linked
type AreLinked func(a, b interface{}) bool

// ABLinker basis for a simple A to B linker
type ABLinker struct {
	manager   string
	typ       string
	graph     *graph.Graph
	aCache    *ResourceCache
	bCache    *ResourceCache
	areLinked AreLinked
}

func (l *ABLinker) newEdge(parent, child *graph.Node) *graph.Edge {
	id := graph.GenID(string(parent.ID), string(child.ID), "RelationType", l.typ)
	m := NewEdgeMetadata(l.manager, l.typ)
	return l.graph.CreateEdge(id, parent, child, m, graph.TimeUTC(), "")
}

// GetABLinks implementing graph.Linker
func (l *ABLinker) GetABLinks(aNode *graph.Node) (edges []*graph.Edge) {
	namespace, _ := aNode.GetFieldString(MetadataField("Namespace"))
	if a := l.aCache.GetByNode(aNode); a != nil {
		for _, b := range l.bCache.getByNamespace(namespace) {
			uid := b.(metav1.Object).GetUID()
			if bNode := l.graph.GetNode(graph.Identifier(uid)); bNode != nil {
				if l.areLinked(a, b) {
					edges = append(edges, l.newEdge(aNode, bNode))
				}
			}
		}
	}
	return
}

// GetBALinks implementing graph.Linker
func (l *ABLinker) GetBALinks(bNode *graph.Node) (edges []*graph.Edge) {
	namespace, _ := bNode.GetFieldString(MetadataField("Namespace"))
	if b := l.bCache.GetByNode(bNode); b != nil {
		for _, a := range l.aCache.getByNamespace(namespace) {
			uid := a.(metav1.Object).GetUID()
			if aNode := l.graph.GetNode(graph.Identifier(uid)); aNode != nil {
				if l.areLinked(a, b) {
					edges = append(edges, l.newEdge(aNode, bNode))
				}
			}
		}
	}
	return
}

// NewABLinker create and initialize an ABLinker based linker
func NewABLinker(g *graph.Graph, aManager, aType, bManager, bType string, areLinked AreLinked) probe.Probe {
	aProbe := GetSubprobe(aManager, aType)
	bProbe := GetSubprobe(bManager, bType)

	if aProbe == nil || bProbe == nil {
		return nil
	}

	innerLinker := new(ABLinker)
	innerLinker.manager = aManager
	innerLinker.typ = aType
	innerLinker.graph = g
	innerLinker.aCache = aProbe.(*ResourceCache)
	innerLinker.bCache = bProbe.(*ResourceCache)
	innerLinker.areLinked = areLinked

	rl := graph.NewResourceLinker(
		g,
		[]graph.ListenerHandler{aProbe},
		[]graph.ListenerHandler{bProbe},
		innerLinker,
		graph.Metadata{"RelationType": aType},
	)

	linker := &Linker{
		ResourceLinker: rl,
	}
	rl.AddEventListener(linker)

	return linker
}
