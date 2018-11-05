/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package config

import (
	"net/http"
	"net/url"
	"time"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/websocket"
)

// NewWSClient creates a Client based on the configuration
func NewWSClient(clientType common.ServiceType, url *url.URL, authOpts *shttp.AuthenticationOpts, headers http.Header) (*websocket.Client, error) {
	host := GetString("host_id")
	queueSize := GetInt("http.ws.queue_size")
	writeCompression := GetBool("http.ws.enable_write_compression")
	tlsConfig, err := GetTLSClientConfig(true)
	if err != nil {
		return nil, err
	}

	return websocket.NewClient(host, clientType, url, authOpts, headers, queueSize, writeCompression, tlsConfig), nil
}

// NewWSServer creates a Server based on the configuration
func NewWSServer(server *shttp.Server, endpoint string, authBackend shttp.AuthenticationBackend) *websocket.Server {
	queueSize := GetInt("http.ws.queue_size")
	writeCompression := GetBool("http.ws.enable_write_compression")
	pingDelay := time.Duration(GetInt("http.ws.ping_delay")) * time.Second
	pongTimeout := time.Duration(GetInt("http.ws.pong_timeout"))*time.Second + pingDelay

	return websocket.NewServer(server, endpoint, authBackend, writeCompression, queueSize, pingDelay, pongTimeout)
}
