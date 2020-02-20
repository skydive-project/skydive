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

package server

import (
	"errors"
	"fmt"
	"io"

	"github.com/skydive-project/skydive/flow"
	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
)

func shortID(s graph.Identifier) graph.Identifier {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func graphToDot(w io.Writer, g *graph.Graph) {
	g.RLock()
	defer g.RUnlock()

	w.Write([]byte("digraph g {\n"))

	nodeMap := make(map[graph.Identifier]*graph.Node)
	for _, n := range g.GetNodes(nil) {
		nodeMap[n.ID] = n
		name, _ := n.GetFieldString("Name")
		title := fmt.Sprintf("%s-%s", name, shortID(n.ID))
		label := title
		for k, v := range n.Metadata {
			switch k {
			case "Type", "IfIndex", "State", "TID", "IPV4", "IPV6":
				label += fmt.Sprintf("\\n%s = %v", k, v)
			}
		}
		w.Write([]byte(fmt.Sprintf("\"%s\" [label=\"%s\"]\n", title, label)))
	}

	for _, e := range g.GetEdges(nil) {
		parent := nodeMap[e.Parent]
		child := nodeMap[e.Child]
		if parent == nil || child == nil {
			continue
		}

		childName, _ := child.GetFieldString("Name")
		parentName, _ := parent.GetFieldString("Name")
		relationType, _ := e.GetFieldString("RelationType")
		linkLabel, linkType, direction := "", "->", "forward"
		switch relationType {
		case "":
		case "layer2":
			direction = "both"
			fallthrough
		default:
			linkLabel = fmt.Sprintf(" [label=%s,dir=%s]\n", relationType, direction)
		}
		link := fmt.Sprintf("\"%s-%s\" %s \"%s-%s\"%s", parentName, shortID(parent.ID), linkType, childName, shortID(child.ID), linkLabel)
		w.Write([]byte(link))
	}

	w.Write([]byte("}"))
}

// dotMarshaller outputs a GraversalStep as dot
func dotMarshaller(step traversal.GraphTraversalStep, w io.Writer) error {
	if graphTraversal, ok := step.(*traversal.GraphTraversal); ok {
		graphToDot(w, graphTraversal.Graph)
		return nil
	}
	return errors.New("Only graph can be outputted as dot")
}

// pcapMarshaller outputs a RawPacketsTraversalStep as pcap trace
func pcapMarshaller(res traversal.GraphTraversalStep, w io.Writer) error {
	rawPacketsTraversal, ok := res.(*ge.RawPacketsTraversalStep)
	if !ok {
		return errors.New("Only RawPackets step result can be outputted as pcap")
	}

	values := rawPacketsTraversal.Values()
	if len(values) == 0 {
		return nil
	}

	pw := flow.NewPcapWriter(w)
	for _, pf := range values {
		m := pf.(map[string][]*flow.RawPacket)
		for _, fr := range m {
			if err := pw.WriteRawPackets(fr); err != nil {
				return err
			}
		}
	}

	return nil
}

// TopologyMarshallers hold the dot and pcap marshallers
var TopologyMarshallers = api.TopologyMarshallers{
	"text/vnd.graphviz":            dotMarshaller,
	"application/vnd.tcpdump.pcap": pcapMarshaller,
}
