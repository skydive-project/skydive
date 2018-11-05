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
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/skydive-project/skydive/common"
)

// RestClient describes a REST API client with a URL and authentication information
type RestClient struct {
	authOpts *AuthenticationOpts
	client   *http.Client
	url      *url.URL
}

// CrudClient describes a REST API client to issue CRUD commands
type CrudClient struct {
	*RestClient
}

func readBody(resp *http.Response) string {
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(data)
}

func getHTTPClient(tlsConfig *tls.Config) *http.Client {
	client := &http.Client{}
	if tlsConfig != nil {
		tr := &http.Transport{TLSClientConfig: tlsConfig}
		client = &http.Client{Transport: tr}
	}
	return client
}

// NewRestClient returns a new REST API client. It takes a URL
// to the HTTP point, authentication information and TLS configuration
func NewRestClient(url *url.URL, authOpts *AuthenticationOpts, tlsConfig *tls.Config) *RestClient {
	return &RestClient{
		client:   getHTTPClient(tlsConfig),
		url:      url,
		authOpts: authOpts,
	}
}

// Request issues a request to the API
func (c *RestClient) Request(method, path string, body io.Reader, header http.Header) (*http.Response, error) {
	url := c.url.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		return nil, err
	}

	if c.authOpts != nil {
		SetAuthHeaders(&req.Header, c.authOpts)
	}

	if header != nil {
		req.Header = header
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Accept-Encoding", "gzip")

	resp, err := c.client.Do(req)
	if err != nil {
		return resp, err
	}

	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		resp.Body, err = gzip.NewReader(resp.Body)
		resp.Uncompressed = true
		resp.ContentLength = -1
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// NewCrudClient returns a new REST client that is able to issue CRUD requests
func NewCrudClient(url *url.URL, authOpts *AuthenticationOpts, tlsConfig *tls.Config) *CrudClient {
	return &CrudClient{
		RestClient: NewRestClient(url, authOpts, tlsConfig),
	}
}

// List returns all the resources for a type
func (c *CrudClient) List(resource string, values interface{}) error {
	resp, err := c.Request("GET", resource, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to list %s, %s: %s", resource, resp.Status, readBody(resp))
	}

	return common.JSONDecode(resp.Body, values)
}

// Get fills the passed value with the resource with the specified ID
func (c *CrudClient) Get(resource string, id string, value interface{}) error {
	resp, err := c.Request("GET", resource+"/"+id, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to get %s, %s: %s", resource, resp.Status, readBody(resp))
	}

	return common.JSONDecode(resp.Body, value)
}

// Create does a POST request to create a new resource
func (c *CrudClient) Create(resource string, value interface{}) error {
	s, err := json.Marshal(value)
	if err != nil {
		return err
	}

	contentReader := bytes.NewReader(s)
	resp, err := c.Request("POST", resource, contentReader, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to create %s, %s: %s", resource, resp.Status, readBody(resp))
	}

	return common.JSONDecode(resp.Body, value)
}

// Update modify a resource using a PUT call to the API
func (c *CrudClient) Update(resource string, id string, value interface{}) error {
	s, err := json.Marshal(value)
	if err != nil {
		return err
	}

	contentReader := bytes.NewReader(s)
	resp, err := c.Request("PUT", resource+"/"+id, contentReader, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to update %s, %s: %s", resource, resp.Status, readBody(resp))
	}

	return common.JSONDecode(resp.Body, value)
}

// Delete removes a resource using a DELETE call to the API
func (c *CrudClient) Delete(resource string, id string) error {
	resp, err := c.Request("DELETE", resource+"/"+id, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to delete %s, %s: %s", resource, resp.Status, readBody(resp))
	}

	return nil
}
