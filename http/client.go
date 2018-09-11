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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

type RestClient struct {
	authOpts *AuthenticationOpts
	client   *http.Client
	url      *url.URL
}

type CrudClient struct {
	RestClient
}

func readBody(resp *http.Response) string {
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(data)
}

func getHttpClient() (*http.Client, error) {
	client := &http.Client{}
	if config.IsTLSenabled() {
		tlsConfig, err := GetTLSConfig(true)
		if err != nil {
			return nil, err
		}
		tr := &http.Transport{TLSClientConfig: tlsConfig}
		client = &http.Client{Transport: tr}
	}
	return client, nil
}

func NewRestClient(url *url.URL, authOpts *AuthenticationOpts) (*RestClient, error) {
	client, err := getHttpClient()
	if err != nil {
		return nil, err
	}
	rc := &RestClient{
		client:   client,
		url:      url,
		authOpts: authOpts,
	}
	return rc, nil
}

func (c *RestClient) debug() bool {
	return config.GetBool("http.rest.debug")
}

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

	if c.debug() {
		if buf, err := httputil.DumpRequest(req, true); err == nil {
			logging.GetLogger().Debugf("Request:\n%s", buf)
		}
	}

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

	if c.debug() {
		buf, err := httputil.DumpResponse(resp, true)
		if err == nil {
			logging.GetLogger().Debugf("Response:\n%s", buf)
		} else {
			logging.GetLogger().Debugf("Response (error):\n%s", err)
		}

	}

	return resp, nil
}

func NewCrudClient(url *url.URL, authOpts *AuthenticationOpts) (*CrudClient, error) {
	restClient, err := NewRestClient(url, authOpts)
	if err != nil {
		return nil, err
	}
	return &CrudClient{
		RestClient: *restClient,
	}, nil
}

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
