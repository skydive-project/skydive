/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package websocket

import (
	fmt "fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/websocket"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/rbac"
)

// IncomerHandler incoming client handler interface.
type IncomerHandler func(*websocket.Conn, *auth.AuthenticatedRequest) (Speaker, error)

// Server implements a websocket server. It owns a Pool of incoming Speakers.
type Server struct {
	common.RWMutex
	*incomerPool
	server           *shttp.Server
	incomerHandler   IncomerHandler
	writeCompression bool
	queueSize        int
	pingDelay        time.Duration
	pongTimeout      time.Duration
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
		logging.GetLogger().Errorf("Unable to upgrade the websocket connection for %s: %s", r.RemoteAddr, err)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// call the incomerHandler that will create the Speaker
	c, err = s.incomerHandler(conn, r)
	if err != nil {
		logging.GetLogger().Warningf("Unable to accept incomer from %s: %s", r.RemoteAddr, err)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	// add the new Speaker to the server pool
	s.AddClient(c)

	// notify the pool listeners that the speaker is connected
	s.OnConnected(c)
}

func (s *Server) newIncomingClient(conn *websocket.Conn, r *auth.AuthenticatedRequest) (*wsIncomingClient, error) {
	logging.GetLogger().Infof("New WebSocket Connection from %s : URI path %s", conn.RemoteAddr().String(), r.URL.Path)

	clientType := common.ServiceType(getRequestParameter(&r.Request, "X-Client-Type"))
	if clientType == "" {
		clientType = common.UnknownService
	}

	var clientProtocol Protocol
	if err := clientProtocol.parse(getRequestParameter(&r.Request, "X-Client-Protocol")); err != nil {
		return nil, fmt.Errorf("Protocol requested error: %s", err)
	}

	svc, _ := common.ServiceAddressFromString(conn.RemoteAddr().String())
	url, _ := url.Parse(fmt.Sprintf("http://%s:%d%s", svc.Addr, svc.Port, r.URL.Path+"?"+r.URL.RawQuery))

	wsconn := newConn(s.server.Host, clientType, clientProtocol, url, r.Header, s.queueSize, s.writeCompression)
	wsconn.conn = conn
	wsconn.RemoteHost = getRequestParameter(&r.Request, "X-Host-ID")

	// NOTE(safchain): fallback to remote addr if host id not provided
	// should be removed, connection should be refused if host id not provided
	if wsconn.RemoteHost == "" {
		wsconn.RemoteHost = r.RemoteAddr
	}

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(s.pongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(s.pongTimeout))
		return nil
	})

	c := &wsIncomingClient{
		Conn: wsconn,
	}
	wsconn.wsSpeaker = c

	atomic.StoreInt32((*int32)(c.State), common.RunningState)

	// send a first ping to help firefox and some other client which wait for a
	// first ping before doing something
	c.sendPing()

	wsconn.pingTicker = time.NewTicker(s.pingDelay)

	c.Start()

	return c, nil
}

// NewServer returns a new Server. The given auth backend will validate the credentials
func NewServer(server *shttp.Server, endpoint string, authBackend shttp.AuthenticationBackend, writeCompression bool, queueSize int, pingDelay, pongTimeout time.Duration) *Server {
	s := &Server{
		incomerPool:      newIncomerPool(endpoint), // server inherits from a Speaker pool
		server:           server,
		writeCompression: writeCompression,
		queueSize:        queueSize,
		pingDelay:        pingDelay,
		pongTimeout:      pongTimeout,
	}

	s.incomerHandler = func(conn *websocket.Conn, r *auth.AuthenticatedRequest) (Speaker, error) {
		return s.newIncomingClient(conn, r)
	}

	server.HandleFunc(endpoint, s.serveMessages, authBackend)
	return s
}
