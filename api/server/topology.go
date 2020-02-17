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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	auth "github.com/abbot/go-http-auth"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/rbac"
	"github.com/skydive-project/skydive/validator"
)

// TopologyAPI exposes the topology query API
type TopologyAPI struct {
	graph         *graph.Graph
	gremlinParser *traversal.GremlinTraversalParser
}

func shortID(s graph.Identifier) graph.Identifier {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func (t *TopologyAPI) graphToDot(w io.Writer, g *graph.Graph) {
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

func (t *TopologyAPI) topologyIndex(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	if !rbac.Enforce(r.Username, "topology", "read") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// use a buffer to render the result in order to limit the lock time
	// if the client is slow
	var b bytes.Buffer

	w.WriteHeader(http.StatusOK)
	if strings.Contains(r.Header.Get("Accept"), "vnd.graphviz") {
		w.Header().Set("Content-Type", "text/vnd.graphviz; charset=UTF-8")
		t.graphToDot(&b, t.graph)
	} else {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")

		t.graph.RLock()
		if err := json.NewEncoder(&b).Encode(t.graph); err != nil {
			t.graph.RUnlock()
			writeError(w, http.StatusNotAcceptable, fmt.Errorf("Error while encoding response: %s", err))
			return
		}
		t.graph.RUnlock()
	}

	if _, err := w.Write(b.Bytes()); err != nil {
		logging.GetLogger().Errorf("Error while writing response: %s", err)
	}
}

func (t *TopologyAPI) topologySearch(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	if !rbac.Enforce(r.Username, "topology", "read") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	resource := types.TopologyParams{}
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

	ts, err := t.gremlinParser.Parse(strings.NewReader(resource.GremlinQuery))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	res, err := ts.Exec(t.graph, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// use a buffer to render the result in order to limit the lock time
	// if the client is slow
	var b bytes.Buffer

	if strings.Contains(r.Header.Get("Accept"), "vnd.graphviz") {
		if graphTraversal, ok := res.(*traversal.GraphTraversal); ok {
			w.Header().Set("Content-Type", "text/vnd.graphviz; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			t.graphToDot(&b, graphTraversal.Graph)
		} else {
			writeError(w, http.StatusNotAcceptable, errors.New("Only graph can be outputted as dot"))
			return
		}
	} else if strings.Contains(r.Header.Get("Accept"), "vnd.tcpdump.pcap") {
		if rawPacketsTraversal, ok := res.(*ge.RawPacketsTraversalStep); ok {
			values := rawPacketsTraversal.Values()
			if len(values) == 0 {
				writeError(w, http.StatusNotFound, errors.New("No raw packet found, please check your Gremlin request and the time context"))
				return
			}
			w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap; charset=UTF-8")
			w.WriteHeader(http.StatusOK)

			pw := flow.NewPcapWriter(&b)
			for _, pf := range values {
				m := pf.(map[string][]*flow.RawPacket)
				for _, fr := range m {
					if err = pw.WriteRawPackets(fr); err != nil {
						writeError(w, http.StatusNotAcceptable, err)
						return
					}
				}
			}
		} else {
			writeError(w, http.StatusNotAcceptable, errors.New("Only RawPackets step result can be outputted as pcap"))
			return
		}
	} else {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(&b).Encode(res); err != nil {
			writeError(w, http.StatusNotAcceptable, fmt.Errorf("Error while encoding response: %s", err))
			return
		}
	}

	if _, err := w.Write(b.Bytes()); err != nil {
		logging.GetLogger().Errorf("Error while writing response: %s", err)
	}
}

func (t *TopologyAPI) registerEndpoints(r *shttp.Server, authBackend shttp.AuthenticationBackend) {
	// swagger:operation GET /topology getTopology
	//
	// Get topology
	//
	// ---
	// summary: Get topology
	//
	// tags:
	// - topology
	//
	// consumes:
	// - application/json
	//
	// produces:
	// - application/json
	// - text/vnd.graphviz
	//
	// schemes:
	// - http
	// - https
	//
	// responses:
	//   200:
	//     description: topology
	//     schema:
	//       type: object

	// swagger:operation POST /topology searchTopology
	//
	// Search topology
	//
	// ---
	// summary: Search topology
	//
	// tags:
	// - topology
	//
	// consumes:
	// - application/json
	//
	// produces:
	// - application/json
	// - text/vnd.graphviz
	// - application/vnd.tcpdump.pcap
	//
	// schemes:
	// - http
	// - https
	//
	// parameters:
	//   - in: body
	//     name: params
	//     required: true
	//     schema:
	//       $ref: '#/definitions/TopologyParams'
	//
	// responses:
	//   200:
	//     description: query result
	//     schema:
	//       $ref: '#/definitions/AnyValue'
	//   204:
	//     description: empty query

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

	r.RegisterRoutes(routes, authBackend)
}

// RegisterTopologyAPI registers a new topology query API
func RegisterTopologyAPI(r *shttp.Server, g *graph.Graph, parser *traversal.GremlinTraversalParser, authBackend shttp.AuthenticationBackend) {
	t := &TopologyAPI{
		gremlinParser: parser,
		graph:         g,
	}

	t.registerEndpoints(r, authBackend)
}
