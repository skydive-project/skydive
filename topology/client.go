/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package topology

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/redhat-cip/skydive/logging"
)

type Client struct {
	Addr     string
	Port     int
	Endpoint string
}

type AsyncClient struct {
	Client
	asyncUpdaterChan chan AsyncUpdateRequest
}

type AsyncUpdateRequest struct {
	Host     string
	JsonData string
}

// TODO(safchain) handle status code
func (c *Client) Update(topo *Topology) error {
	url := "http://" + c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10) + c.Endpoint + "/" + topo.Host

	jsonData := []byte(topo.String())
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *AsyncClient) asyncUpdater() {
	var url string
	var request AsyncUpdateRequest

	for {
		request = <-c.asyncUpdaterChan

		url = "http://" + c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10) + c.Endpoint + "/" + request.Host

		jsonData := []byte(request.JsonData)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			logging.GetLogger().Error("Unable to post topology: %s", err.Error())
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logging.GetLogger().Error("Unable to post topology: %s", err.Error())
		}
		resp.Body.Close()
	}
}

// TODO(safchain) implement a real async update
func (c *AsyncClient) AsyncUpdate(host string, data string) {

	request := AsyncUpdateRequest{
		Host:     host,
		JsonData: data,
	}

	c.asyncUpdaterChan <- request
}

func NewClient(addr string, port int) *Client {
	return &Client{
		Addr:     addr,
		Port:     port,
		Endpoint: "/rpc/topology",
	}
}

func (c *AsyncClient) Start() {
	go c.asyncUpdater()
}

func NewAsyncClient(addr string, port int) *AsyncClient {
	c := &AsyncClient{
		Client: Client{
			Addr:     addr,
			Port:     port,
			Endpoint: "/rpc/topology",
		},
		asyncUpdaterChan: make(chan AsyncUpdateRequest),
	}

	return c
}
