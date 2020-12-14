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

package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/skydive-project/skydive/graffiti/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
)

// ErrNoResult is returned when a query returned no result
var ErrNoResult = errors.New("no result")

// GremlinQueryHelper describes a gremlin query request query helper mechanism
type GremlinQueryHelper struct {
	restClient  *shttp.RestClient
	authOptions *shttp.AuthenticationOpts
}

// Request send a Gremlin request to the topology API
func (g *GremlinQueryHelper) Request(query interface{}, header http.Header) (*http.Response, error) {
	var queryString string
	switch query := query.(type) {
	case fmt.Stringer:
		queryString = query.String()
	case string:
		queryString = query
	default:
		return nil, errors.New("query must be either a string or Stringer")
	}
	gq := types.TopologyParams{GremlinQuery: queryString}
	s, err := json.Marshal(gq)
	if err != nil {
		return nil, err
	}

	contentReader := bytes.NewReader(s)

	return g.restClient.Request("POST", "topology", contentReader, header)
}

// Query queries the topology API
func (g *GremlinQueryHelper) Query(query interface{}) ([]byte, error) {
	resp, err := g.Request(query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, string(data))
	}

	return data, nil
}

// GetInt64 parse the query result as int64
func (g *GremlinQueryHelper) GetInt64(query interface{}) (int64, error) {
	data, err := g.Query(query)
	if err != nil {
		return 0, err
	}

	var i int64
	if err := json.Unmarshal(data, &i); err != nil {
		return 0, err
	}

	return i, nil
}

// GetNodes from the Gremlin query
func (g *GremlinQueryHelper) GetNodes(query interface{}) ([]*graph.Node, error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	var result []json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	var nodes []*graph.Node
	for _, obj := range result {
		// hacky stuff to know how to decode
		switch obj[0] {
		case '[':
			var n []*graph.Node
			if err := json.Unmarshal(obj, &n); err != nil {
				return nil, err
			}
			nodes = append(nodes, n...)
		case '{':
			var n graph.Node
			if err := json.Unmarshal(obj, &n); err != nil {
				return nil, err
			}
			nodes = append(nodes, &n)
		}
	}

	return nodes, nil
}

// GetNode from the Gremlin query
func (g *GremlinQueryHelper) GetNode(query interface{}) (node *graph.Node, _ error) {
	nodes, err := g.GetNodes(query)
	if err != nil {
		return nil, err
	}

	if len(nodes) > 0 {
		return nodes[0], nil
	}

	return nil, ErrNoResult
}

// GetEdges from the Gremlin query
func (g *GremlinQueryHelper) GetEdges(query interface{}) ([]*graph.Edge, error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	var result []json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	var edges []*graph.Edge
	for _, obj := range result {
		// hacky stuff to know how to decode
		switch obj[0] {
		case '[':
			var e []*graph.Edge
			if err := json.Unmarshal(obj, &e); err != nil {
				return nil, err
			}
			edges = append(edges, e...)
		case '{':
			var e graph.Edge
			if err := json.Unmarshal(obj, &e); err != nil {
				return nil, err
			}
			edges = append(edges, &e)
		}
	}

	return edges, nil
}

// NewGremlinQueryHelper creates a new Gremlin query helper based on authentication
func NewGremlinQueryHelper(restClient *shttp.RestClient) *GremlinQueryHelper {
	return &GremlinQueryHelper{restClient: restClient}
}
