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

package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
)

type RestClient struct {
	addr   string
	port   int
	client *http.Client
}

func NewClient(addr string, port int) *RestClient {
	client := &http.Client{}
	return &RestClient{client: client, addr: addr, port: port}
}

func NewClientFromConfig() *RestClient {
	addr, port, err := config.GetAnalyzerClientAddr()
	if err != nil {
		logging.GetLogger().Errorf("Unable to parse analyzer client %s", err.Error())
		os.Exit(1)
	}

	return NewClient(addr, port)
}

func (c *RestClient) getPrefix() string {
	return fmt.Sprintf("http://%s:%d/rpc", c.addr, c.port)
}

func (c *RestClient) Request(method, urlStr string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return c.client.Do(req)
}

func (c *RestClient) List(resource string, values interface{}) error {
	url := fmt.Sprintf("%s/%s", c.getPrefix(), resource)
	resp, err := c.Request("GET", url, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Failed to retrieve list of %s", resource))
	}

	return json.NewDecoder(resp.Body).Decode(values)
}

func (c *RestClient) Get(resource string, id string, value interface{}) error {
	url := fmt.Sprintf("%s/%s/%s", c.getPrefix(), resource, id)
	resp, err := c.Request("GET", url, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Failed to retrieve %s", resource))
	}

	return json.NewDecoder(resp.Body).Decode(value)
}

func (c *RestClient) Create(resource string, value interface{}) error {
	s, err := json.Marshal(value)
	if err != nil {
		return err
	}

	contentReader := bytes.NewReader(s)
	url := fmt.Sprintf("%s/%s", c.getPrefix(), resource)

	resp, err := c.Request("POST", url, contentReader)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Failed to create %s", resource))
	}

	return json.NewDecoder(resp.Body).Decode(value)
}

func (c *RestClient) Update(resource string, id string, value interface{}) error {
	s, err := json.Marshal(value)
	if err != nil {
		return err
	}

	contentReader := bytes.NewReader(s)
	url := fmt.Sprintf("%s/%s/%s", c.getPrefix(), resource, id)

	resp, err := c.Request("PUT", url, contentReader)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Failed to update %s", resource))
	}

	return json.NewDecoder(resp.Body).Decode(value)
}

func (c *RestClient) Delete(resource string, id string) error {
	url := fmt.Sprintf("%s/%s/%s", c.getPrefix(), resource, id)

	resp, err := c.Request("DELETE", url, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Failed to delete %s", resource))
	}

	return nil
}
