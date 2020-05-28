/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package topology

import (
	"fmt"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/netns"
)

// ErrNetworkPathNotFound
const (
	OwnershipLink = "ownership"
	Layer2Link    = "layer2"
)

var (
	// ErrNoPathToHost is called when no host could be found as the parent of a node
	ErrNoPathToHost = func(name string) error { return fmt.Errorf("Failed to determine network namespace path for %s", name) }
)

// OwnershipMetadata returns metadata for an ownership link
func OwnershipMetadata() graph.Metadata {
	return graph.Metadata{"RelationType": OwnershipLink}
}

// Layer2Metadata returns metadata for a layer2 link
func Layer2Metadata() graph.Metadata {
	return graph.Metadata{"RelationType": Layer2Link}
}

// NamespaceFromNode returns the namespace name and the path of a node in the graph
func NamespaceFromNode(g *graph.Graph, n *graph.Node) (string, string, error) {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return "", "", fmt.Errorf("No Name for node %v", n)
	}

	nodes := g.LookupShortestPath(n, graph.Metadata{"Type": "host"}, OwnershipMetadata())
	if len(nodes) == 0 {
		return "", "", ErrNoPathToHost(name)
	}

	for _, node := range nodes {
		tp, _ := node.GetFieldString("Type")
		if tp == "" {
			return "", "", fmt.Errorf("No Type for node %v", n)
		}

		if tp == "netns" {
			name, _ := node.GetFieldString("Name")
			if name == "" {
				return "", "", fmt.Errorf("No Name for node %v", node)
			}

			path, _ := node.GetFieldString("Path")
			if path == "" {
				return "", "", fmt.Errorf("No Path for node %v", node)
			}

			return name, path, nil
		}
	}

	return "", "", nil
}

// NewNetNSContextByNode creates a new network namespace context based on the node
func NewNetNSContextByNode(g *graph.Graph, n *graph.Node) (*netns.Context, error) {
	name, path, err := NamespaceFromNode(g, n)
	if err != nil || name == "" || path == "" {
		return nil, err
	}

	logging.GetLogger().Debugf("Switching to namespace %s (path: %s)", name, path)
	return netns.NewContext(path)
}

// HostNodeTIDMap a map that store the value node TID and the key node host
type HostNodeTIDMap map[string][]string

// BuildHostNodeTIDMap creates a new node and host (key) map
func BuildHostNodeTIDMap(nodes []*graph.Node) HostNodeTIDMap {
	hnmap := make(HostNodeTIDMap)
	for _, node := range nodes {
		if tid, _ := node.GetFieldString("TID"); tid != "" {
			hnmap[node.Host] = append(hnmap[node.Host], tid)
		}
	}
	return hnmap
}

// HaveOwnershipLink returns true if parent and child have an ownership link
func HaveOwnershipLink(g *graph.Graph, parent *graph.Node, child *graph.Node) bool {
	return HaveLink(g, parent, child, OwnershipLink)
}

// IsOwnershipLinked checks whether the node has an OwnershipLink
func IsOwnershipLinked(g *graph.Graph, node *graph.Node) bool {
	edges := g.GetNodeEdges(node, OwnershipMetadata())
	return len(edges) != 0
}

// GetOwnershipLink get ownership Link between the parent and the child node or nil
func GetOwnershipLink(g *graph.Graph, parent *graph.Node, child *graph.Node) *graph.Edge {
	return g.GetFirstLink(parent, child, OwnershipMetadata())
}

// AddOwnershipLink Link between the parent and the child node, the child can have only one parent, previous will be overwritten
func AddOwnershipLink(g *graph.Graph, parent *graph.Node, child *graph.Node, metadata graph.Metadata) (*graph.Edge, error) {
	// a child node can only have one parent of type ownership, so delete the previous link
	for _, e := range g.GetNodeEdges(child, OwnershipMetadata()) {
		if e.Child == child.ID {
			logging.GetLogger().Debugf("Delete previous ownership link: %v", e)
			if err := g.DelEdge(e); err != nil {
				return nil, err
			}
		}
	}

	return AddLink(g, parent, child, OwnershipLink, metadata)
}

// HaveLink returns true if parent and child are linked
func HaveLink(g *graph.Graph, node1 *graph.Node, node2 *graph.Node, relationType string) bool {
	return g.AreLinked(node1, node2, graph.Metadata{"RelationType": relationType})
}

// NewLink creates a link between a parent and a child node with the specified relation type and metadata
func NewLink(g *graph.Graph, node1 *graph.Node, node2 *graph.Node, relationType string, metadata graph.Metadata) (*graph.Edge, error) {
	m := graph.Metadata{"RelationType": relationType}
	for k, v := range metadata {
		m[k] = v
	}

	id, _ := uuid.NewV5(uuid.NamespaceOID, []byte(node1.ID+node2.ID+graph.Identifier(relationType)))
	edge := g.CreateEdge(graph.Identifier(id.String()), node1, node2, m, graph.Time(time.Now()))
	if edge == nil {
		return nil, fmt.Errorf("Failed to create edge with id %s", id.String())
	}
	return edge, nil
}

// AddLink links the parent and the child node with the specified relation type and metadata
func AddLink(g *graph.Graph, node1 *graph.Node, node2 *graph.Node, relationType string, metadata graph.Metadata) (*graph.Edge, error) {
	edge, err := NewLink(g, node1, node2, relationType, metadata)
	if err != nil {
		return nil, err
	}
	return edge, g.AddEdge(edge)
}

// HaveLayer2Link returns true if parent and child have the same layer 2
func HaveLayer2Link(g *graph.Graph, node1 *graph.Node, node2 *graph.Node) bool {
	return HaveLink(g, node1, node2, Layer2Link)
}

// AddLayer2Link links the parent and the child node
func AddLayer2Link(g *graph.Graph, node1 *graph.Node, node2 *graph.Node, metadata graph.Metadata) (*graph.Edge, error) {
	return AddLink(g, node1, node2, Layer2Link, metadata)
}

// IsInterfaceUp returns whether an interface has the flag UP set
func IsInterfaceUp(node *graph.Node) bool {
	linkFlags, _ := node.GetFieldStringList("LinkFlags")
	for _, flag := range linkFlags {
		if flag == "UP" {
			return true
		}
	}
	return false
}
