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
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/abbot/go-http-auth"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

const (
	// WildcardNamespace is the namespace used as wildcard. It is used by listeners to filter callbacks.
	WildcardNamespace = "*"
	ProtobufProtocol  = "protobuf"
	JsonProtocol      = "json"
)

// DefaultRequestTimeout default timeout used for Request/Reply JSON message.
var DefaultRequestTimeout = 10 * time.Second

type WSStructMessageJSON struct {
	Namespace string
	Type      string
	UUID      string
	Status    int64
	Obj       *json.RawMessage
}

type WSStructMessage struct {
	Protocol  string
	Namespace string
	Type      string
	UUID      string
	Status    int64
	value     interface{}

	JsonObj            *json.RawMessage
	jsonSerialized     []byte
	ProtobufObj        []byte
	protobufSerialized []byte
}

// Debug representation of the struct WSStructMessage
func (g *WSStructMessage) Debug() string {
	if g.Protocol == JsonProtocol {
		return fmt.Sprintf("Namespace %s Type %s UUID %s Status %d Obj JSON (%d) : %q",
			g.Namespace, g.Type, g.UUID, g.Status, len(*g.JsonObj), string(*g.JsonObj))
	}
	return fmt.Sprintf("Namespace %s Type %s UUID %s Status %d Obj Protobuf (%d bytes)",
		g.Namespace, g.Type, g.UUID, g.Status, len(g.ProtobufObj))
}

// Marshal serializes the WSStructMessage into a JSON string.
func (g *WSStructMessageJSON) Marshal() []byte {
	j, err := json.Marshal(g)
	if err != nil {
		panic("JSON Marshal WSStructMessage encode failed")
	}
	return j
}

func (g *WSStructMessageProtobuf) Marshal() []byte {
	b, err := proto.Marshal(g)
	if err != nil {
		panic("Protobuf Marshal WSStructMessage encode failed")
	}
	return b
}

// Bytes see Marshal
func (g WSStructMessage) Bytes(protocol string) []byte {
	g.Protocol = protocol
	if g.Protocol == ProtobufProtocol {
		if len(g.protobufSerialized) > 0 {
			return g.protobufSerialized
		}
		g.marshalObj()
		msgProto := &WSStructMessageProtobuf{
			Namespace: g.Namespace,
			Type:      g.Type,
			UUID:      g.UUID,
			Status:    g.Status,
			Obj:       g.ProtobufObj,
		}
		g.protobufSerialized = msgProto.Marshal()
		return g.protobufSerialized
	}

	if len(g.jsonSerialized) > 0 {
		return g.jsonSerialized
	}
	g.marshalObj()
	msgJSON := &WSStructMessageJSON{
		Namespace: g.Namespace,
		Type:      g.Type,
		UUID:      g.UUID,
		Status:    g.Status,
		Obj:       g.JsonObj,
	}
	g.jsonSerialized = msgJSON.Marshal()
	return g.jsonSerialized
}

func (g *WSStructMessage) marshalObj() {
	if g.Protocol == JsonProtocol {
		b, err := json.Marshal(g.value)
		if err != nil {
			logging.GetLogger().Error("Json Marshal encode value failed", err)
			return
		}
		raw := json.RawMessage(b)
		g.JsonObj = &raw
	}
	if g.Protocol == ProtobufProtocol {
		b, err := json.Marshal(g.value)
		if err != nil {
			logging.GetLogger().Error("ProtobufProtocol : Json Marshal encode value failed", err)
			return
		}
		g.ProtobufObj = []byte(json.RawMessage(b))
	}
}

func (g *WSStructMessage) DecodeObj(obj interface{}) error {
	if g.Protocol == JsonProtocol {
		if err := common.JSONDecode(bytes.NewReader([]byte(*g.JsonObj)), obj); err != nil {
			return err
		}
	}
	if g.Protocol == ProtobufProtocol {
		if err := common.JSONDecode(bytes.NewReader(g.ProtobufObj), obj); err != nil {
			return err
		}
	}
	return nil
}

func (g *WSStructMessage) UnmarshalObj(obj interface{}) error {
	if g.Protocol == JsonProtocol {
		if err := json.Unmarshal(*g.JsonObj, obj); err != nil {
			return err
		}
	}
	if g.Protocol == ProtobufProtocol {
		if err := json.Unmarshal(g.ProtobufObj, obj); err != nil {
			return err
		}
	}
	return nil
}

// NewWSStructMessage creates a new WSStructMessage with the given namespace, type, value
// and optionally the UUID.
func NewWSStructMessage(ns string, tp string, v interface{}, uuids ...string) *WSStructMessage {
	var u string
	if len(uuids) != 0 {
		u = uuids[0]
	} else {
		v4, _ := uuid.NewV4()
		u = v4.String()
	}

	msg := &WSStructMessage{
		Namespace: ns,
		Type:      tp,
		UUID:      u,
		Status:    int64(http.StatusOK),
		value:     v,
	}
	return msg
}

// Reply returns a reply message with the given value, type and status.
// Basically it return a new WSStructMessage with the correct Namespace and UUID.
func (g *WSStructMessage) Reply(v interface{}, kind string, status int) *WSStructMessage {
	msg := &WSStructMessage{
		Namespace: g.Namespace,
		Type:      kind,
		UUID:      g.UUID,
		Status:    int64(status),
		value:     v,
	}
	return msg
}

// WSSpeakerStructMessageDispatcher interface is used to dispatch OnWSStructMessage events.
type WSSpeakerStructMessageDispatcher interface {
	AddStructMessageHandler(h WSSpeakerStructMessageHandler, namespaces []string)
}

type wsStructSpeakerEventDispatcher struct {
	eventHandlersLock common.RWMutex
	nsEventHandlers   map[string][]WSSpeakerStructMessageHandler
}

func newWSStructSpeakerEventDispatcher() *wsStructSpeakerEventDispatcher {
	return &wsStructSpeakerEventDispatcher{
		nsEventHandlers: make(map[string][]WSSpeakerStructMessageHandler),
	}
}

// AddStructMessageHandler adds a new listener for Struct messages.
func (a *wsStructSpeakerEventDispatcher) AddStructMessageHandler(h WSSpeakerStructMessageHandler, namespaces []string) {
	a.eventHandlersLock.Lock()
	// add this handler per namespace
	for _, ns := range namespaces {
		if _, ok := a.nsEventHandlers[ns]; !ok {
			a.nsEventHandlers[ns] = []WSSpeakerStructMessageHandler{h}
		} else {
			a.nsEventHandlers[ns] = append(a.nsEventHandlers[ns], h)
		}
	}
	a.eventHandlersLock.Unlock()
}

func (a *wsStructSpeakerEventDispatcher) dispatchMessage(c *WSStructSpeaker, m *WSStructMessage) {
	// check whether it is a reply
	if c.onReply(m) {
		return
	}

	a.eventHandlersLock.RLock()
	for _, l := range a.nsEventHandlers[m.Namespace] {
		l.OnWSStructMessage(c, m)
	}
	for _, l := range a.nsEventHandlers[WildcardNamespace] {
		l.OnWSStructMessage(c, m)
	}
	a.eventHandlersLock.RUnlock()
}

// OnDisconnected is implemented here to avoid infinite loop since the default
// implemtation is triggering OnDisconnected too.
func (p *wsStructSpeakerEventDispatcher) OnDisconnected(c WSSpeaker) {
}

// OnConnected is implemented here to avoid infinite loop since the default
// implemtation is triggering OnDisconnected too.
func (p *wsStructSpeakerEventDispatcher) OnConnected(c WSSpeaker) {
}

type wsStructSpeakerPoolEventDispatcher struct {
	dispatcher *wsStructSpeakerEventDispatcher
	pool       WSSpeakerPool
}

// AddStructMessageHandler adds a new listener for Struct messages.
func (d *wsStructSpeakerPoolEventDispatcher) AddStructMessageHandler(h WSSpeakerStructMessageHandler, namespaces []string) {
	d.dispatcher.AddStructMessageHandler(h, namespaces)
	for _, client := range d.pool.GetSpeakers() {
		client.(*WSStructSpeaker).AddStructMessageHandler(h, namespaces)
	}
}

func (d *wsStructSpeakerPoolEventDispatcher) AddStructSpeaker(c *WSStructSpeaker) {
	d.dispatcher.eventHandlersLock.RLock()
	for ns, handlers := range d.dispatcher.nsEventHandlers {
		for _, handler := range handlers {
			c.AddStructMessageHandler(handler, []string{ns})
		}
	}
	d.dispatcher.eventHandlersLock.RUnlock()
}

func newWSStructSpeakerPoolEventDispatcher(pool WSSpeakerPool) *wsStructSpeakerPoolEventDispatcher {
	return &wsStructSpeakerPoolEventDispatcher{
		dispatcher: newWSStructSpeakerEventDispatcher(),
		pool:       pool,
	}
}

// WSStructSpeaker is a WSSpeaker able to handle Struct Message and Request/Reply calls.
type WSStructSpeaker struct {
	WSSpeaker
	*wsStructSpeakerEventDispatcher
	nsSubscribed   map[string]bool
	replyChanMutex common.RWMutex
	replyChan      map[string]chan *WSStructMessage
}

// Send sends a message according to the namespace.
func (s *WSStructSpeaker) Send(m WSMessage) {
	if msg, ok := m.(WSStructMessage); ok {
		if _, ok := s.nsSubscribed[msg.Namespace]; !ok {
			if _, ok := s.nsSubscribed[WildcardNamespace]; !ok {
				return
			}
		}
	}

	s.WSSpeaker.SendMessage(m)
}

func (s *WSStructSpeaker) onReply(m *WSStructMessage) bool {
	s.replyChanMutex.RLock()
	ch, ok := s.replyChan[m.UUID]
	if ok {
		ch <- m
	}
	s.replyChanMutex.RUnlock()

	return ok
}

// Request sends a Struct message request waiting for a reply using the given timeout.
func (s *WSStructSpeaker) Request(m *WSStructMessage, timeout time.Duration) (*WSStructMessage, error) {
	ch := make(chan *WSStructMessage, 1)

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

// OnMessage checks that the WSMessage comes from a WSStructSpeaker. It parses
// the Struct message and then dispatch the message to the proper listeners according
// to the namespace.
func (s *WSStructSpeaker) OnMessage(c WSSpeaker, m WSMessage) {
	if c, ok := c.(*WSStructSpeaker); ok {
		msg := WSStructMessage{}
		if c.GetClientProtocol() == ProtobufProtocol {
			mProtobuf := WSStructMessageProtobuf{}
			b := m.Bytes(ProtobufProtocol)
			if err := proto.Unmarshal(b, &mProtobuf); err != nil {
				logging.GetLogger().Errorf("Error while decoding Protobuf WSStructMessage %s\n%s", err.Error(), hex.Dump(b))
				return
			}
			msg.Protocol = ProtobufProtocol
			msg.Namespace = mProtobuf.Namespace
			msg.Type = mProtobuf.Type
			msg.UUID = mProtobuf.UUID
			msg.Status = mProtobuf.Status
			msg.ProtobufObj = mProtobuf.Obj
		} else {
			mJSON := WSStructMessageJSON{}
			b := m.Bytes(JsonProtocol)
			if err := json.Unmarshal(b, &mJSON); err != nil {
				logging.GetLogger().Errorf("Error while decoding JSON WSStructMessage %s\n%s", err.Error(), hex.Dump(b))
				return
			}
			msg.Protocol = JsonProtocol
			msg.Namespace = mJSON.Namespace
			msg.Type = mJSON.Type
			msg.UUID = mJSON.UUID
			msg.Status = mJSON.Status
			msg.JsonObj = mJSON.Obj
		}
		s.wsStructSpeakerEventDispatcher.dispatchMessage(c, &msg)
	}
}

func newWSStructSpeaker(c WSSpeaker) *WSStructSpeaker {
	s := &WSStructSpeaker{
		WSSpeaker:                      c,
		wsStructSpeakerEventDispatcher: newWSStructSpeakerEventDispatcher(),
		nsSubscribed:                   make(map[string]bool),
		replyChan:                      make(map[string]chan *WSStructMessage),
	}

	// subscribing to itself so that the WSStructSpeaker can get WSMessage and can convert them
	// to WSStructMessage and then forward them to its own even listeners.
	s.AddEventHandler(s)
	return s
}

func (c *WSClient) UpgradeToWSStructSpeaker() *WSStructSpeaker {
	s := newWSStructSpeaker(c)
	c.Lock()
	c.wsSpeaker = s
	c.Unlock()
	return s
}

func (c *wsIncomingClient) upgradeToWSStructSpeaker() *WSStructSpeaker {
	s := newWSStructSpeaker(c)
	c.Lock()
	c.wsSpeaker = s
	c.Unlock()
	return s
}

// WSStructSpeakerPool is the interface of a pool of WSStructSpeakers.
type WSStructSpeakerPool interface {
	WSSpeakerPool
	WSSpeakerStructMessageDispatcher
	Request(host string, request *WSStructMessage, timeout time.Duration) (*WSStructMessage, error)
}

// WSStructClientPool is a WSClientPool able to send WSStructMessage.
type WSStructClientPool struct {
	*WSClientPool
	*wsStructSpeakerPoolEventDispatcher
}

// AddClient adds a WSClient to the pool.
func (a *WSStructClientPool) AddClient(c WSSpeaker) error {
	if wc, ok := c.(*WSClient); ok {
		speaker := wc.UpgradeToWSStructSpeaker()
		a.WSClientPool.AddClient(speaker)
		a.wsStructSpeakerPoolEventDispatcher.AddStructSpeaker(speaker)
	} else {
		return errors.New("wrong client type")
	}
	return nil
}

// Request sends a Request Struct message to the WSSpeaker of the given host.
func (s *WSStructClientPool) Request(host string, request *WSStructMessage, timeout time.Duration) (*WSStructMessage, error) {
	c := s.WSClientPool.GetSpeakerByHost(host)
	if c == nil {
		return nil, common.ErrNotFound
	}

	return c.(*WSStructSpeaker).Request(request, timeout)
}

// NewWSStructClientPool returns a new WSStructClientPool.
func NewWSStructClientPool(name string) *WSStructClientPool {
	pool := NewWSClientPool(name)
	return &WSStructClientPool{
		WSClientPool:                       pool,
		wsStructSpeakerPoolEventDispatcher: newWSStructSpeakerPoolEventDispatcher(pool),
	}
}

// WSStructServer is a WSServer able to handle WSStructSpeaker.
type WSStructServer struct {
	*WSServer
	*wsStructSpeakerPoolEventDispatcher
}

// Request sends a Request Struct message to the WSSpeaker of the given host.
func (s *WSStructServer) Request(host string, request *WSStructMessage, timeout time.Duration) (*WSStructMessage, error) {
	c := s.WSServer.GetSpeakerByHost(host)
	if c == nil {
		return nil, common.ErrNotFound
	}

	return c.(*WSStructSpeaker).Request(request, timeout)
}

// OnMessage websocket event.
func (s *WSStructServer) OnMessage(c WSSpeaker, m WSMessage) {
}

// OnConnected websocket event.
func (s *WSStructServer) OnConnected(c WSSpeaker) {
}

// OnDisconnected removes the WSSpeaker from the incomer pool.
func (s *WSStructServer) OnDisconnected(c WSSpeaker) {
	s.WSServer.wsIncomerPool.RemoveClient(c)
}

// NewWSStructServer returns a new WSStructServer
func NewWSStructServer(server *WSServer) *WSStructServer {
	s := &WSStructServer{
		WSServer: server,
		wsStructSpeakerPoolEventDispatcher: newWSStructSpeakerPoolEventDispatcher(server),
	}

	s.WSServer.wsIncomerPool.AddEventHandler(s)

	// This incomerHandler upgrades the incomers to WSStructSpeaker thus being able to parse StructMessage.
	// The server set also the WSJsonSpeaker with the proper namspaces it subscribes to thanks to the
	// headers.
	s.WSServer.incomerHandler = func(conn *websocket.Conn, r *auth.AuthenticatedRequest) WSSpeaker {
		// the default incomer handler creates a standard wsIncomerClient that we upgrade to a WSStructSpeaker
		// being able to handle the StructMessage
		c := defaultIncomerHandler(conn, r).upgradeToWSStructSpeaker()

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
			c.nsSubscribed[WildcardNamespace] = true
		}

		s.wsStructSpeakerPoolEventDispatcher.AddStructSpeaker(c)

		return c
	}

	return s
}
