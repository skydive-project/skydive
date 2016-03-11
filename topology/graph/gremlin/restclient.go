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

package gremlin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/redhat-cip/skydive/logging"
)

type gremlinServerResponse struct {
	RequestID string `json:"requestId"`
	Status    struct {
		Message    string      `json:"message"`
		Code       int         `json:"code"`
		Attributes interface{} `json:"attributes"`
	} `json:"status"`
	Result json.RawMessage `json:"result"`
}

type gremlinServerData struct {
	Data results `json:"data"`
}

// provides support for
//   https://github.com/thinkaurelius/neo4j-gremlin-plugin
type neo4jGremlinPluginResponse struct {
	Success bool            `json:"success"`
	Results json.RawMessage `json:"results"`
}

type results []map[string]interface{}

type responseParser interface {
	IsSuccess(b []byte) (bool, error)
	Results(b []byte) (results, error)
}

type restclient struct {
	Endpoint       string
	responseParser responseParser
}

type gremlinServerParser struct {
}

type neo4jGremlinParser struct {
}

func (g *gremlinServerParser) IsSuccess(b []byte) (bool, error) {
	resp := gremlinServerResponse{}
	err := json.Unmarshal(b, &resp)
	if err != nil {
		return false, err
	}

	if resp.Status.Code != 200 {
		return false, fmt.Errorf("Gremlin server response error: %d, %s", resp.Status.Message)
	}

	return true, nil
}

func (g *gremlinServerParser) Results(b []byte) (results, error) {
	resp := gremlinServerResponse{}
	err := json.Unmarshal(b, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Status.Code != 200 {
		return nil, fmt.Errorf("Gremlin server response error: %d, %s", resp.Status.Message)
	}

	data := gremlinServerData{}
	err = json.Unmarshal(resp.Result, &data)
	if err != nil {
		return nil, err
	}

	return data.Data, nil
}

func (n *neo4jGremlinParser) IsSuccess(b []byte) (bool, error) {
	resp := neo4jGremlinPluginResponse{}
	err := json.Unmarshal(b, &resp)
	if err != nil {
		return false, err
	}

	if !resp.Success {
		return false, errors.New("Neo4J gremlin plugin error")
	}

	return true, nil
}

func (n *neo4jGremlinParser) Results(b []byte) (results, error) {
	resp := neo4jGremlinPluginResponse{}
	err := json.Unmarshal(b, &resp)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, errors.New("Neo4J gremlin plugin error")
	}

	res := results{}
	err = json.Unmarshal(resp.Results, &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *restclient) parseID(i interface{}) string {
	id, ok := i.(string)
	if !ok {
		id = strconv.FormatFloat(i.(float64), 'f', 0, 64)
	}
	return id
}

func (c *restclient) queryElements(q string) ([]GremlinElement, error) {
	resp, err := c.query(q)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin query error: %s, %s", q, err.Error())
		return nil, err
	}

	res, err := c.responseParser.Results(resp)
	if err != nil {
		return nil, err
	}

	els := make([]GremlinElement, len(res))

	i := 0
	for _, m := range res {
		el := GremlinElement{
			ID:         GremlinID(c.parseID(m["id"])),
			Type:       m["type"].(string),
			Properties: make(GremlinProperties),
		}

		for k, v := range m["properties"].(map[string]interface{}) {
			a, ok := v.([]interface{})
			if !ok {
				// format {k: v}
				prop := GremlinProperty{
					Value: v.(string),
				}
				el.Properties[k] = []GremlinProperty{prop}
			} else {
				// format [{id:k, value:v}]
				el.Properties[k] = make([]GremlinProperty, len(a))
				for j, e := range a {
					m := e.(map[string]interface{})
					prop := GremlinProperty{
						ID:    GremlinID(c.parseID(m["id"])),
						Value: m["value"],
					}
					el.Properties[k][j] = prop
				}
			}
		}
		els[i] = el
		i++
	}

	return els, nil
}

func (c *restclient) query(q string) ([]byte, error) {
	q = c.Endpoint + url.QueryEscape(q)

	req, err := http.NewRequest("GET", q, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	tr := &http.Transport{
		DisableKeepAlives:  true,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gremlin request error %s, %d, %s", q, resp.StatusCode, resp.Status)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	return body, nil
}

func (c *restclient) detectServer() error {
	body, err := c.query(`"Hello"`)
	if err != nil {
		return err
	}

	g := &gremlinServerParser{}
	if _, err = g.IsSuccess(body); err == nil {
		c.responseParser = g
		return nil
	}

	n := &neo4jGremlinParser{}
	if _, err = n.IsSuccess(body); err == nil {
		c.responseParser = n
		return nil
	}

	return errors.New("Unable to detect rest gremlin server")
}

func (c *restclient) connect(endpoint string) error {
	c.Endpoint = endpoint

	if err := c.detectServer(); err != nil {
		return err
	}

	return nil
}

func (c *restclient) close() {
}
