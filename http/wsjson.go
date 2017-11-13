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
	"errors"
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
	// WilcardNamespace is the namespace used as wildcard. It is used by listeners to filter callbacks.
	WilcardNamespace = "*"
	BulkMsgType      = "BulkMessage"
)

// DefaultRequestTimeout default timeout used for Request/Reply JSON message.
var DefaultRequestTimeout = 10 * time.Second

// WSJSONMessage is JSON based message on top of WSMessage. It implements to
// WSMessage interface and can be sent with via a WSSpeaker.
type WSJSONMessage struct {
	Namespace string
	Type      string
	UUID      string `json:",omitempty"`
	Obj       *json.RawMessage
	Status    int
}

// Marshal serializes the WSJSONMessage into a JSON string.
func (g WSJSONMessage) Marshal() []byte {
	j, _ := json.Marshal(g)
	return j
}

// Bytes see Marshal
func (g WSJSONMessage) Bytes() []byte {
	return g.Marshal()
}

// Reply returns a reply message with the given value, type and status.
// Basically it return a new WSJSONMessage with the correct Namespace and UUID.
func (g *WSJSONMessage) Reply(v interface{}, kind string, status int) *WSJSONMessage {
	b, _ := json.Marshal(v)
	raw := json.RawMessage(b)

	return &WSJSONMessage{
		Namespace: g.Namespace,
		UUID:      g.UUID,
		Obj:       &raw,
		Type:      kind,
		Status:    status,
	}
}

// NewWSJSONMessage creates a new WSJSONMessage with the given namespace, type, value
// and optionally the UUID.
func NewWSJSONMessage(ns string, tp string, v interface{}, uuids ...string) *WSJSONMessage {
	var u string
	if len(uuids) != 0 {
		u = uuids[0]
	} else {
		v4, _ := uuid.NewV4()
		u = v4.String()
	}

	b, _ := json.Marshal(v)
	raw := json.RawMessage(b)

	return &WSJSONMessage{
		Namespace: ns,
		Type:      tp,
		UUID:      u,
		Obj:       &raw,
		Status:    http.StatusOK,
	}
}

// WSBulkMessage bulk of RawMessage.
type WSBulkMessage []json.RawMessage

// WSSpeakerJSONMessageHandler interface used to receive JSON messages.
type WSSpeakerJSONMessageHandler interface {
	OnWSJSONMessage(c WSSpeaker, m *WSJSONMessage)
}

// WSSpeakerJSONMessageDispatcher interface is used to dispatch OnWSJSONMessage events.
type WSSpeakerJSONMessageDispatcher interface {
	AddJSONMessageHandler(h WSSpeakerJSONMessageHandler, namespaces []string)
}

type wsJSONSpeakerEventDispatcher struct {
	eventHandlersLock sync.RWMutex
	nsEventHandlers   map[string][]WSSpeakerJSONMessageHandler
}

func newWSJSONSpeakerEventDispatcher() *wsJSONSpeakerEventDispatcher {
	return &wsJSONSpeakerEventDispatcher{
		nsEventHandlers: make(map[string][]WSSpeakerJSONMessageHandler),
	}
}

// AddJSONMessageHandler adds a new listener for JSON messages.
func (a *wsJSONSpeakerEventDispatcher) AddJSONMessageHandler(h WSSpeakerJSONMessageHandler, namespaces []string) {
	a.eventHandlersLock.Lock()
	// add this handler per namespace
	for _, ns := range namespaces {
		if _, ok := a.nsEventHandlers[ns]; !ok {
			a.nsEventHandlers[ns] = []WSSpeakerJSONMessageHandler{h}
		} else {
			a.nsEventHandlers[ns] = append(a.nsEventHandlers[ns], h)
		}
	}
	a.eventHandlersLock.Unlock()
}

func (a *wsJSONSpeakerEventDispatcher) dispatchMessage(c *WSJSONSpeaker, m *WSJSONMessage) {
	// check whether it is a reply
	if c.onReply(m) {
		return
	}

	a.eventHandlersLock.RLock()
	for _, l := range a.nsEventHandlers[m.Namespace] {
		l.OnWSJSONMessage(c, m)
	}
	for _, l := range a.nsEventHandlers[WilcardNamespace] {
		l.OnWSJSONMessage(c, m)
	}
	a.eventHandlersLock.RUnlock()
}

// OnDisconnected is implemented here to avoid infinite loop since the default
// implemtation is triggering OnDisconnected too.
func (p *wsJSONSpeakerEventDispatcher) OnDisconnected(c WSSpeaker) {
}

// OnConnected is implemented here to avoid infinite loop since the default
// implemtation is triggering OnDisconnected too.
func (p *wsJSONSpeakerEventDispatcher) OnConnected(c WSSpeaker) {
}

type wsJSONSpeakerPoolEventDispatcher struct {
	dispatcher *wsJSONSpeakerEventDispatcher
	pool       WSSpeakerPool
}

// AddJSONMessageHandler adds a new listener for JSON messages.
func (d *wsJSONSpeakerPoolEventDispatcher) AddJSONMessageHandler(h WSSpeakerJSONMessageHandler, namespaces []string) {
	d.dispatcher.AddJSONMessageHandler(h, namespaces)
	for _, client := range d.pool.GetSpeakers() {
		client.(*WSJSONSpeaker).AddJSONMessageHandler(h, namespaces)
	}
}

func (d *wsJSONSpeakerPoolEventDispatcher) AddJSONSpeaker(c *WSJSONSpeaker) {
	d.dispatcher.eventHandlersLock.RLock()
	for ns, handlers := range d.dispatcher.nsEventHandlers {
		for _, handler := range handlers {
			c.AddJSONMessageHandler(handler, []string{ns})
		}
	}
	d.dispatcher.eventHandlersLock.RUnlock()
}

func newWSJSONSpeakerPoolEventDispatcher(pool WSSpeakerPool) *wsJSONSpeakerPoolEventDispatcher {
	return &wsJSONSpeakerPoolEventDispatcher{
		dispatcher: newWSJSONSpeakerEventDispatcher(),
		pool:       pool,
	}
}

// WSJSONSpeaker is a WSSpeaker able to handle JSON Message and Request/Reply calls.
type WSJSONSpeaker struct {
	WSSpeaker
	*wsJSONSpeakerEventDispatcher
	nsSubscribed   map[string]bool
	replyChanMutex sync.RWMutex
	replyChan      map[string]chan *WSJSONMessage
}

// Send sends a message according to the namespace.
func (s *WSJSONSpeaker) Send(m WSMessage) {
	if msg, ok := m.(WSJSONMessage); ok {
		if _, ok := s.nsSubscribed[msg.Namespace]; !ok {
			if _, ok := s.nsSubscribed[WilcardNamespace]; !ok {
				return
			}
		}
	}

	s.WSSpeaker.SendMessage(m)
}

func (s *WSJSONSpeaker) onReply(m *WSJSONMessage) bool {
	s.replyChanMutex.RLock()
	ch, ok := s.replyChan[m.UUID]
	if ok {
		ch <- m
	}
	s.replyChanMutex.RUnlock()

	return ok
}

// Request sends a JSON message request waiting for a reply using the given timeout.
func (s *WSJSONSpeaker) Request(m *WSJSONMessage, timeout time.Duration) (*WSJSONMessage, error) {
	ch := make(chan *WSJSONMessage, 1)

	s.replyChanMutex.Lock()
	s.replyChan[m.UUID] = ch
	s.replyChanMutex.Unlock()

	defer func() {
		s.replyChanMutex.Lock()
		delete(s.replyChan, m.UUID)
		close(ch)
		s.replyChanMutex.Unlock()
	}()

	s.Send(m)

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		return nil, common.ErrTimeout
	}
}

// OnMessage checks that the WSMessage comes from a WSJSONSpeaker. It parses
// the JSON message and then dispatch the message to the proper listeners according
// to the namespace.
func (s *WSJSONSpeaker) OnMessage(c WSSpeaker, m WSMessage) {
	if c, ok := c.(*WSJSONSpeaker); ok {
		jm := &WSJSONMessage{}
		if err := json.Unmarshal(m.Bytes(), jm); err != nil {
			logging.GetLogger().Errorf("Error while decoding WSJSONMessage %s", err.Error())
			return
		}

		if jm.Type == BulkMsgType {
			var bulkMessage WSBulkMessage
			if err := json.Unmarshal([]byte(*jm.Obj), &bulkMessage); err != nil {
				for _, jm := range bulkMessage {
					s.OnMessage(c, WSRawMessage([]byte(jm)))
				}
			}
			return
		}

		s.wsJSONSpeakerEventDispatcher.dispatchMessage(c, jm)
	}
}

func newWSJSONSpeaker(c WSSpeaker) *WSJSONSpeaker {
	s := &WSJSONSpeaker{
		WSSpeaker:                    c,
		wsJSONSpeakerEventDispatcher: newWSJSONSpeakerEventDispatcher(),
		nsSubscribed:                 make(map[string]bool),
		replyChan:                    make(map[string]chan *WSJSONMessage),
	}

	// subscribing to itself so that the WSJSONSpeaker can get WSMessage and can convert them
	// to WSJSONMessage and then forward them to its own even listeners.
	s.AddEventHandler(s)
	return s
}

func (c *WSClient) UpgradeToWSJSONSpeaker() *WSJSONSpeaker {
	js := newWSJSONSpeaker(c)
	c.wsSpeaker = js
	return js
}

func (c *wsIncomingClient) upgradeToWSJSONSpeaker() *WSJSONSpeaker {
	js := newWSJSONSpeaker(c)
	c.wsSpeaker = js
	return js
}

// WSJSONSpeakerPool is the interface of a pool of WSJSONSpeakers.
type WSJSONSpeakerPool interface {
	WSSpeakerPool
	WSSpeakerJSONMessageDispatcher
	Request(host string, request *WSJSONMessage, timeout time.Duration) (*WSJSONMessage, error)
}

// WSJSONClientPool is a WSClientPool able to send WSJSONMessage.
type WSJSONClientPool struct {
	*WSClientPool
	*wsJSONSpeakerPoolEventDispatcher
}

// AddClient adds a WSClient to the pool.
func (a *WSJSONClientPool) AddClient(c WSSpeaker) error {
	if wc, ok := c.(*WSClient); ok {
		jsonSpeaker := wc.UpgradeToWSJSONSpeaker()
		a.WSClientPool.AddClient(jsonSpeaker)
		a.wsJSONSpeakerPoolEventDispatcher.AddJSONSpeaker(jsonSpeaker)
	} else {
		return errors.New("wrong client type")
	}
	return nil
}

// Request sends a Request JSON message to the WSSpeaker of the given host.
func (s *WSJSONClientPool) Request(host string, request *WSJSONMessage, timeout time.Duration) (*WSJSONMessage, error) {
	c := s.WSClientPool.GetSpeakerByHost(host)
	if c == nil {
		return nil, common.ErrNotFound
	}

	return c.(*WSJSONSpeaker).Request(request, timeout)
}

// NewWSJSONClientPool returns a new WSJSONClientPool.
func NewWSJSONClientPool() *WSJSONClientPool {
	pool := NewWSClientPool()
	return &WSJSONClientPool{
		WSClientPool:                     pool,
		wsJSONSpeakerPoolEventDispatcher: newWSJSONSpeakerPoolEventDispatcher(pool),
	}
}

// WSJSONServer is a WSServer able to handle WSJSONSpeaker.
type WSJSONServer struct {
	*WSServer
	*wsJSONSpeakerPoolEventDispatcher
}

// Request sends a Request JSON message to the WSSpeaker of the given host.
func (s *WSJSONServer) Request(host string, request *WSJSONMessage, timeout time.Duration) (*WSJSONMessage, error) {
	c := s.WSServer.GetSpeakerByHost(host)
	if c == nil {
		return nil, common.ErrNotFound
	}

	return c.(*WSJSONSpeaker).Request(request, timeout)
}

// OnMessage websocket event.
func (s *WSJSONServer) OnMessage(c WSSpeaker, m WSMessage) {
}

// OnConnected websocket event.
func (s *WSJSONServer) OnConnected(c WSSpeaker) {
}

// OnDisconnected removes the WSSpeaker from the incomer pool.
func (s *WSJSONServer) OnDisconnected(c WSSpeaker) {
	s.WSServer.wsIncomerPool.RemoveClient(c)
}

// NewWSJSONServer returns a new WSJSONServer
func NewWSJSONServer(server *WSServer) *WSJSONServer {
	s := &WSJSONServer{
		WSServer: server,
		wsJSONSpeakerPoolEventDispatcher: newWSJSONSpeakerPoolEventDispatcher(server),
	}

	s.WSServer.wsIncomerPool.AddEventHandler(s)

	// This incomerHandler upgrades the incomers to WSJSONSpeaker thus being able to parse JSONMessage.
	// The server set also the WSJsonSpeaker with the proper namspaces it subscribes to thanks to the
	// headers.
	s.WSServer.incomerHandler = func(conn *websocket.Conn, r *auth.AuthenticatedRequest) WSSpeaker {
		// the default incomer handler creates a standard wsIncomerClient that we upgrade to a WSJSONSpeaker
		// being able to handle the JSONMessage
		c := defaultIncomerHandler(conn, r).upgradeToWSJSONSpeaker()

		// from headers
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

		s.wsJSONSpeakerPoolEventDispatcher.AddJSONSpeaker(c)

		return c
	}

	return s
}
