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

type WSIncomerHandler func(*websocket.Conn, *auth.AuthenticatedRequest) WSSpeaker

type WSServer struct {
	sync.RWMutex
	*wsIncomerPool
	incomerHandler WSIncomerHandler
}

func defaultIncomerHandler(conn *websocket.Conn, r *auth.AuthenticatedRequest) *wsIncomingClient {
	host := r.Header.Get("X-Host-ID")
	clientType := r.Header.Get("X-Client-Type")

	logging.GetLogger().Infof("New WebSocket Connection from %s : URI path %s", conn.RemoteAddr().String(), r.URL.Path)

	c := newIncomingWSClient(host, common.ServiceType(clientType), conn)
	c.start()

	return c
}

func (s *WSServer) serveMessages(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	// if X-Host-ID specified avoid having twice the same ID
	host := r.Header.Get("X-Host-ID")
	if host != "" {
		s.wsIncomerPool.RLock()
		for _, c := range s.clients {
			if c.GetHost() == host {
				logging.GetLogger().Errorf("host_id error, connection from %s(%s) conflicts with another one", r.RemoteAddr, host)
				w.Header().Set("Connection", "close")
				w.WriteHeader(http.StatusConflict)
				s.wsIncomerPool.RUnlock()
				return
			}
		}
		s.wsIncomerPool.RUnlock()
	}

	conn, err := websocket.Upgrade(w, &r.Request, nil, 1024, 1024)
	if err != nil {
		return
	}

	// call the incomerHandler that will create the WSSpeaker
	c := s.incomerHandler(conn, r)

	// add the new WSSPeaker to the server pool
	s.AddClient(c)

	// notify the pool listeners that the speaker is connected
	s.OnConnected(c)
}

func NewWSServer(server *Server, endpoint string) *WSServer {
	s := &WSServer{
		wsIncomerPool: newWSIncomerPool(), // server inherites from a WSSpeaker pool
		incomerHandler: func(c *websocket.Conn, a *auth.AuthenticatedRequest) WSSpeaker {
			return defaultIncomerHandler(c, a)
		},
	}

	server.HandleFunc(endpoint, s.serveMessages)
	return s
}

func NewWSServerFromConfig(server *Server, endpoint string) *WSServer {
	return NewWSServer(server, endpoint)
}
