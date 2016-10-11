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

package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

type RestClient struct {
	authClient *AuthenticationClient
	client     *http.Client
}

type CrudClient struct {
	RestClient
	Root string
}

func readBody(resp *http.Response) string {
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(data)
}

func NewRestClient(addr string, port int, authOptions *AuthenticationOpts) *RestClient {
	client := &http.Client{}
	authClient := NewAuthenticationClient(addr, port, authOptions)
	return &RestClient{
		client:     client,
		authClient: authClient,
	}
}

func NewRestClientFromConfig(authOptions *AuthenticationOpts) (*RestClient, error) {
	addr, port, err := config.GetAnalyzerClientAddr()
	if err != nil {
		logging.GetLogger().Errorf("Unable to parse analyzer client %s", err.Error())
		return nil, err
	}

	return NewRestClient(addr, port, authOptions), nil
}

func (c *RestClient) Request(method, path string, body io.Reader) (*http.Response, error) {
	if !c.authClient.Authenticated() {
		if err := c.authClient.Authenticate(); err != nil {
			return nil, err
		}
	}

	url := fmt.Sprintf("%s/%s", c.authClient.getPrefix(), path)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	cookie := http.Cookie{Name: "authtok", Value: c.authClient.AuthToken}
	req.Header.Set("Cookie", cookie.String())
	req.Header.Set("Content-Type", "application/json")

	return c.client.Do(req)
}

func NewCrudClient(addr string, port int, authOpts *AuthenticationOpts, root string) *CrudClient {
	restClient := NewRestClient(addr, port, authOpts)
	return &CrudClient{
		RestClient: *restClient,
		Root:       root,
	}
}

func NewCrudClientFromConfig(authOpts *AuthenticationOpts, root string) (*CrudClient, error) {
	restClient, err := NewRestClientFromConfig(authOpts)
	if err != nil {
		return nil, err
	}

	return &CrudClient{
		RestClient: *restClient,
		Root:       root,
	}, nil
}

func (c *CrudClient) List(resource string, values interface{}) error {
	path := fmt.Sprintf("%s/%s", c.Root, resource)
	resp, err := c.Request("GET", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Failed to list %s, %s: %s", resource, resp.Status, readBody(resp)))
	}

	return json.NewDecoder(resp.Body).Decode(values)
}

func (c *CrudClient) Get(resource string, id string, value interface{}) error {
	path := fmt.Sprintf("%s/%s/%s", c.Root, resource, id)
	resp, err := c.Request("GET", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Failed to get %s, %s: %s", resource, resp.Status, readBody(resp)))
	}

	return json.NewDecoder(resp.Body).Decode(value)
}

func (c *CrudClient) Create(resource string, value interface{}) error {
	s, err := json.Marshal(value)
	if err != nil {
		return err
	}

	contentReader := bytes.NewReader(s)

	path := fmt.Sprintf("%s/%s", c.Root, resource)

	resp, err := c.Request("POST", path, contentReader)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Failed to create %s, %s: %s", resource, resp.Status, readBody(resp)))
	}

	return json.NewDecoder(resp.Body).Decode(value)
}

func (c *CrudClient) Update(resource string, id string, value interface{}) error {
	s, err := json.Marshal(value)
	if err != nil {
		return err
	}

	contentReader := bytes.NewReader(s)
	path := fmt.Sprintf("%s/%s/%s", c.Root, resource, id)

	resp, err := c.Request("PUT", path, contentReader)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Failed to update %s, %s: %s", resource, resp.Status, readBody(resp)))
	}

	return json.NewDecoder(resp.Body).Decode(value)
}

func (c *CrudClient) Delete(resource string, id string) error {
	path := fmt.Sprintf("%s/%s/%s", c.Root, resource, id)

	resp, err := c.Request("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Failed to delete %s, %s: %s", resource, resp.Status, readBody(resp)))
	}

	return nil
}
