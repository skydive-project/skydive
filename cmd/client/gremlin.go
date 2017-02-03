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

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/topology/graph"
)

var NotFound = errors.New("No result found")

type GremlinQueryHelper struct {
	authOptions *shttp.AuthenticationOpts
}

func (g *GremlinQueryHelper) Query(query string, values interface{}) error {
	client, err := api.NewRestClientFromConfig(g.authOptions)
	if err != nil {
		return err
	}

	gq := api.Topology{GremlinQuery: query}
	s, err := json.Marshal(gq)
	if err != nil {
		return err
	}

	contentReader := bytes.NewReader(s)

	resp, err := client.Request("POST", "api/topology", contentReader)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, string(data))
	}

	if err = common.JsonDecode(resp.Body, values); err != nil {
		return err
	}

	return nil
}

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

func (g *GremlinQueryHelper) GetNode(query string) (node *graph.Node, _ error) {
	nodes, err := g.GetNodes(query)
	if err != nil {
		return nil, err
	}

	if len(nodes) > 0 {
		return nodes[0], nil
	}

	return nil, NotFound
}

func (g *GremlinQueryHelper) GetFlows(query string) (flows []*flow.Flow, err error) {
	err = g.Query(query, &flows)
	return
}

func (g *GremlinQueryHelper) GetFlowMetric(query string) (m *flow.FlowMetric, _ error) {
	flows, err := g.GetFlows(query)
	if err != nil {
		return nil, err
	}

	if len(flows) == 0 {
		return nil, NotFound
	}

	return flows[0].Metric, nil
}

func (g *GremlinQueryHelper) GetMetrics(query string) (m []*flow.FlowMetric, _ error) {
	if err := g.Query(query, &m); err != nil {
		return nil, err
	}

	return m, nil
}

func (g *GremlinQueryHelper) GetMetric(query string) (m *flow.FlowMetric, _ error) {
	if err := g.Query(query, &m); err != nil {
		return nil, err
	}

	if m == nil {
		return nil, NotFound
	}

	return m, nil
}

func (g *GremlinQueryHelper) GetInterfaceAggregatedMetrics(query string) (metrics []*graph.InterfaceMetric, _ error) {
	var obj []map[string]interface{}
	if err := g.Query(query, &obj); err != nil {
		return nil, err
	}

	if len(obj) == 0 {
		return nil, errors.New("No metrics found")
	}

	if aggregated, ok := obj[0]["Aggregated"]; ok && aggregated != nil {
		for _, i := range aggregated.([]interface{}) {
			obj := i.(map[string]interface{})
			RxPackets, _ := obj["RxPackets"].(json.Number).Int64()
			TxPackets, _ := obj["TxPackets"].(json.Number).Int64()
			RxBytes, _ := obj["RxBytes"].(json.Number).Int64()
			TxBytes, _ := obj["TxBytes"].(json.Number).Int64()

			metric := &graph.InterfaceMetric{
				RxPackets: RxPackets,
				TxPackets: TxPackets,
				RxBytes:   RxBytes,
				TxBytes:   TxBytes,
			}

			metrics = append(metrics, metric)
		}

		return metrics, nil
	}

	return nil, errors.New("No metrics found")
}

func NewGremlinQueryHelper(authOptions *shttp.AuthenticationOpts) *GremlinQueryHelper {
	return &GremlinQueryHelper{
		authOptions: authOptions,
	}
}
