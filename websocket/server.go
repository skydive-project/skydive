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

package websocket

import (
	"net/http"
	"strings"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/websocket"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/rbac"
)

// IncomerHandler incoming client handler interface.
type IncomerHandler func(*websocket.Conn, *auth.AuthenticatedRequest) Speaker

// Server implements a websocket server. It owns a Pool of incoming Speakers.
type Server struct {
	common.RWMutex
	*incomerPool
	server         *shttp.Server
	incomerHandler IncomerHandler
}

func defaultIncomerHandler(conn *websocket.Conn, r *auth.AuthenticatedRequest) *wsIncomingClient {
	logging.GetLogger().Infof("New WebSocket Connection from %s : URI path %s", conn.RemoteAddr().String(), r.URL.Path)

	c := newIncomingClient(conn, r)
	c.start()

	return c
}

func getRequestParameter(r *http.Request, name string) string {
	param := r.Header.Get(name)
	if param == "" {
		param = r.URL.Query().Get(strings.ToLower(name))
	}
	return param
}

func (s *Server) serveMessages(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	logging.GetLogger().Debugf("Enforcing websocket for %s, %s", s.name, r.Username)
	if rbac.Enforce(r.Username, "websocket", s.name) == false {
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// if X-Host-ID specified avoid having twice the same ID
	host := getRequestParameter(&r.Request, "X-Host-ID")
	if host == "" {
		host = r.RemoteAddr
	}
	logging.GetLogger().Debugf("Serving messages for client %s for pool %s", host, s.GetName())

	s.incomerPool.RLock()
	c := s.GetSpeakerByRemoteHost(host)
	s.incomerPool.RUnlock()
	if c != nil {
		logging.GetLogger().Errorf("host_id(%s) conflict, same host_id used by %s", host, r.RemoteAddr)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusConflict)
		return
	}

	// reply with host-id and service type of the server
	header := http.Header{}
	header.Set("X-Host-ID", s.server.Host)
	header.Set("X-Service-Type", s.server.ServiceType.String())

	conn, err := websocket.Upgrade(w, &r.Request, header, 1024, 1024)
	if err != nil {
		return
	}

	// call the incomerHandler that will create the Speaker
	c = s.incomerHandler(conn, r)

	// add the new Speaker to the server pool
	s.AddClient(c)

	// notify the pool listeners that the speaker is connected
	s.OnConnected(c)
}

// NewServer returns a new Server. The given auth backend will validate the credentials
func NewServer(server *shttp.Server, endpoint string, authBackend shttp.AuthenticationBackend) *Server {
	s := &Server{
		incomerPool: newIncomerPool(endpoint), // server inherites from a Speaker pool
		incomerHandler: func(c *websocket.Conn, a *auth.AuthenticatedRequest) Speaker {
			return defaultIncomerHandler(c, a)
		},
		server: server,
	}

	server.HandleFunc(endpoint, s.serveMessages, authBackend)
	return s
}
