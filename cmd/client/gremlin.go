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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/mitchellh/mapstructure"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/topology/graph"
)

// ErrNotFound error no result found
var ErrNotFound = errors.New("No result found")

// GremlinQueryHelper describes a gremlin query request query helper mechanism
type GremlinQueryHelper struct {
	authOptions *shttp.AuthenticationOpts
}

// Request send a Gremlin request to the topology API
func (g *GremlinQueryHelper) Request(query string, header http.Header) (*http.Response, error) {
	client, err := api.NewRestClientFromConfig(g.authOptions)
	if err != nil {
		return nil, err
	}

	gq := api.TopologyParam{GremlinQuery: query}
	s, err := json.Marshal(gq)
	if err != nil {
		return nil, err
	}

	contentReader := bytes.NewReader(s)

	return client.Request("POST", "api/topology", contentReader, header)
}

// Query the topology API
func (g *GremlinQueryHelper) Query(query string, values interface{}) error {
	resp, err := g.Request(query, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, string(data))
	}

	if err = common.JSONDecode(resp.Body, values); err != nil {
		return err
	}

	return nil
}

// GetNodes from the Gremlin query
func (g *GremlinQueryHelper) GetNodes(query string) ([]*graph.Node, error) {
	var values []interface{}
	if err := g.Query(query, &values); err != nil {
		return nil, err
	}

	var nodes []*graph.Node
	for _, obj := range values {
		switch t := obj.(type) {
		case []interface{}:
			for _, node := range t {
				n := new(graph.Node)
				if err := n.Decode(node); err != nil {
					return nil, err
				}
				nodes = append(nodes, n)
			}
		case interface{}:
			n := new(graph.Node)
			if err := n.Decode(t); err != nil {
				return nil, err
			}
			nodes = append(nodes, n)
		}
	}

	return nodes, nil
}

// GetNode from the Gremlin query
func (g *GremlinQueryHelper) GetNode(query string) (node *graph.Node, _ error) {
	nodes, err := g.GetNodes(query)
	if err != nil {
		return nil, err
	}

	if len(nodes) > 0 {
		return nodes[0], nil
	}

	return nil, ErrNotFound
}

// GetFlows form the Gremlin query
func (g *GremlinQueryHelper) GetFlows(query string) (flows []*flow.Flow, err error) {
	err = g.Query(query, &flows)
	return
}

// GetFlowMetric from Gremlin query
func (g *GremlinQueryHelper) GetFlowMetric(query string) (m *flow.FlowMetric, _ error) {
	flows, err := g.GetFlows(query)
	if err != nil {
		return nil, err
	}

	if len(flows) == 0 {
		return nil, ErrNotFound
	}

	return flows[0].Metric, nil
}

func flatMetrictoTimedMetric(flat map[string]interface{}) (*common.TimedMetric, error) {
	start, _ := flat["Start"].(json.Number).Int64()
	last, _ := flat["Last"].(json.Number).Int64()

	tm := &common.TimedMetric{
		TimeSlice: common.TimeSlice{
			Start: start,
			Last:  last,
		},
	}

	// check whether interface metrics or flow metrics
	if _, ok := flat["ABBytes"]; ok {
		metric := flow.FlowMetric{}
		if err := mapstructure.WeakDecode(flat, &metric); err != nil {
			return nil, err
		}
		tm.Metric = &metric
	} else {
		metric := graph.InterfaceMetric{}
		if err := mapstructure.WeakDecode(flat, &metric); err != nil {
			return nil, err
		}
		tm.Metric = &metric
	}

	return tm, nil
}

// GetMetrics from Gremlin query
func (g *GremlinQueryHelper) GetMetrics(query string) (map[string][]*common.TimedMetric, error) {
	flat := []map[string][]map[string]interface{}{}

	if err := g.Query(query, &flat); err != nil {
		return nil, err
	}

	result := make(map[string][]*common.TimedMetric)

	if len(flat) == 0 {
		return result, nil
	}

	for id, metrics := range flat[0] {
		result[id] = make([]*common.TimedMetric, len(metrics))
		for i, metric := range metrics {
			tm, err := flatMetrictoTimedMetric(metric)
			if err != nil {
				return nil, err
			}
			result[id][i] = tm
		}
	}

	return result, nil
}

// GetMetric from Gremlin query
func (g *GremlinQueryHelper) GetMetric(query string) (*common.TimedMetric, error) {
	flat := map[string]interface{}{}

	if err := g.Query(query, &flat); err != nil {
		return nil, err
	}

	return flatMetrictoTimedMetric(flat)
}

// NewGremlinQueryHelper creates a new Gremlin query helper based on authentication
func NewGremlinQueryHelper(authOptions *shttp.AuthenticationOpts) *GremlinQueryHelper {
	return &GremlinQueryHelper{
		authOptions: authOptions,
	}
}
