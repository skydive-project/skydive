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

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/gremlin"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/sflow"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes/socketinfo"
)

// GremlinQueryHelper describes a gremlin query request query helper mechanism
type GremlinQueryHelper struct {
	authOptions *shttp.AuthenticationOpts
}

// Request send a Gremlin request to the topology API
func (g *GremlinQueryHelper) Request(query interface{}, header http.Header) (*http.Response, error) {
	client, err := NewRestClientFromConfig(g.authOptions)
	if err != nil {
		return nil, err
	}

	gq := types.TopologyParam{GremlinQuery: gremlin.NewQueryStringFromArgument(query).String()}
	s, err := json.Marshal(gq)
	if err != nil {
		return nil, err
	}

	contentReader := bytes.NewReader(s)

	return client.Request("POST", "topology", contentReader, header)
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

	return nil, common.ErrNotFound
}

// GetFlows from the Gremlin query
func (g *GremlinQueryHelper) GetFlows(query interface{}) ([]*flow.Flow, error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	var flows []*flow.Flow
	if err := json.Unmarshal(data, &flows); err != nil {
		return nil, err
	}

	return flows, nil
}

// GetInterfaceMetrics from Gremlin query
func (g *GremlinQueryHelper) GetInterfaceMetrics(query interface{}) (map[string][]*topology.InterfaceMetric, error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	var result []map[string][]*topology.InterfaceMetric
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	return result[0], nil
}

// GetSFlowMetrics from Gremlin query
func (g *GremlinQueryHelper) GetSFlowMetrics(query interface{}) (map[string][]*sflow.SFMetric, error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	var result []map[string][]*sflow.SFMetric
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	return result[0], nil
}

// GetFlowMetrics from Gremlin query
func (g *GremlinQueryHelper) GetFlowMetrics(query interface{}) (map[string][]*flow.FlowMetric, error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	var result []map[string][]*flow.FlowMetric
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, common.ErrNotFound
	}

	return result[0], nil
}

// GetFlowMetric from Gremlin query
func (g *GremlinQueryHelper) GetFlowMetric(query interface{}) (*flow.FlowMetric, error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	var result flow.FlowMetric
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetInterfaceMetric from Gremlin query
func (g *GremlinQueryHelper) GetInterfaceMetric(query interface{}) (*topology.InterfaceMetric, error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	var result topology.InterfaceMetric
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetSFlowMetric from Gremlin query
func (g *GremlinQueryHelper) GetSFlowMetric(query interface{}) (*sflow.SFMetric, error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	var result sflow.SFMetric
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetSockets from the Gremlin query
func (g *GremlinQueryHelper) GetSockets(query interface{}) (sockets map[string][]*socketinfo.ConnectionInfo, err error) {
	data, err := g.Query(query)
	if err != nil {
		return nil, err
	}

	// TODO: use real objects instead of interface + decode
	// should be []map[string][]ConnectionInfo
	var maps []map[string][]interface{}
	if err := common.JSONDecode(bytes.NewReader(data), &maps); err != nil {
		return nil, err
	}

	sockets = make(map[string][]*socketinfo.ConnectionInfo)
	for id, objs := range maps[0] {
		sockets[id] = make([]*socketinfo.ConnectionInfo, 0)
		for _, obj := range objs {
			var socket socketinfo.ConnectionInfo
			if err = socket.Decode(obj); err == nil {
				sockets[id] = append(sockets[id], &socket)
			}
		}
	}

	return
}

// NewGremlinQueryHelper creates a new Gremlin query helper based on authentication
func NewGremlinQueryHelper(authOptions *shttp.AuthenticationOpts) *GremlinQueryHelper {
	return &GremlinQueryHelper{
		authOptions: authOptions,
	}
}
