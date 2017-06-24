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

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/abbot/go-http-auth"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	"github.com/skydive-project/skydive/validator"
)

// TopologyAPI exposes the topology query API
type TopologyAPI struct {
	gremlinParser *traversal.GremlinTraversalParser
}

// TopologyParam topology API parameter
type TopologyParam struct {
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
}

func (t *TopologyAPI) graphToDot(w http.ResponseWriter, g *graph.Graph) {
	g.RLock()
	defer g.RUnlock()

	w.Write([]byte("digraph g {\n"))

	nodeMap := make(map[graph.Identifier]*graph.Node)
	for _, n := range g.GetNodes(nil) {
		nodeMap[n.ID] = n
		name, _ := n.GetFieldString("Name")
		title := fmt.Sprintf("%s-%s", name, n.ID[:7])
		label := title
		for k, v := range n.Metadata() {
			switch k {
			case "Type", "IfIndex", "State", "TID":
				label += fmt.Sprintf("\\n%s = %v", k, v)
			}
		}
		w.Write([]byte(fmt.Sprintf("\"%s\" [label=\"%s\"]\n", title, label)))
	}

	for _, e := range g.GetEdges(nil) {
		parent := nodeMap[e.GetParent()]
		child := nodeMap[e.GetChild()]
		if parent == nil || child == nil {
			continue
		}

		childName, _ := child.GetFieldString("Name")
		parentName, _ := parent.GetFieldString("Name")
		relationType, _ := e.GetFieldString("RelationType")
		linkLabel, linkType := "", "->"
		switch relationType {
		case "":
		case "layer2":
			linkType = "--"
			fallthrough
		default:
			linkLabel = fmt.Sprintf(" [label=%s]\n", relationType)
		}
		link := fmt.Sprintf("\"%s-%s\" %s \"%s-%s\"%s", parentName, parent.ID[:7], linkType, childName, child.ID[:7], linkLabel)
		w.Write([]byte(link))
	}

	w.Write([]byte("}"))
}

func (t *TopologyAPI) topologyIndex(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	g := t.gremlinParser.Graph
	g.RLock()
	defer g.RUnlock()

	w.WriteHeader(http.StatusOK)
	if strings.Contains(r.Header.Get("Accept"), "vnd.graphviz") {
		w.Header().Set("Content-Type", "text/vnd.graphviz; charset=UTF-8")
		t.graphToDot(w, g)
	} else {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		if err := json.NewEncoder(w).Encode(g); err != nil {
			panic(err)
		}
	}
}

func (t *TopologyAPI) topologySearch(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	resource := TopologyParam{}

	data, _ := ioutil.ReadAll(r.Body)
	if len(data) != 0 {
		if err := json.Unmarshal(data, &resource); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := validator.Validate(resource); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	if resource.GremlinQuery == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ts, err := t.gremlinParser.Parse(strings.NewReader(resource.GremlinQuery), true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	res, err := ts.Exec()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "vnd.graphviz") {
		if graphTraversal, ok := res.(*traversal.GraphTraversal); ok {
			w.Header().Set("Content-Type", "text/vnd.graphviz; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			t.graphToDot(w, graphTraversal.Graph)
		} else {
			writeError(w, http.StatusNotAcceptable, errors.New("Only graph can be outputted as dot"))
		}
	} else {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		if err := json.NewEncoder(w).Encode(res); err != nil {
			panic(err)
		}
	}
}

func (t *TopologyAPI) registerEndpoints(r *shttp.Server) {
	routes := []shttp.Route{
		{
			Name:        "TopologiesIndex",
			Method:      "GET",
			Path:        "/api/topology",
			HandlerFunc: t.topologyIndex,
		},
		{
			Name:        "TopologiesSearch",
			Method:      "POST",
			Path:        "/api/topology",
			HandlerFunc: t.topologySearch,
		},
	}

	r.RegisterRoutes(routes)
}

// RegisterTopologyAPI registers a new topology query API
func RegisterTopologyAPI(r *shttp.Server, parser *traversal.GremlinTraversalParser) {
	t := &TopologyAPI{
		gremlinParser: parser,
	}

	t.registerEndpoints(r)
}
