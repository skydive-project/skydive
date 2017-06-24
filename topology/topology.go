/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package topology

import (
	"fmt"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// Describe the relation type between nodes
const (
	OwnershipLink = "ownership"
	Layer2Link    = "layer2"
)

// Describe the relation type between nodes in the graph
var (
	OwnershipMetadata = graph.Metadata{"RelationType": OwnershipLink}
	Layer2Metadata    = graph.Metadata{"RelationType": Layer2Link}
)

// NodePath describes a path in the graph between 2 nodes
type NodePath []*graph.Node

// Marshal Serialize the node path
func (p NodePath) Marshal() string {
	var path string
	for i := len(p) - 1; i >= 0; i-- {
		if len(path) > 0 {
			path += "/"
		}

		metadata := p[i].Metadata()
		n := metadata["Name"]
		t := metadata["Type"]

		if n == nil || t == nil {
			return ""
		}

		path += fmt.Sprintf("%s[Type=%s]", n.(string), t.(string))
	}

	return path
}

// GraphPath returns a string representation of the shortestpath between 2 host on the same owner
func GraphPath(g *graph.Graph, n *graph.Node) string {
	nodes := g.LookupShortestPath(n, graph.Metadata{"Type": "host"}, graph.Metadata{"RelationType": "ownership"})
	if len(nodes) > 0 {
		return NodePath(nodes).Marshal()
	}
	return ""
}

// NamespaceFromNode returns the namespace name and the path of a node in the graph
func NamespaceFromNode(g *graph.Graph, n *graph.Node) (string, string, error) {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		return "", "", fmt.Errorf("No Name for node %v", n)
	}

	nodes := g.LookupShortestPath(n, graph.Metadata{"Type": "host"}, graph.Metadata{"RelationType": "ownership"})
	if len(nodes) == 0 {
		return "", "", fmt.Errorf("Failed to determine probePath for %s", name)
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
func NewNetNSContextByNode(g *graph.Graph, n *graph.Node) (*common.NetNSContext, error) {
	name, path, err := NamespaceFromNode(g, n)
	if err != nil || name == "" || path == "" {
		return nil, err
	}

	logging.GetLogger().Debugf("Switching to namespace %s (path: %s)", name, path)
	return common.NewNetNsContext(path)
}

// HostNodeTIDMap a map that store the value node TID and the key node host
type HostNodeTIDMap map[string][]string

// BuildHostNodeTIDMap creates a new node and host (key) map
func BuildHostNodeTIDMap(nodes []*graph.Node) HostNodeTIDMap {
	hnmap := make(HostNodeTIDMap)
	for _, node := range nodes {
		if tid, _ := node.GetFieldString("TID"); tid != "" {
			hnmap[node.Host()] = append(hnmap[node.Host()], tid)
		}
	}
	return hnmap
}

// HaveOwnershipLink returns true parent and child have the same ownership
func HaveOwnershipLink(g *graph.Graph, parent *graph.Node, child *graph.Node, metadata graph.Metadata) bool {
	// do not add or change original metadata
	m := metadata.Clone()
	m["RelationType"] = OwnershipLink

	return g.AreLinked(parent, child, m)
}

// AddOwnershipLink Link the parent and the child node, the child can have only one parent, previous will be overwritten
func AddOwnershipLink(g *graph.Graph, parent *graph.Node, child *graph.Node, metadata graph.Metadata) *graph.Edge {
	// a child node can only have one parent of type ownership, so delete the previous link
	for _, e := range g.GetNodeEdges(child, graph.Metadata{"RelationType": OwnershipLink}) {
		if e.GetChild() == child.ID {
			logging.GetLogger().Debugf("Delete previous ownership link: %v", e)
			g.DelEdge(e)
		}
	}

	// do not add or change original metadata
	m := metadata.Clone()
	m["RelationType"] = OwnershipLink

	return g.Link(parent, child, m)
}

// HaveLayer2Link returns true if parent and child have the same layer 2
func HaveLayer2Link(g *graph.Graph, node1 *graph.Node, node2 *graph.Node, metadata graph.Metadata) bool {
	// do not add or change original metadata
	m := metadata.Clone()
	m["RelationType"] = Layer2Link

	return g.AreLinked(node1, node2, m)
}

// AddLayer2Link Link the parent and the child node
func AddLayer2Link(g *graph.Graph, node1 *graph.Node, node2 *graph.Node, metadata graph.Metadata) *graph.Edge {
	// do not add or change original metadata
	m := metadata.Clone()
	m["RelationType"] = Layer2Link

	return g.Link(node1, node2, m)
}
