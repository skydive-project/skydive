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

package websocket

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

	// ProtobufProtocol is used for protobuf encoded messages
	ProtobufProtocol = "protobuf"
	// JSONProtocol is used for JSON encoded messages
	JSONProtocol = "json"
)

// DefaultRequestTimeout default timeout used for Request/Reply JSON message.
var DefaultRequestTimeout = 10 * time.Second

// StructMessageJSON defines a JSON serialized object
type StructMessageJSON struct {
	Namespace string
	Type      string
	UUID      string
	Status    int64
	Obj       *json.RawMessage
}

// StructMessage defines a basic structured message
type StructMessage struct {
	Protocol  string
	Namespace string
	Type      string
	UUID      string
	Status    int64
	value     interface{}

	// JSONObj embeds an other JSON object in the message
	JSONObj            *json.RawMessage
	jsonSerialized     []byte
	ProtobufObj        []byte
	protobufSerialized []byte
}

// Debug representation of the struct StructMessage
func (g *StructMessage) Debug() string {
	if g.Protocol == JSONProtocol {
		return fmt.Sprintf("Namespace %s Type %s UUID %s Status %d Obj JSON (%d) : %q",
			g.Namespace, g.Type, g.UUID, g.Status, len(*g.JSONObj), string(*g.JSONObj))
	}
	return fmt.Sprintf("Namespace %s Type %s UUID %s Status %d Obj Protobuf (%d bytes)",
		g.Namespace, g.Type, g.UUID, g.Status, len(g.ProtobufObj))
}

// Marshal serializes the StructMessage into a JSON string.
func (g *StructMessageJSON) Marshal() ([]byte, error) {
	return json.Marshal(g)
}

// Bytes see Marshal
func (g StructMessage) Bytes(protocol string) []byte {
	g.Protocol = protocol
	if g.Protocol == ProtobufProtocol {
		if len(g.protobufSerialized) > 0 {
			return g.protobufSerialized
		}
		g.marshalObj()
		msgProto := &StructMessageProtobuf{
			Namespace: g.Namespace,
			Type:      g.Type,
			UUID:      g.UUID,
			Status:    g.Status,
			Obj:       g.ProtobufObj,
		}
		g.protobufSerialized, _ = msgProto.Marshal()
		return g.protobufSerialized
	}

	if len(g.jsonSerialized) > 0 {
		return g.jsonSerialized
	}
	g.marshalObj()
	msgJSON := &StructMessageJSON{
		Namespace: g.Namespace,
		Type:      g.Type,
		UUID:      g.UUID,
		Status:    g.Status,
		Obj:       g.JSONObj,
	}
	g.jsonSerialized, _ = msgJSON.Marshal()
	return g.jsonSerialized
}

func (g *StructMessage) marshalObj() {
	if g.Protocol == JSONProtocol {
		b, err := json.Marshal(g.value)
		if err != nil {
			logging.GetLogger().Error("Json Marshal encode value failed", err)
			return
		}
		raw := json.RawMessage(b)
		g.JSONObj = &raw
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

// DecodeObj decodes an object either as protobuf or as JSON
func (g *StructMessage) DecodeObj(obj interface{}) error {
	if g.Protocol == JSONProtocol {
		if err := common.JSONDecode(bytes.NewReader([]byte(*g.JSONObj)), obj); err != nil {
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

// UnmarshalObj unmarshals an object from JSON or protobuf
func (g *StructMessage) UnmarshalObj(obj interface{}) error {
	if g.Protocol == JSONProtocol {
		if err := json.Unmarshal(*g.JSONObj, obj); err != nil {
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

// NewStructMessage creates a new StructMessage with the given namespace, type, value
// and optionally the UUID.
func NewStructMessage(ns string, tp string, v interface{}, uuids ...string) *StructMessage {
	var u string
	if len(uuids) != 0 {
		u = uuids[0]
	} else {
		v4, _ := uuid.NewV4()
		u = v4.String()
	}

	msg := &StructMessage{
		Namespace: ns,
		Type:      tp,
		UUID:      u,
		Status:    int64(http.StatusOK),
		value:     v,
	}
	return msg
}

// Reply returns a reply message with the given value, type and status.
// Basically it return a new StructMessage with the correct Namespace and UUID.
func (g *StructMessage) Reply(v interface{}, kind string, status int) *StructMessage {
	msg := &StructMessage{
		Namespace: g.Namespace,
		Type:      kind,
		UUID:      g.UUID,
		Status:    int64(status),
		value:     v,
	}
	return msg
}

// SpeakerStructMessageDispatcher interface is used to dispatch OnStructMessage events.
type SpeakerStructMessageDispatcher interface {
	AddStructMessageHandler(h SpeakerStructMessageHandler, namespaces []string)
}

type structSpeakerEventDispatcher struct {
	eventHandlersLock common.RWMutex
	nsEventHandlers   map[string][]SpeakerStructMessageHandler
}

func newStructSpeakerEventDispatcher() *structSpeakerEventDispatcher {
	return &structSpeakerEventDispatcher{
		nsEventHandlers: make(map[string][]SpeakerStructMessageHandler),
	}
}

// AddStructMessageHandler adds a new listener for Struct messages.
func (a *structSpeakerEventDispatcher) AddStructMessageHandler(h SpeakerStructMessageHandler, namespaces []string) {
	a.eventHandlersLock.Lock()
	// add this handler per namespace
	for _, ns := range namespaces {
		if _, ok := a.nsEventHandlers[ns]; !ok {
			a.nsEventHandlers[ns] = []SpeakerStructMessageHandler{h}
		} else {
			a.nsEventHandlers[ns] = append(a.nsEventHandlers[ns], h)
		}
	}
	a.eventHandlersLock.Unlock()
}

func (a *structSpeakerEventDispatcher) dispatchMessage(c *StructSpeaker, m *StructMessage) {
	// check whether it is a reply
	if c.onReply(m) {
		return
	}

	a.eventHandlersLock.RLock()
	for _, l := range a.nsEventHandlers[m.Namespace] {
		l.OnStructMessage(c, m)
	}
	for _, l := range a.nsEventHandlers[WildcardNamespace] {
		l.OnStructMessage(c, m)
	}
	a.eventHandlersLock.RUnlock()
}

// OnDisconnected is implemented here to avoid infinite loop since the default
// implemtation is triggering OnDisconnected too.
func (a *structSpeakerEventDispatcher) OnDisconnected(c Speaker) {
}

// OnConnected is implemented here to avoid infinite loop since the default
// implemtation is triggering OnDisconnected too.
func (a *structSpeakerEventDispatcher) OnConnected(c Speaker) {
}

type structSpeakerPoolEventDispatcher struct {
	dispatcher *structSpeakerEventDispatcher
	pool       SpeakerPool
}

// AddStructMessageHandler adds a new listener for Struct messages.
func (d *structSpeakerPoolEventDispatcher) AddStructMessageHandler(h SpeakerStructMessageHandler, namespaces []string) {
	d.dispatcher.AddStructMessageHandler(h, namespaces)
	for _, client := range d.pool.GetSpeakers() {
		client.(*StructSpeaker).AddStructMessageHandler(h, namespaces)
	}
}

func (d *structSpeakerPoolEventDispatcher) AddStructSpeaker(c *StructSpeaker) {
	d.dispatcher.eventHandlersLock.RLock()
	for ns, handlers := range d.dispatcher.nsEventHandlers {
		for _, handler := range handlers {
			c.AddStructMessageHandler(handler, []string{ns})
		}
	}
	d.dispatcher.eventHandlersLock.RUnlock()
}

func newStructSpeakerPoolEventDispatcher(pool SpeakerPool) *structSpeakerPoolEventDispatcher {
	return &structSpeakerPoolEventDispatcher{
		dispatcher: newStructSpeakerEventDispatcher(),
		pool:       pool,
	}
}

// StructSpeaker is a Speaker able to handle Struct Message and Request/Reply calls.
type StructSpeaker struct {
	Speaker
	*structSpeakerEventDispatcher
	nsSubscribed   map[string]bool
	replyChanMutex common.RWMutex
	replyChan      map[string]chan *StructMessage
}

// Send sends a message according to the namespace.
func (s *StructSpeaker) Send(m Message) {
	if msg, ok := m.(StructMessage); ok {
		if _, ok := s.nsSubscribed[msg.Namespace]; !ok {
			if _, ok := s.nsSubscribed[WildcardNamespace]; !ok {
				return
			}
		}
	}

	s.Speaker.SendMessage(m)
}

func (s *StructSpeaker) onReply(m *StructMessage) bool {
	s.replyChanMutex.RLock()
	ch, ok := s.replyChan[m.UUID]
	if ok {
		ch <- m
	}
	s.replyChanMutex.RUnlock()

	return ok
}

// Request sends a Struct message request waiting for a reply using the given timeout.
func (s *StructSpeaker) Request(m *StructMessage, timeout time.Duration) (*StructMessage, error) {
	ch := make(chan *StructMessage, 1)

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

// OnMessage checks that the Message comes from a StructSpeaker. It parses
// the Struct message and then dispatch the message to the proper listeners according
// to the namespace.
func (s *StructSpeaker) OnMessage(c Speaker, m Message) {
	if c, ok := c.(*StructSpeaker); ok {
		msg := StructMessage{}
		if c.GetClientProtocol() == ProtobufProtocol {
			mProtobuf := StructMessageProtobuf{}
			b := m.Bytes(ProtobufProtocol)
			if err := proto.Unmarshal(b, &mProtobuf); err != nil {
				logging.GetLogger().Errorf("Error while decoding Protobuf StructMessage %s\n%s", err.Error(), hex.Dump(b))
				return
			}
			msg.Protocol = ProtobufProtocol
			msg.Namespace = mProtobuf.Namespace
			msg.Type = mProtobuf.Type
			msg.UUID = mProtobuf.UUID
			msg.Status = mProtobuf.Status
			msg.ProtobufObj = mProtobuf.Obj
		} else {
			mJSON := StructMessageJSON{}
			b := m.Bytes(JSONProtocol)
			if err := json.Unmarshal(b, &mJSON); err != nil {
				logging.GetLogger().Errorf("Error while decoding JSON StructMessage %s\n%s", err.Error(), hex.Dump(b))
				return
			}
			msg.Protocol = JSONProtocol
			msg.Namespace = mJSON.Namespace
			msg.Type = mJSON.Type
			msg.UUID = mJSON.UUID
			msg.Status = mJSON.Status
			msg.JSONObj = mJSON.Obj
		}
		s.structSpeakerEventDispatcher.dispatchMessage(c, &msg)
	}
}

func newStructSpeaker(c Speaker) *StructSpeaker {
	s := &StructSpeaker{
		Speaker: c,
		structSpeakerEventDispatcher: newStructSpeakerEventDispatcher(),
		nsSubscribed:                 make(map[string]bool),
		replyChan:                    make(map[string]chan *StructMessage),
	}

	// subscribing to itself so that the StructSpeaker can get Message and can convert them
	// to StructMessage and then forward them to its own even listeners.
	s.AddEventHandler(s)
	return s
}

// UpgradeToStructSpeaker a WebSocket client to a StructSpeaker
func (c *Client) UpgradeToStructSpeaker() *StructSpeaker {
	s := newStructSpeaker(c)
	c.Lock()
	c.wsSpeaker = s
	c.Unlock()
	return s
}

func (c *wsIncomingClient) upgradeToStructSpeaker() *StructSpeaker {
	s := newStructSpeaker(c)
	c.Lock()
	c.wsSpeaker = s
	c.Unlock()
	return s
}

// StructSpeakerPool is the interface of a pool of StructSpeakers.
type StructSpeakerPool interface {
	SpeakerPool
	SpeakerStructMessageDispatcher
	Request(host string, request *StructMessage, timeout time.Duration) (*StructMessage, error)
}

// StructClientPool is a ClientPool able to send StructMessage.
type StructClientPool struct {
	*ClientPool
	*structSpeakerPoolEventDispatcher
}

// AddClient adds a Client to the pool.
func (s *StructClientPool) AddClient(c Speaker) error {
	if wc, ok := c.(*Client); ok {
		speaker := wc.UpgradeToStructSpeaker()
		s.ClientPool.AddClient(speaker)
		s.structSpeakerPoolEventDispatcher.AddStructSpeaker(speaker)
	} else {
		return errors.New("wrong client type")
	}
	return nil
}

// Request sends a Request Struct message to the Speaker of the given remote host.
func (s *StructClientPool) Request(host string, request *StructMessage, timeout time.Duration) (*StructMessage, error) {
	c := s.ClientPool.GetSpeakerByRemoteHost(host)
	if c == nil {
		return nil, common.ErrNotFound
	}

	return c.(*StructSpeaker).Request(request, timeout)
}

// NewStructClientPool returns a new StructClientPool.
func NewStructClientPool(name string) *StructClientPool {
	pool := NewClientPool(name)
	return &StructClientPool{
		ClientPool:                       pool,
		structSpeakerPoolEventDispatcher: newStructSpeakerPoolEventDispatcher(pool),
	}
}

// StructServer is a Server able to handle StructSpeaker.
type StructServer struct {
	*Server
	*structSpeakerPoolEventDispatcher
}

// Request sends a Request Struct message to the Speaker of the given remote host.
func (s *StructServer) Request(host string, request *StructMessage, timeout time.Duration) (*StructMessage, error) {
	c := s.Server.GetSpeakerByRemoteHost(host)
	if c == nil {
		return nil, common.ErrNotFound
	}

	return c.(*StructSpeaker).Request(request, timeout)
}

// OnMessage websocket event.
func (s *StructServer) OnMessage(c Speaker, m Message) {
}

// OnConnected websocket event.
func (s *StructServer) OnConnected(c Speaker) {
}

// OnDisconnected removes the Speaker from the incomer pool.
func (s *StructServer) OnDisconnected(c Speaker) {
	s.Server.incomerPool.RemoveClient(c)
}

// NewStructServer returns a new StructServer
func NewStructServer(server *Server) *StructServer {
	s := &StructServer{
		Server: server,
		structSpeakerPoolEventDispatcher: newStructSpeakerPoolEventDispatcher(server),
	}

	s.Server.incomerPool.AddEventHandler(s)

	// This incomerHandler upgrades the incomers to StructSpeaker thus being able to parse StructMessage.
	// The server set also the StructSpeaker with the proper namspaces it subscribes to thanks to the
	// headers.
	s.Server.incomerHandler = func(conn *websocket.Conn, r *auth.AuthenticatedRequest) Speaker {
		// the default incomer handler creates a standard wsIncomingClient that we upgrade to a StructSpeaker
		// being able to handle the StructMessage
		c := s.Server.newIncomingClient(conn, r).upgradeToStructSpeaker()

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

		s.structSpeakerPoolEventDispatcher.AddStructSpeaker(c)

		return c
	}

	return s
}
