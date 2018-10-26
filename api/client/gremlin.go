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

	"github.com/mitchellh/mapstructure"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/gremlin"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
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

// QueryRaw queries the topology API and returns the raw result
func (g *GremlinQueryHelper) QueryRaw(query interface{}) ([]byte, error) {
	resp, err := g.Request(query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading response: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s (for query %s)", resp.Status, string(data), query)
	}

	return data, nil
}

// QueryObject queries the topology API and deserialize into value
func (g *GremlinQueryHelper) QueryObject(query interface{}, value interface{}) error {
	resp, err := g.Request(query, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, string(data))
	}

	return common.JSONDecode(resp.Body, value)
}

// GetNodes from the Gremlin query
func (g *GremlinQueryHelper) GetNodes(query interface{}) ([]*graph.Node, error) {
	var values []interface{}
	if err := g.QueryObject(query, &values); err != nil {
		return nil, err
	}

	var nodes []*graph.Node
	for _, obj := range values {
		switch t := obj.(type) {
		case []interface{}:
			/*for _, node := range t {
				n := new(graph.Node)
				if err := n.Decode(node); err != nil {
					return nil, err
				}
				nodes = append(nodes, ni)
			}*/
			_ = t
		case interface{}:
			n := new(graph.Node)
			/*if err := n.Decode(t); err != nil {
				return nil, err
			}*/
			nodes = append(nodes, n)
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
func (g *GremlinQueryHelper) GetFlows(query interface{}) (flows []*flow.Flow, err error) {
	err = g.QueryObject(query, &flows)
	return
}

// GetFlowMetric from Gremlin query
func (g *GremlinQueryHelper) GetFlowMetric(query interface{}) (m *flow.FlowMetric, _ error) {
	flows, err := g.GetFlows(query)
	if err != nil {
		return nil, err
	}

	if len(flows) == 0 {
		return nil, common.ErrNotFound
	}

	return flows[0].Metric, nil
}

func flatMetricToTypedMetric(flat map[string]interface{}) (common.Metric, error) {
	var metric common.Metric

	// check whether interface metrics or flow metrics
	if _, ok := flat["ABBytes"]; ok {
		metric = &flow.FlowMetric{}
		if err := mapstructure.WeakDecode(flat, metric); err != nil {
			return nil, err
		}
	} else {
		metric = &topology.InterfaceMetric{}
		if err := mapstructure.WeakDecode(flat, metric); err != nil {
			return nil, err
		}
	}

	return metric, nil
}

// GetMetrics from Gremlin query
func (g *GremlinQueryHelper) GetMetrics(query interface{}) (map[string][]common.Metric, error) {
	flat := []map[string][]map[string]interface{}{}

	if err := g.QueryObject(query, &flat); err != nil {
		return nil, fmt.Errorf("QueryObject error: %s", err)
	}

	result := make(map[string][]common.Metric)

	if len(flat) == 0 {
		return result, nil
	}

	for id, metrics := range flat[0] {
		result[id] = make([]common.Metric, len(metrics))
		for i, metric := range metrics {
			tm, err := flatMetricToTypedMetric(metric)
			if err != nil {
				return nil, fmt.Errorf("Flat to typed metric error: %s", err)
			}
			result[id][i] = tm
		}
	}

	return result, nil
}

// GetMetric from Gremlin query
func (g *GremlinQueryHelper) GetMetric(query interface{}) (common.Metric, error) {
	flat := map[string]interface{}{}

	if err := g.QueryObject(query, &flat); err != nil {
		return nil, err
	}

	if len(flat) == 0 {
		return nil, fmt.Errorf("Failed to get metric for %s", query)
	}

	return flatMetricToTypedMetric(flat)
}

// GetSockets from the Gremlin query
func (g *GremlinQueryHelper) GetSockets(query interface{}) (sockets map[string][]*socketinfo.ConnectionInfo, err error) {
	var maps []map[string][]interface{}
	if err = g.QueryObject(query, &maps); err != nil || len(maps) == 0 {
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
