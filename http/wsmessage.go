/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/websocket"
	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

const (
	WilcardNamespace = "*"
	BulkMsgType      = "BulkMessage"
)

var DefaultRequestTimeout = 10 * time.Second

// WSMessage describes a WebSocket message
type WSMessage struct {
	Namespace string
	Type      string
	UUID      string `json:",omitempty"`
	Obj       *json.RawMessage
	Status    int
}

func (g WSMessage) Marshal() []byte {
	j, _ := json.Marshal(g)
	return j
}

func (g WSMessage) Bytes() []byte {
	return g.Marshal()
}

func (g *WSMessage) Reply(v interface{}, kind string, status int) *WSMessage {
	b, _ := json.Marshal(v)
	raw := json.RawMessage(b)

	return &WSMessage{
		Namespace: g.Namespace,
		UUID:      g.UUID,
		Obj:       &raw,
		Type:      kind,
		Status:    status,
	}
}

func NewWSMessage(ns string, tp string, v interface{}, uuids ...string) *WSMessage {
	var u string
	if len(uuids) != 0 {
		u = uuids[0]
	} else {
		v4, _ := uuid.NewV4()
		u = v4.String()
	}

	b, _ := json.Marshal(v)
	raw := json.RawMessage(b)

	return &WSMessage{
		Namespace: ns,
		Type:      tp,
		UUID:      u,
		Obj:       &raw,
		Status:    http.StatusOK,
	}
}

type WSBulkMessage []json.RawMessage

type WSMessageHandler interface {
	OnWSMessage(c WSClient, m WSMessage)
}

type WSClientNamespaceEventHandler struct {
	DefaultWSClientEventHandler
	eventHandlersLock sync.RWMutex
	nsEventHandlers   map[string][]WSMessageHandler
}

func NewWSClientNamespaceEventHandler() *WSClientNamespaceEventHandler {
	return &WSClientNamespaceEventHandler{
		nsEventHandlers: make(map[string][]WSMessageHandler),
	}
}

type WSMessageAsyncClient struct {
	*WSAsyncClient
	*WSClientNamespaceEventHandler
	DefaultWSClientEventHandler
	nsSubscribed   map[string]bool
	replyChanMutex sync.RWMutex
	replyChan      map[string]chan *WSMessage
}

func (c *WSMessageAsyncClient) Send(m Message) {
	if msg, ok := m.(WSMessage); ok {
		if _, ok := c.nsSubscribed[msg.Namespace]; !ok {
			if _, ok := c.nsSubscribed[WilcardNamespace]; !ok {
				return
			}
		}
	}
	c.WSAsyncClient.Send(m)
}

func (a *WSClientNamespaceEventHandler) AddMessageHandler(h WSMessageHandler, namespaces []string) {
	a.eventHandlersLock.Lock()
	// add this handler per namespace
	for _, ns := range namespaces {
		if _, ok := a.nsEventHandlers[ns]; !ok {
			a.nsEventHandlers[ns] = []WSMessageHandler{h}
		} else {
			a.nsEventHandlers[ns] = append(a.nsEventHandlers[ns], h)
		}
	}
	a.eventHandlersLock.Unlock()
}

func (a *WSClientNamespaceEventHandler) dispatchMessage(c WSClient, m WSMessage) {
	a.eventHandlersLock.RLock()
	for _, l := range a.nsEventHandlers[m.Namespace] {
		l.OnWSMessage(c, m)
	}
	for _, l := range a.nsEventHandlers[WilcardNamespace] {
		l.OnWSMessage(c, m)
	}
	a.eventHandlersLock.RUnlock()
}

func (a *WSMessageAsyncClient) OnMessage(c WSClient, m Message) {
	var msg WSMessage
	if err := json.Unmarshal(m.Bytes(), &msg); err != nil {
		logging.GetLogger().Errorf("Error while decoding WSMessage %s", err.Error())
		return
	}

	if msg.Type == BulkMsgType {
		var bulkMessage WSBulkMessage
		if err := json.Unmarshal([]byte(*msg.Obj), &bulkMessage); err != nil {
			for _, msg := range bulkMessage {
				a.OnMessage(c, WSRawMessage([]byte(msg)))
			}
		}
		return
	}

	a.dispatchMessage(c, msg)
}

func (c *WSMessageAsyncClient) OnWSMessage(client WSClient, msg WSMessage) {
	c.replyChanMutex.RLock()
	if ch, ok := c.replyChan[msg.UUID]; ok {
		ch <- &msg
	}
	c.replyChanMutex.RUnlock()
}

func (c *WSMessageAsyncClient) Request(msg *WSMessage, timeout time.Duration) (*WSMessage, error) {
	ch := make(chan *WSMessage, 1)

	c.replyChanMutex.Lock()
	c.replyChan[msg.UUID] = ch
	c.replyChanMutex.Unlock()

	defer func() {
		c.replyChanMutex.Lock()
		delete(c.replyChan, msg.UUID)
		close(ch)
		c.replyChanMutex.Unlock()
	}()

	c.Send(msg)

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		return nil, common.ErrTimeout
	}
}

func NewWSMessageAsyncClient(client *WSAsyncClient) *WSMessageAsyncClient {
	msgClient := &WSMessageAsyncClient{
		WSAsyncClient:                 client,
		WSClientNamespaceEventHandler: NewWSClientNamespaceEventHandler(),
		nsSubscribed:                  make(map[string]bool),
		replyChan:                     make(map[string]chan *WSMessage),
	}

	// We add an event handler to unmarshal message to WSMessage
	// and catch responses to requests
	client.AddEventHandler(msgClient)
	msgClient.AddMessageHandler(msgClient, []string{WilcardNamespace})

	return msgClient
}

type WSMessageClientPool struct {
	*WSClientPool
	*WSClientNamespaceEventHandler
}

func (a *WSMessageClientPool) AddClient(client WSClient) {
	a.WSClientPool.AddClient(client)
	if wsMessage, ok := client.(*WSMessageAsyncClient); ok {
		// This is to call WSClientPool.OnWSMessage
		wsMessage.AddMessageHandler(a, []string{WilcardNamespace})
	}
}

func (p *WSMessageClientPool) OnWSMessage(c WSClient, m WSMessage) {
	p.dispatchMessage(c, m)
}

func (p *WSMessageClientPool) OnMessage(c WSClient, m Message) {
}

func (p *WSMessageClientPool) OnDisconnected(c WSClient) {
}

func (p *WSMessageClientPool) OnConnected(c WSClient) {
	if msg, ok := c.(*WSMessageAsyncClient); ok {
		msg.AddMessageHandler(p, []string{WilcardNamespace})
	}
}

func NewWSMessageClientPool(pool *WSClientPool) *WSMessageClientPool {
	msgPool := &WSMessageClientPool{
		WSClientPool:                  pool,
		WSClientNamespaceEventHandler: NewWSClientNamespaceEventHandler(),
	}
	pool.AddEventHandler(msgPool)
	return msgPool
}

type WSMessageServer struct {
	*WSServer
	pool *WSMessageClientPool
}

func (s *WSMessageServer) OnWSMessage(c WSClient, m WSMessage) {
}

func (s *WSMessageServer) AddMessageHandler(h WSMessageHandler, namespaces []string) {
	s.pool.AddMessageHandler(h, namespaces)
}

func (s *WSMessageServer) Request(host string, request *WSMessage, timeout time.Duration) (*WSMessage, error) {
	client := s.GetClient(host)
	if client == nil {
		return nil, common.ErrNotFound
	}

	return client.(*WSMessageAsyncClient).Request(request, timeout)
}

func NewWSMessageServer(server *WSServer) *WSMessageServer {
	pool := NewWSMessageClientPool(server.WSClientPool)
	s := &WSMessageServer{
		pool:     pool,
		WSServer: server,
	}

	s.WSServer.clientHandler = func(conn *websocket.Conn, r *auth.AuthenticatedRequest) WSClient {
		c := NewWSMessageAsyncClient(DefaultClientHandler(conn, r))

		// from header
		if namespaces, ok := r.Header["X-Websocket-Namespace"]; ok {
			for _, ns := range namespaces {
				c.nsSubscribed[ns] = true
			}
		}

		// from parameter, useful for browser client
		if namespaces, ok := r.URL.Query()["x-websocket-namespace"]; ok {
			for _, ns := range namespaces {
				c.nsSubscribed[ns] = true
			}
		}

		// if empty use wilcard for backward compatibility
		if len(c.nsSubscribed) == 0 {
			c.nsSubscribed[WilcardNamespace] = true
		}

		c.AddMessageHandler(s, []string{WilcardNamespace})
		return c
	}

	return s
}
