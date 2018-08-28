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
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

// WSSpeakerPool is the interface that WSSpeaker pools have to implement.
type WSSpeakerPool interface {
	AddClient(c WSSpeaker) error
	RemoveClient(c WSSpeaker) bool
	AddEventHandler(h WSSpeakerEventHandler)
	GetSpeakers() []WSSpeaker
	GetSpeakerByRemoteHost(host string) WSSpeaker
	PickConnectedSpeaker() WSSpeaker
	BroadcastMessage(m WSMessage)
	SendMessageTo(m WSMessage, host string) error
}

// WSPool is a connection container. It embed a list of WSSpeaker.
type WSPool struct {
	common.RWMutex
	name              string
	wg                sync.WaitGroup
	running           atomic.Value
	eventHandlers     []WSSpeakerEventHandler
	eventHandlersLock common.RWMutex
	speakers          []WSSpeaker
}

// WSClientPool is a pool of out going WSSpeaker meaning connection to a remote
// WSServer.
type WSClientPool struct {
	*WSPool
}

// wsIncomerPool is used to store incoming WSSpeaker meaning remote client connected
// to a local WSSpeaker.
type wsIncomerPool struct {
	*WSPool
}

// OnConnected forwards the OnConnected event to event listeners of the pool.
func (s *WSPool) OnConnected(c WSSpeaker) {
	s.eventHandlersLock.RLock()
	for _, h := range s.eventHandlers {
		h.OnConnected(c)
		if !c.IsConnected() {
			break
		}
	}
	s.eventHandlersLock.RUnlock()
}

// OnDisconnected forwards the OnConnected event to event listeners of the pool.
func (s *WSPool) OnDisconnected(c WSSpeaker) {
	logging.GetLogger().Debugf("OnDisconnected %s for pool %s ", c.GetRemoteHost(), s.GetName())
	s.eventHandlersLock.RLock()
	for _, h := range s.eventHandlers {
		h.OnDisconnected(c)
	}
	s.eventHandlersLock.RUnlock()
}

// OnDisconnected forwards the OnConnected event to event listeners of the pool.
func (s *wsIncomerPool) OnDisconnected(c WSSpeaker) {
	s.WSPool.OnDisconnected(c)

	s.RemoveClient(c)
}

// AddClient adds the given WSSpeaker to the pool.
func (s *WSPool) AddClient(c WSSpeaker) error {
	logging.GetLogger().Debugf("AddClient %s for pool %s", c.GetRemoteHost(), s.GetName())
	s.Lock()
	s.speakers = append(s.speakers, c)
	s.Unlock()

	// This is to call WSSpeakerPool.On{Message,Disconnected}
	c.AddEventHandler(s)

	return nil
}

// AddClient adds the given WSSpeaker to the wsIncomerPool.
func (s *wsIncomerPool) AddClient(c WSSpeaker) error {
	logging.GetLogger().Debugf("AddClient %s for pool %s", c.GetRemoteHost(), s.GetName())
	s.Lock()
	s.speakers = append(s.speakers, c)
	s.Unlock()

	// This is to call WSSpeakerPool.On{Message,Disconnected}
	c.AddEventHandler(s)

	return nil
}

// OnMessage forwards the OnMessage event to event listeners of the pool.
func (s *WSPool) OnMessage(c WSSpeaker, m WSMessage) {
	s.eventHandlersLock.RLock()
	for _, h := range s.eventHandlers {
		h.OnMessage(c, m)
	}
	s.eventHandlersLock.RUnlock()
}

// RemoveClient removes client from the pool
func (s *WSPool) RemoveClient(c WSSpeaker) bool {
	s.Lock()
	defer s.Unlock()

	host := c.GetRemoteHost()
	for i, ic := range s.speakers {
		if ic.GetRemoteHost() == host {
			logging.GetLogger().Debugf("Successfully removed client %s for pool %s", host, s.GetName())
			s.speakers = append(s.speakers[:i], s.speakers[i+1:]...)
			return true
		}
	}
	logging.GetLogger().Debugf("Failed to remove client %s for pool %s", host, s.GetName())

	return false
}

// GetStatus returns the states of the WebSocket clients
func (s *WSPool) GetStatus() map[string]ConnStatus {
	clients := make(map[string]ConnStatus)
	for _, client := range s.GetSpeakers() {
		clients[client.GetRemoteHost()] = client.GetStatus()
	}
	return clients
}

// GetName returns the name of the pool
func (s *WSPool) GetName() string {
	return s.name + " type : [" + (reflect.TypeOf(s).String()) + "]"
}

// GetSpeakers returns the WSSpeakers of the pool.
func (s *WSPool) GetSpeakers() (speakers []WSSpeaker) {
	s.RLock()
	speakers = append(speakers, s.speakers...)
	s.RUnlock()
	return
}

// PickConnectedSpeaker returns randomly a connected WSSpeaker
func (s *WSPool) PickConnectedSpeaker() WSSpeaker {
	s.RLock()
	defer s.RUnlock()

	length := len(s.speakers)
	if length == 0 {
		return nil
	}

	index := rand.Intn(length)
	for i := 0; i != length; i++ {
		if c := s.speakers[index]; c != nil && c.IsConnected() {
			return c
		}

		if index+1 >= length {
			index = 0
		} else {
			index++
		}
	}

	return nil
}

// DisconnectAll disconnects all the WSSpeaker
func (s *WSPool) DisconnectAll() {
	s.eventHandlersLock.Lock()
	s.eventHandlers = s.eventHandlers[:0]
	s.eventHandlersLock.Unlock()

	s.RLock()
	for _, c := range s.speakers {
		c.Disconnect()
	}
	s.RUnlock()
}

// GetSpeakersByType returns WSSpeakers matching the given type.
func (s *WSPool) GetSpeakersByType(serviceType common.ServiceType) (speakers []WSSpeaker) {
	s.RLock()
	for _, c := range s.speakers {
		if c.GetServiceType() == serviceType {
			speakers = append(speakers, c)
		}
	}
	s.RUnlock()
	return
}

// GetSpeakerByRemoteHost returns the WSSpeaker for the given remote host.
func (s *WSPool) GetSpeakerByRemoteHost(host string) WSSpeaker {
	for _, c := range s.speakers {
		if c.GetRemoteHost() == host {
			return c
		}
	}
	return nil
}

// SendMessageTo sends message to WSSpeaker for the given remote host.
func (s *WSPool) SendMessageTo(m WSMessage, host string) error {
	c := s.GetSpeakerByRemoteHost(host)
	if c == nil {
		return common.ErrNotFound
	}

	c.SendMessage(m)
	return nil
}

// BroadcastMessage broadcasts the given message.
func (s *WSPool) BroadcastMessage(m WSMessage) {
	s.RLock()
	defer s.RUnlock()

	for _, c := range s.speakers {
		r := m.Bytes(c.GetClientProtocol())
		if err := c.SendRaw(r); err != nil {
			logging.GetLogger().Errorf("Unable to send raw message: %s", err)
		}
	}
}

// AddEventHandler registers a new event handler.
func (s *WSPool) AddEventHandler(h WSSpeakerEventHandler) {
	s.eventHandlersLock.Lock()
	s.eventHandlers = append(s.eventHandlers, h)
	s.eventHandlersLock.Unlock()
}

// Start starts the pool in a goroutine.
func (s *WSPool) Start() {
}

// Stop stops the pool and wait until stopped.
func (s *WSPool) Stop() {
	s.DisconnectAll()
}

// ConnectAll calls connect to all the wSSpeakers of the pool.
func (s *WSClientPool) ConnectAll() {
	s.RLock()
	// shuffle connections to avoid election of the same client as master
	indexes := rand.Perm(len(s.speakers))
	for _, i := range indexes {
		s.speakers[i].Connect()
	}
	s.RUnlock()
}

func newWSPool(name string) *WSPool {
	return &WSPool{
		name: name,
	}
}

func newWSIncomerPool(name string) *wsIncomerPool {
	return &wsIncomerPool{
		WSPool: newWSPool(name),
	}
}

// NewWSClientPool returns a new WSClientPool meaning a pool of outgoing WSClient.
func NewWSClientPool(name string) *WSClientPool {
	s := &WSClientPool{
		WSPool: newWSPool(name),
	}

	s.Start()

	return s
}
