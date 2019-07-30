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

package fabric

import (
	"errors"
	"fmt"
	"strings"

	uuid "github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology"
)

type fabricLink struct {
	parentMetadata graph.Metadata
	childMetadata  graph.Metadata
	linkMetadata   graph.Metadata
}

// Probe describes a topology probe
type Probe struct {
	graph.DefaultGraphListener
	Graph *graph.Graph
	links map[*graph.Node][]fabricLink
}

// OnEdgeAdded event
func (fb *Probe) OnEdgeAdded(e *graph.Edge) {
	parents, children := fb.Graph.GetEdgeNodes(e, nil, nil)
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

// OnNodeDeleted event
func (fb *Probe) OnNodeDeleted(n *graph.Node) {
	if probe, _ := n.GetFieldString("Probe"); probe != "fabric" {
		delete(fb.links, n)
	}
}

// LinkNodes link the parent and child (layer 2) if there not linked already
func (fb *Probe) LinkNodes(parent *graph.Node, child *graph.Node, linkMetadata *graph.Metadata) {
	if !topology.HaveLayer2Link(fb.Graph, child, parent) {
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

func (fb *Probe) getOrCreateFabricNodeFromDef(nodeDef string) (*graph.Node, error) {
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

	return fb.Graph.NewNode(id, metadata)
}

// Start the probe
func (fb *Probe) Start() error {
	fb.Graph.Lock()
	defer fb.Graph.Unlock()

	fb.Graph.AddEventListener(fb)

	for _, link := range config.GetStringSlice("analyzer.topology.fabric") {
		l2OnlyLink := false
		pc := strings.Split(link, "-->")
		if len(pc) == 2 {
			l2OnlyLink = true
		} else {
			pc = strings.Split(link, "->")
		}

		if len(pc) != 2 || pc[0] == "" || pc[1] == "" {
			return fmt.Errorf("Fabric link definition should have two endpoints: %s", link)
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
			return errors.New("Fabric probe doesn't support wildcard node as parent node")
		}

		if strings.HasPrefix(childDef, "*") {
			nodes := strings.SplitN(childDef, "/", 2)
			if len(nodes) < 2 {
				return fmt.Errorf("Invalid wildcard pattern: %s" + childDef)
			}

			parentNode, err := fb.getOrCreateFabricNodeFromDef(parentDef)
			if err != nil {
				return err
			}

			_, parentMetadata, err := nodeDefToMetadata(nodes[0])
			if err != nil {
				return err
			}

			_, childMetadata, err := nodeDefToMetadata(nodes[1])
			if err != nil {
				return err
			}

			// queue it as the node doesn't exist at start
			fb.links[parentNode] = append(fb.links[parentNode], fabricLink{childMetadata: childMetadata, parentMetadata: parentMetadata, linkMetadata: linkMetadata})
		} else {
			// Fabric Node to Fabric Node
			node1, err := fb.getOrCreateFabricNodeFromDef(parentDef)
			if err != nil {
				return err
			}

			node2, err := fb.getOrCreateFabricNodeFromDef(childDef)
			if err != nil {
				return err
			}

			if !topology.HaveOwnershipLink(fb.Graph, node1, node2) {
				if !l2OnlyLink {
					topology.AddOwnershipLink(fb.Graph, node1, node2, nil)
				}
				topology.AddLayer2Link(fb.Graph, node1, node2, linkMetadata)
			}
		}
	}

	return nil
}

// Stop the probe
func (fb *Probe) Stop() {
	fb.Graph.RemoveEventListener(fb)
}

// NewProbe creates a new probe to enhance the graph
func NewProbe(g *graph.Graph) (*Probe, error) {
	return &Probe{
		Graph: g,
		links: make(map[*graph.Node][]fabricLink),
	}, nil
}
