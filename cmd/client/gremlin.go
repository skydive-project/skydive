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

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/flow"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/topology/graph"
)

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

	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()

	if err = decoder.Decode(values); err != nil {
		return err
	}

	return nil
}

func (g *GremlinQueryHelper) GetNodes(query string) ([]*graph.Node, error) {
	var values []interface{}
	if err := g.Query(query, &values); err != nil {
		return nil, err
	}

	nodes := make([]*graph.Node, len(values))
	for i, node := range values {
		n := new(graph.Node)
		if err := n.Decode(node); err != nil {
			return nil, err
		}
		nodes[i] = n
	}

	return nodes, nil
}

func (g *GremlinQueryHelper) GetNode(query string) (node *graph.Node, _ error) {
	nodes, err := g.GetNodes(query)
	if err != nil {
		return nil, err
	}

	if len(nodes) > 0 {
		node = nodes[0]
	}

	return
}

func (g *GremlinQueryHelper) GetFlows(query string) (flows []*flow.Flow, err error) {
	err = g.Query(query, &flows)
	return
}

func (g *GremlinQueryHelper) GetFlowMetric(query string) (metric flow.FlowMetric, err error) {
	err = g.Query(query, &metric)
	return
}

func NewGremlinQueryHelper(authOptions *shttp.AuthenticationOpts) *GremlinQueryHelper {
	return &GremlinQueryHelper{
		authOptions: authOptions,
	}
}
