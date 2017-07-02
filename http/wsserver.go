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

package http

import (
	"net/http"
	"sync"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/websocket"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

type ClientHandler func(*websocket.Conn, *auth.AuthenticatedRequest) WSClient

type WSServer struct {
	sync.RWMutex
	*WSClientPool
	clientHandler ClientHandler
	HTTPServer    *Server
}

func DefaultClientHandler(conn *websocket.Conn, r *auth.AuthenticatedRequest) *WSAsyncClient {
	host := r.Header.Get("X-Host-ID")
	clientType := r.Header.Get("X-Client-Type")

	logging.GetLogger().Infof("New WebSocket Connection from %s : URI path %s", conn.RemoteAddr().String(), r.URL.Path)

	wsClient := NewWSAsyncClientFromConnection(host, common.ServiceType(clientType), conn)
	wsClient.start()

	return wsClient
}

func (s *WSServer) serveMessages(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	// if X-Host-ID specified avoid having twice the same ID
	host := r.Header.Get("X-Host-ID")
	if host != "" {
		s.WSClientPool.RLock()
		for _, c := range s.clients {
			if c.GetHost() == host {
				logging.GetLogger().Errorf("host_id error, connection from %s(%s) conflicts with another one", r.RemoteAddr, host)
				w.Header().Set("Connection", "close")
				w.WriteHeader(http.StatusConflict)
				s.WSClientPool.RUnlock()
				return
			}
		}
		s.WSClientPool.RUnlock()
	}

	conn, err := websocket.Upgrade(w, &r.Request, nil, 1024, 1024)
	if err != nil {
		return
	}

	client := s.clientHandler(conn, r)
	s.AddClient(client)
	s.OnConnected(client)
}

func NewWSServer(server *Server, endpoint string) *WSServer {
	s := &WSServer{
		WSClientPool: NewWSClientPool(),
		HTTPServer:   server,
	}

	s.clientHandler = func(c *websocket.Conn, r *auth.AuthenticatedRequest) WSClient {
		client := DefaultClientHandler(c, r)
		return client
	}

	server.HandleFunc(endpoint, s.serveMessages)
	return s
}

func NewWSServerFromConfig(server *Server, endpoint string) *WSServer {
	return NewWSServer(server, endpoint)
}
