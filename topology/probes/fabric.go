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

package probes

import (
	"fmt"
	"strings"

	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	FabricNamespace = "Fabric"
)

type fabricLink struct {
	parentMetadata graph.Metadata
	childMetadata  graph.Metadata
}

type FabricProbe struct {
	graph.DefaultGraphListener
	Graph *graph.Graph
	links map[*graph.Node]fabricLink
}

var fabricL2Link = graph.Metadata{"RelationType": "layer2", "Type": "fabric"}

type FabricRegisterLinkWSMessage struct {
	ParentNodeID   graph.Identifier
	ParentMetadata graph.Metadata
	ChildMetadata  graph.Metadata
}

func (fb *FabricProbe) OnEdgeAdded(e *graph.Edge) {
	parent, child := fb.Graph.GetEdgeNodes(e)
	if parent == nil || child == nil {
		return
	}

	for node, link := range fb.links {
		if parent.MatchMetadata(link.parentMetadata) && child.MatchMetadata(link.childMetadata) {
			fb.LinkNodes(node, child)
		}
	}
}

func (fb *FabricProbe) OnNodeDeleted(n *graph.Node) {
	if probe, ok := n.Metadata()["Probe"]; !ok || probe != "fabric" {
		delete(fb.links, n)
	}
}

func (fb *FabricProbe) LinkNodes(parent *graph.Node, child *graph.Node) {
	if !fb.Graph.AreLinked(child, parent, fabricL2Link) {
		fb.Graph.Link(parent, child, fabricL2Link)
	}
}

func nodeDefToMetadata(nodeDef string) (string, graph.Metadata, error) {
	f := strings.FieldsFunc(nodeDef, func(r rune) bool {
		return r == '[' || r == ']'
	})
	if len(f) > 2 {
		return "", nil, fmt.Errorf("Unable to parse node definition: %s", nodeDef)
	}

	// by default use the node name as Name metadata attribute
	metadata := graph.Metadata{}
	if f[0] != "*" {
		metadata["Name"] = f[0]
	}

	// parse attributes metadata given
	if len(f) > 1 {
		for _, pair := range strings.Split(f[1], ",") {
			pair = strings.TrimSpace(pair)

			kv := strings.Split(pair, "=")
			if len(kv)%2 != 0 {
				return "", nil, fmt.Errorf("attributes must be defined by pair k=v: %v", nodeDef)
			}
			key := strings.Trim(kv[0], `"`)
			value := strings.Trim(kv[1], `"`)

			metadata[key] = value
		}
	}

	return f[0], metadata, nil
}

func (fb *FabricProbe) getOrCreateFabricNodeFromDef(nodeDef string) (*graph.Node, error) {
	nodeName, metadata, err := nodeDefToMetadata(nodeDef)
	if err != nil {
		return nil, err
	}

	if _, ok := metadata["Type"]; !ok {
		metadata["Type"] = "device"
	}

	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte("fabric"+nodeName))
	id := graph.Identifier(u.String())

	if node := fb.Graph.GetNode(id); node != nil {
		return node, nil
	}

	metadata["Probe"] = "fabric"

	return fb.Graph.NewNode(id, metadata, ""), nil
}

func (fb *FabricProbe) Start() {
}

func (fb *FabricProbe) Stop() {
}

func NewFabricProbe(g *graph.Graph) *FabricProbe {
	fb := &FabricProbe{
		Graph: g,
		links: make(map[*graph.Node]fabricLink),
	}

	g.AddEventListener(fb)

	fb.Graph.Lock()
	defer fb.Graph.Unlock()

	list := config.GetConfig().GetStringSlice("analyzer.topology.fabric")
	for _, link := range list {
		pc := strings.Split(link, "->")
		if len(pc) != 2 {
			logging.GetLogger().Errorf("FabricProbe link definition should have two endpoint: %s", link)
			continue
		}

		parentDef := strings.TrimSpace(pc[0])
		childDef := strings.TrimSpace(pc[1])

		if strings.HasPrefix(parentDef, "*") {
			logging.GetLogger().Error("FabricProbe doesn't support wildcard node as parent node")
			continue
		}

		if strings.HasPrefix(childDef, "*") {
			nodes := strings.SplitN(childDef, "/", 2)
			if len(nodes) < 2 {
				logging.GetLogger().Error("Invalid wildcard pattern: " + childDef)
				continue
			}

			// Fabric Node to Incoming Node

			parentNode, err := fb.getOrCreateFabricNodeFromDef(parentDef)
			if err != nil {
				logging.GetLogger().Error(err.Error())
				continue
			}

			_, parentMetadata, err := nodeDefToMetadata(nodes[0])
			if err != nil {
				logging.GetLogger().Error(err.Error())
				continue
			}

			_, childMetadata, err := nodeDefToMetadata(nodes[1])
			if err != nil {
				logging.GetLogger().Error(err.Error())
				continue
			}

			// queue it as the node doesn't exist at start
			fb.links[parentNode] = fabricLink{childMetadata: childMetadata, parentMetadata: parentMetadata}
		} else {
			// Fabric Node to Fabric Node
			node1, err := fb.getOrCreateFabricNodeFromDef(parentDef)
			if err != nil {
				logging.GetLogger().Error(err.Error())
				continue
			}

			node2, err := fb.getOrCreateFabricNodeFromDef(childDef)
			if err != nil {
				logging.GetLogger().Error(err.Error())
				continue
			}

			if !fb.Graph.AreLinked(node1, node2) {
				fb.Graph.Link(node1, node2, fabricL2Link)
			}
		}
	}

	return fb
}
