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

type fabricLink struct {
	metadata graph.Metadata
}

type FabricProbe struct {
	graph.DefaultGraphListener
	Graph *graph.Graph
	links map[*graph.Node]fabricLink
}

func (fb *FabricProbe) OnEdgeAdded(e *graph.Edge) {
	parent, child := fb.Graph.GetEdgeNodes(e)
	if parent == nil || child == nil || e.Metadata()["RelationType"] != "ownership" {
		return
	}

	if parent.Metadata()["Type"] == "host" {
		for node, link := range fb.links {
			if child.MatchMetadata(link.metadata) {
				if !fb.Graph.AreLinked(child, node) {
					fb.Graph.Link(node, child, graph.Metadata{"RelationType": "layer2", "Type": "fabric"})
				}
				delete(fb.links, node)
				break
			}
		}
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
	metadata := graph.Metadata{
		"Name": f[0],
		"Type": "device",
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

func (fb *FabricProbe) addFabricNodeFromDef(nodeDef string) (*graph.Node, error) {
	nodeName, metadata, err := nodeDefToMetadata(nodeDef)
	if err != nil {
		return nil, err
	}

	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte("fabric"+nodeName))
	id := graph.Identifier(u.String())

	if node := fb.Graph.GetNode(id); node != nil {
		return node, nil
	}
	metadata["Probe"] = "fabric"

	return fb.Graph.NewNode(id, metadata), nil
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

	list := config.GetConfig().GetStringSlice("agent.topology.fabric")
	for _, link := range list {
		pc := strings.Split(link, "->")
		if len(pc) != 2 {
			logging.GetLogger().Errorf("FabricProbe link definition should have two endpoint: %s", link)
			continue
		}

		parentDef := strings.TrimSpace(pc[0])
		childDef := strings.TrimSpace(pc[1])

		if strings.HasPrefix(parentDef, "local/") {
			logging.GetLogger().Error("FabricProbe doesn't support local node as parent node")
			continue
		}

		if strings.HasPrefix(childDef, "local/") {
			// Fabric Node to Local Node
			childDef = strings.TrimPrefix(childDef, "local/")

			parentNode, err := fb.addFabricNodeFromDef(parentDef)
			if err != nil {
				logging.GetLogger().Error(err.Error())
				continue
			}

			_, childMetadata, err := nodeDefToMetadata(childDef)
			if err != nil {
				logging.GetLogger().Error(err.Error())
				continue
			}

			// queue it as the local doesn't exist at start
			fb.links[parentNode] = fabricLink{metadata: childMetadata}
		} else {
			// Fabric Node to Fabric Node
			node1, err := fb.addFabricNodeFromDef(parentDef)
			if err != nil {
				logging.GetLogger().Error(err.Error())
				continue
			}

			node2, err := fb.addFabricNodeFromDef(childDef)
			if err != nil {
				logging.GetLogger().Error(err.Error())
				continue
			}

			if !fb.Graph.AreLinked(node1, node2) {
				fb.Graph.Link(node1, node2, graph.Metadata{"RelationType": "layer2", "Type": "fabric"})
			}
		}
	}

	return fb
}
