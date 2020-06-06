/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package config

import (
	"net/url"
	"time"

	shttp "github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/graffiti/websocket"
)

// NewWSClientOpts creates WebSocket options object from the configuration
func NewWSClientOpts(authOpts *shttp.AuthenticationOpts) (*websocket.ClientOpts, error) {
	// override some of the options with config value
	tlsConfig, err := GetTLSClientConfig(true)
	if err != nil {
		return nil, err
	}

	return &websocket.ClientOpts{
		QueueSize:        GetInt("http.ws.queue_size"),
		WriteCompression: GetBool("http.ws.enable_write_compression"),
		TLSConfig:        tlsConfig,
		AuthOpts:         authOpts,
	}, nil
}

// NewWSClient creates a Client based on the configuration
func NewWSClient(clientType service.Type, url *url.URL, opts websocket.ClientOpts) (*websocket.Client, error) {
	host := GetString("host_id")

	return websocket.NewClient(host, clientType, url, opts), nil
}

// NewWSServerOpts returns WebSocket server options
func NewWSServerOpts() websocket.ServerOpts {
	pingDelay := time.Duration(GetInt("http.ws.ping_delay")) * time.Second

	return websocket.ServerOpts{
		WriteCompression: GetBool("http.ws.enable_write_compression"),
		QueueSize:        GetInt("http.ws.queue_size"),
		PingDelay:        pingDelay,
		PongTimeout:      time.Duration(GetInt("http.ws.pong_timeout"))*time.Second + pingDelay,
	}
}

// NewWSServer creates a Server based on the configuration
func NewWSServer(server *shttp.Server, endpoint string, authBackend shttp.AuthenticationBackend) *websocket.Server {
	opts := NewWSServerOpts()
	opts.AuthBackend = authBackend
	return websocket.NewServer(server, endpoint, opts)
}
