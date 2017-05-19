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
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	FabricNamespace = "Fabric"
)

type fabricLink struct {
	parentMetadata graph.Metadata
	childMetadata  graph.Metadata
	linkMetadata   graph.Metadata
}

type FabricProbe struct {
	graph.DefaultGraphListener
	Graph *graph.Graph
	links map[*graph.Node][]fabricLink
}

type FabricRegisterLinkWSMessage struct {
	ParentNodeID   graph.Identifier
	ParentMetadata graph.Metadata
	ChildMetadata  graph.Metadata
}

func (fb *FabricProbe) OnEdgeAdded(e *graph.Edge) {
	parents, children := fb.Graph.GetEdgeNodes(e, graph.Metadata{}, graph.Metadata{})
	if len(parents) == 0 || len(children) == 0 {
		return
	}

	parent, child := parents[0], children[0]
	for node, links := range fb.links {
		for _, link := range links {
			if parent.MatchMetadata(link.parentMetadata) && child.MatchMetadata(link.childMetadata) {
				fb.LinkNodes(node, child, &link.linkMetadata)
			}
		}
	}
}

func (fb *FabricProbe) OnNodeDeleted(n *graph.Node) {
	if probe, _ := n.GetFieldString("Probe"); probe != "fabric" {
		delete(fb.links, n)
	}
}

func (fb *FabricProbe) LinkNodes(parent *graph.Node, child *graph.Node, linkMetadata *graph.Metadata) {
	if !topology.HaveLayer2Link(fb.Graph, child, parent, *linkMetadata) {
		topology.AddLayer2Link(fb.Graph, parent, child, *linkMetadata)
	}
}

func defToMetadata(def string, metadata graph.Metadata) (graph.Metadata, error) {
	for _, pair := range strings.Split(def, ",") {
		pair = strings.TrimSpace(pair)

		kv := strings.Split(pair, "=")
		if len(kv)%2 != 0 {
			return nil, fmt.Errorf("attributes must be defined by pair k=v: %v", def)
		}
		key := strings.Trim(kv[0], `"`)
		value := strings.Trim(kv[1], `"`)

		metadata[key] = value
	}

	return metadata, nil
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

	var err error
	// parse attributes metadata given
	if len(f) > 1 {
		metadata, err = defToMetadata(f[1], metadata)
	}

	return f[0], metadata, err
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
		links: make(map[*graph.Node][]fabricLink),
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
		linkMetadata := graph.Metadata{"Type": "fabric"}

		if strings.HasPrefix(childDef, "[") {
			// Parse link metadata
			index := strings.Index(childDef, "]")
			f := strings.FieldsFunc(childDef[:index+1], func(r rune) bool {
				return r == '[' || r == ']'
			})
			linkMetadata, _ = defToMetadata(f[0], linkMetadata)
			childDef = strings.TrimSpace(childDef[index+1:])
		}

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
			fb.links[parentNode] = append(fb.links[parentNode], fabricLink{childMetadata: childMetadata, parentMetadata: parentMetadata, linkMetadata: linkMetadata})
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

			if !topology.HaveOwnershipLink(fb.Graph, node1, node2, nil) {
				topology.AddOwnershipLink(fb.Graph, node1, node2, nil)
				topology.AddLayer2Link(fb.Graph, node1, node2, linkMetadata)
			}
		}
	}

	return fb
}
