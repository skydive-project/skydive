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
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/websocket"

	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/rbac"
	"github.com/skydive-project/skydive/graffiti/service"
	shttp "github.com/skydive-project/skydive/http"
)

type clientPromoter func(c *wsIncomingClient) (Speaker, error)

// IncomerHandler incoming client handler interface.
type IncomerHandler func(*websocket.Conn, *auth.AuthenticatedRequest, clientPromoter) (Speaker, error)

// Server implements a websocket server. It owns a Pool of incoming Speakers.
type Server struct {
	*incomerPool
	server         *shttp.Server
	incomerHandler IncomerHandler
	opts           ServerOpts
}

// ServerOpts defines server options
type ServerOpts struct {
	WriteCompression bool
	QueueSize        int
	PingDelay        time.Duration
	PongTimeout      time.Duration
	Logger           logging.Logger
	AuthBackend      shttp.AuthenticationBackend
}

func getRequestParameter(r *http.Request, name string) string {
	param := r.Header.Get(name)
	if param == "" {
		param = r.URL.Query().Get(strings.ToLower(name))
	}
	return param
}

func (s *Server) serveMessages(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	s.opts.Logger.Debugf("Enforcing websocket for %s, %s", s.name, r.Username)
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
	s.opts.Logger.Debugf("Serving messages for client %s for pool %s", host, s.GetName())

	if c, err := s.GetSpeakerByRemoteHost(host); err == nil {
		s.opts.Logger.Errorf("host_id '%s' (%s) conflicts, same host_id used by %s:%s", host, r.RemoteAddr, c.GetRemoteHost(), c.GetRemoteServiceType())
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
		s.opts.Logger.Errorf("Unable to upgrade the websocket connection for %s: %s", r.RemoteAddr, err)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// call the incomerHandler that will create the Speaker
	_, err = s.incomerHandler(conn, r, func(c *wsIncomingClient) (Speaker, error) { return c, nil })
	if err != nil {
		s.opts.Logger.Warningf("Unable to accept incomer from %s: %s", r.RemoteAddr, err)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
}

func (s *Server) newIncomingClient(conn *websocket.Conn, r *auth.AuthenticatedRequest, promoter clientPromoter) (*wsIncomingClient, error) {
	s.opts.Logger.Infof("New WebSocket Connection from %s : URI path %s", conn.RemoteAddr().String(), r.URL.Path)

	clientType := service.Type(getRequestParameter(&r.Request, "X-Client-Type"))
	if clientType == "" {
		clientType = service.UnknownService
	}

	var clientProtocol Protocol
	if err := clientProtocol.parse(getRequestParameter(&r.Request, "X-Client-Protocol")); err != nil {
		return nil, fmt.Errorf("Protocol requested error: %s", err)
	}

	svc, _ := service.AddressFromString(conn.RemoteAddr().String())
	url, _ := url.Parse(fmt.Sprintf("http://%s:%d%s", svc.Addr, svc.Port, r.URL.Path+"?"+r.URL.RawQuery))

	opts := ClientOpts{
		QueueSize:        s.opts.QueueSize,
		WriteCompression: s.opts.WriteCompression,
		Logger:           s.opts.Logger,
	}

	wsconn := newConn(s.server.Host, clientType, clientProtocol, url, r.Header, opts)
	wsconn.conn = conn
	wsconn.RemoteHost = getRequestParameter(&r.Request, "X-Host-ID")

	// NOTE(safchain): fallback to remote addr if host id not provided
	// should be removed, connection should be refused if host id not provided
	if wsconn.RemoteHost == "" {
		wsconn.RemoteHost = r.RemoteAddr
	}

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(s.opts.PongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(s.opts.PongTimeout))
		return nil
	})

	c := &wsIncomingClient{
		Conn: wsconn,
	}
	wsconn.wsSpeaker = c

	pc, err := promoter(c)
	if err != nil {
		return nil, err
	}

	c.State.Store(service.RunningState)

	// add the new Speaker to the server pool
	s.AddClient(pc)

	// notify the pool listeners that the speaker is connected
	s.OnConnected(pc)

	// send a first ping to help firefox and some other client which wait for a
	// first ping before doing something
	c.sendPing()

	wsconn.pingTicker = time.NewTicker(s.opts.PingDelay)

	c.Start()

	return c, nil
}

// NewServer returns a new Server. The given auth backend will validate the credentials
func NewServer(server *shttp.Server, endpoint string, opts ServerOpts) *Server {
	if opts.Logger == nil {
		opts.Logger = logging.GetLogger()
	}

	poolOpts := PoolOpts{
		Logger: opts.Logger,
	}

	s := &Server{
		incomerPool: newIncomerPool(endpoint, poolOpts), // server inherits from a Speaker pool
		server:      server,
		opts:        opts,
	}

	s.incomerHandler = func(conn *websocket.Conn, r *auth.AuthenticatedRequest, promoter clientPromoter) (Speaker, error) {
		return s.newIncomingClient(conn, r, promoter)
	}

	server.HandleFunc(endpoint, s.serveMessages, opts.AuthBackend)
	return s
}
