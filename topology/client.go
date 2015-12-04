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
)

type Client struct {
	Addr            string
	Port            int
	Endpoint        string
	asyncUpdateChan chan string
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

// TODO(safchain) implement a real async update
func (c *Client) AsyncUpdate(topo *Topology) error {
	return c.Update(topo)
}

func NewClient(addr string, port int) *Client {
	return &Client{
		Addr:     addr,
		Port:     port,
		Endpoint: "/rpc/topology",
	}
}
