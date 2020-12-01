/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"encoding/json"
	"errors"
	fmt "fmt"
	"net/http"
	"time"

	auth "github.com/abbot/go-http-auth"
	proto "github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/safchain/insanelock"

	"github.com/skydive-project/skydive/graffiti/logging"
)

// Protocol used to transport messages
type Protocol string

const (
	// WildcardNamespace is the namespace used as wildcard. It is used by listeners to filter callbacks.
	WildcardNamespace = "*"

	// RawProtocol is used for raw messages
	RawProtocol Protocol = "raw"
	// ProtobufProtocol is used for protobuf encoded messages
	ProtobufProtocol Protocol = "protobuf"
	// JSONProtocol is used for JSON encoded messages
	JSONProtocol Protocol = "json"
)

// DefaultRequestTimeout default timeout used for Request/Reply JSON message.
var DefaultRequestTimeout = 10 * time.Second

// ProtobufObject defines an object that can be serialized in protobuf
type ProtobufObject interface {
	proto.Marshaler
	proto.Message
}

type structMessageState struct {
	value interface{}

	// keep a cached version to avoid to serialize multiple time during
	// the call flow
	jsonCache     []byte
	protobufCache []byte
}

func (p *Protocol) parse(s string) error {
	switch s {
	case "", "json":
		*p = JSONProtocol
	case "protobuf":
		*p = ProtobufProtocol
	default:
		return errors.New("protocol not supported")
	}
	return nil
}

func (p *Protocol) String() string {
	return string(*p)
}

// Debug representation of the struct StructMessage
func (g *StructMessage) Debug() string {
	return fmt.Sprintf("Namespace %s Type %s UUID %s Status %d Obj (%d bytes)",
		g.Namespace, g.Type, g.UUID, g.Status, len(g.Obj))
}

func (g *StructMessage) protobufBytes() ([]byte, error) {
	if len(g.XXX_state.protobufCache) > 0 {
		return g.XXX_state.protobufCache, nil
	}

	obj, ok := g.XXX_state.value.(ProtobufObject)
	if ok {
		g.Format = Format_Protobuf

		msgObj, err := obj.Marshal()
		if err != nil {
			return nil, err
		}
		g.Obj = msgObj
	} else {
		// fallback to json
		g.Format = Format_Json

		msgObj, err := json.Marshal(g.XXX_state.value)
		if err != nil {
			return nil, err
		}
		g.Obj = msgObj
	}

	msgBytes, err := g.Marshal()
	if err != nil {
		return nil, err
	}

	g.XXX_state.protobufCache = msgBytes

	return msgBytes, nil
}

func (g *StructMessage) jsonBytes() ([]byte, error) {
	if len(g.XXX_state.jsonCache) > 0 {
		return g.XXX_state.jsonCache, nil
	}

	m := struct {
		*StructMessage
		Obj interface{}
	}{
		StructMessage: g,
		Obj:           g.XXX_state.value,
	}

	msgBytes, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	g.XXX_state.jsonCache = msgBytes

	return msgBytes, nil
}

// Bytes implements the message interface
func (g *StructMessage) Bytes(protocol Protocol) ([]byte, error) {
	if protocol == ProtobufProtocol {
		return g.protobufBytes()
	}
	return g.jsonBytes()
}

// UnmarshalJSON custom unmarshal
func (g *StructMessage) UnmarshalJSON(b []byte) error {
	m := struct {
		Namespace string
		Type      string
		UUID      string
		Status    int64
		Obj       json.RawMessage
	}{}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	g.Namespace = m.Namespace
	g.Type = m.Type
	g.UUID = m.UUID
	g.Status = m.Status
	g.Obj = []byte(m.Obj)

	return nil
}

func (g *StructMessage) unmarshalByProtocol(b []byte, protocol Protocol) error {
	if protocol == ProtobufProtocol {
		if err := g.Unmarshal(b); err != nil {
			return fmt.Errorf("Error while decoding Protobuf StructMessage %s", err)
		}
	} else {
		if err := g.UnmarshalJSON(b); err != nil {
			return fmt.Errorf("Error while decoding JSON StructMessage %s", err)
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
	}
	msg.XXX_state.value = v

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
	}
	msg.XXX_state.value = v

	return msg
}

// SpeakerStructMessageDispatcher interface is used to dispatch OnStructMessage events.
type SpeakerStructMessageDispatcher interface {
	AddStructMessageHandler(h SpeakerStructMessageHandler, namespaces []string)
}

type structSpeakerEventDispatcher struct {
	eventHandlersLock insanelock.RWMutex
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
	var handlers []SpeakerStructMessageHandler
	handlers = append(handlers, a.nsEventHandlers[m.Namespace]...)
	handlers = append(handlers, a.nsEventHandlers[WildcardNamespace]...)
	a.eventHandlersLock.RUnlock()

	for _, h := range handlers {
		h.OnStructMessage(c, m)
	}
}

// OnDisconnected is implemented here to avoid infinite loop since the default
// implementation is triggering OnDisconnected too.
func (a *structSpeakerEventDispatcher) OnDisconnected(c Speaker) {
}

// OnConnected is implemented here to avoid infinite loop since the default
// implementation is triggering OnDisconnected too.
func (a *structSpeakerEventDispatcher) OnConnected(c Speaker) error {
	return nil
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
	replyChanMutex insanelock.RWMutex
	replyChan      map[string]chan *StructMessage
	logger         logging.Logger
}

// SendMessage sends a message according to the namespace.
func (s *StructSpeaker) SendMessage(m Message) error {
	if msg, ok := m.(*StructMessage); ok {
		if _, ok := s.nsSubscribed[msg.Namespace]; !ok {
			if _, ok := s.nsSubscribed[WildcardNamespace]; !ok {
				return nil
			}
		}
	}

	return s.Speaker.SendMessage(m)
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

	s.SendMessage(m)

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		return nil, errors.New("Timeout")
	}
}

// OnMessage checks that the Message comes from a StructSpeaker. It parses
// the Struct message and then dispatch the message to the proper listeners according
// to the namespace.
func (s *StructSpeaker) OnMessage(c Speaker, m Message) {
	if c, ok := c.(*StructSpeaker); ok {
		// m is a rawmessage at this point
		bytes, _ := m.Bytes(RawProtocol)

		var structMsg StructMessage
		if err := structMsg.unmarshalByProtocol(bytes, c.GetClientProtocol()); err != nil {
			s.logger.Error(err)
			return
		}
		s.structSpeakerEventDispatcher.dispatchMessage(c, &structMsg)
	}
}

func newStructSpeaker(c Speaker, logger logging.Logger) *StructSpeaker {
	s := &StructSpeaker{
		Speaker:                      c,
		structSpeakerEventDispatcher: newStructSpeakerEventDispatcher(),
		nsSubscribed:                 make(map[string]bool),
		replyChan:                    make(map[string]chan *StructMessage),
		logger:                       logger,
	}

	// subscribing to itself so that the StructSpeaker can get Message and can convert them
	// to StructMessage and then forward them to its own even listeners.
	s.AddEventHandler(s)
	return s
}

// UpgradeToStructSpeaker a WebSocket client to a StructSpeaker
func (c *Client) UpgradeToStructSpeaker() *StructSpeaker {
	s := newStructSpeaker(c, c.logger)
	c.Lock()
	c.wsSpeaker = s
	s.nsSubscribed[WildcardNamespace] = true
	c.Unlock()
	return s
}

func (c *wsIncomingClient) upgradeToStructSpeaker() *StructSpeaker {
	s := newStructSpeaker(c, c.logger)
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
	c, err := s.ClientPool.GetSpeakerByRemoteHost(host)
	if err != nil {
		return nil, err
	}

	return c.(*StructSpeaker).Request(request, timeout)
}

// NewStructClientPool returns a new StructClientPool.
func NewStructClientPool(name string, opts PoolOpts) *StructClientPool {
	pool := NewClientPool(name, opts)
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
	c, err := s.Server.GetSpeakerByRemoteHost(host)
	if err != nil {
		return nil, err
	}

	return c.(*StructSpeaker).Request(request, timeout)
}

// OnMessage websocket event.
func (s *StructServer) OnMessage(c Speaker, m Message) {
}

// OnConnected websocket event.
func (s *StructServer) OnConnected(c Speaker) error {
	return nil
}

// OnDisconnected removes the Speaker from the incomer pool.
func (s *StructServer) OnDisconnected(c Speaker) {
}

// NewStructServer returns a new StructServer
func NewStructServer(server *Server) *StructServer {
	s := &StructServer{
		Server:                           server,
		structSpeakerPoolEventDispatcher: newStructSpeakerPoolEventDispatcher(server),
	}

	s.Server.incomerPool.AddEventHandler(s)

	// This incomerHandler upgrades the incomers to StructSpeaker thus being able to parse StructMessage.
	// The server set also the StructSpeaker with the proper namspaces it subscribes to thanks to the
	// headers.
	s.Server.incomerHandler = func(conn *websocket.Conn, r *auth.AuthenticatedRequest, promoter clientPromoter) (Speaker, error) {
		// the default incomer handler creates a standard wsIncomingClient that we upgrade to a StructSpeaker
		// being able to handle the StructMessage
		uc, err := s.Server.newIncomingClient(conn, r, func(ic *wsIncomingClient) (Speaker, error) {
			c := ic.upgradeToStructSpeaker()

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

			// if empty use wildcard for backward compatibility
			if len(c.nsSubscribed) == 0 {
				c.nsSubscribed[WildcardNamespace] = true
			}

			s.structSpeakerPoolEventDispatcher.AddStructSpeaker(c)

			return c, nil
		})
		if err != nil {
			return nil, err
		}

		return uc, nil
	}

	return s
}
