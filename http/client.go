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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
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

func getHttpClient() *http.Client {
	client := &http.Client{}
	if config.IsTLSenabled() == true {
		certPEM := config.GetConfig().GetString("agent.X509_cert")
		keyPEM := config.GetConfig().GetString("agent.X509_key")
		analyzerCertPEM := config.GetConfig().GetString("analyzer.X509_cert")
		tlsConfig := common.SetupTLSClientConfig(certPEM, keyPEM)
		tlsConfig.RootCAs = common.SetupTLSLoadCertificate(analyzerCertPEM)
		checkTLSConfig(tlsConfig)

		tr := &http.Transport{TLSClientConfig: tlsConfig}
		client = &http.Client{Transport: tr}
	}
	return client
}

func NewRestClient(addr string, port int, authOptions *AuthenticationOpts) *RestClient {
	client := getHttpClient()
	authClient := NewAuthenticationClient(addr, port, authOptions)
	return &RestClient{
		client:     client,
		authClient: authClient,
	}
}

func (c *RestClient) Request(method, path string, body io.Reader, header http.Header) (*http.Response, error) {
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

	if header != nil {
		req.Header = header
	}
	req.Header.Set("Content-Type", "application/json")

	cookie := http.Cookie{Name: "authtok", Value: c.authClient.AuthToken}
	req.Header.Set("Cookie", cookie.String())

	return c.client.Do(req)
}

func NewCrudClient(addr string, port int, authOpts *AuthenticationOpts, root string) *CrudClient {
	restClient := NewRestClient(addr, port, authOpts)
	return &CrudClient{
		RestClient: *restClient,
		Root:       root,
	}
}

func (c *CrudClient) List(resource string, values interface{}) error {
	path := fmt.Sprintf("%s/%s", c.Root, resource)
	resp, err := c.Request("GET", path, nil, nil)
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
	path := fmt.Sprintf("%s/%s/%s", c.Root, resource, id)
	resp, err := c.Request("GET", path, nil, nil)
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

	path := fmt.Sprintf("%s/%s", c.Root, resource)

	resp, err := c.Request("POST", path, contentReader, nil)
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
	path := fmt.Sprintf("%s/%s/%s", c.Root, resource, id)

	resp, err := c.Request("PUT", path, contentReader, nil)
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
	path := fmt.Sprintf("%s/%s/%s", c.Root, resource, id)

	resp, err := c.Request("DELETE", path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to delete %s, %s: %s", resource, resp.Status, readBody(resp))
	}

	return nil
}
