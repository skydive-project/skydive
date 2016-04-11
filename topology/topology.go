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
	"strings"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

type NodePath struct {
	Nodes []*graph.Node
}

func (p NodePath) Marshal() string {
	var path string
	for i := len(p.Nodes) - 1; i >= 0; i-- {
		if len(path) > 0 {
			path += "/"
		}

		metadata := p.Nodes[i].Metadata()
                n := metadata["Name"]
                t := metadata["Type"]

                if n == nil || t == nil {
			return ""
                }

		path += fmt.Sprintf("%s[Type=%s]", n.(string), t.(string))
	}

	return path
}

func nodePathPartToMetadata(p string) (graph.Metadata, error) {
	f := strings.FieldsFunc(p, func(r rune) bool {
		return r == '[' || r == ']'
	})
	if len(f) != 2 {
		return nil, fmt.Errorf("Unable to parse part of node path: %s", p)
	}

	n := f[0]
	t := strings.Split(f[1], "=")
	if len(t) != 2 || t[0] != "Type" {
		return nil, fmt.Errorf("Unable to parse part of node path: %s", p)
	}

	return graph.Metadata{
		"Name": n,
		"Type": t[1],
	}, nil
}

func LookupNodeFromNodePathString(g *graph.Graph, s string) *graph.Node {
	parts := strings.Split(s, "/")

	m, err := nodePathPartToMetadata(parts[0])
	if err != nil {
		logging.GetLogger().Errorf("Unable to parse a part of node path string (%s): %s", parts[0], err.Error())
		return nil
	}

	node := g.LookupFirstNode(m)
	if node == nil {
		return nil
	}

	for _, part := range parts[1:] {
		m, err := nodePathPartToMetadata(part)
		if err != nil {
			return nil
		}

		node = g.LookupFirstChild(node, m)
		if node == nil {
			return nil
		}
	}

	return node
}

func IsOwnershipEdge(e *graph.Edge) bool {
	if t, ok := e.Metadata()["RelationType"]; ok && t.(string) == "ownership" {
		return true
	}
	return false
}

func IsLayer2Edge(e *graph.Edge) bool {
	if t, ok := e.Metadata()["RelationType"]; ok && t.(string) == "layer2" {
		return true
	}
	return false
}
