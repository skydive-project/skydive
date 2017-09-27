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
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
)

type WSSpeakerPool interface {
	AddClient(c WSSpeaker) error
	AddEventHandler(h WSSpeakerEventHandler)
	GetClients() []WSSpeaker
	GetConnectedClient() WSSpeaker
	BroadcastMessage(m WSMessage)
	SendMessageTo(m WSMessage, host string) error
}

type WSPool struct {
	sync.RWMutex
	quit            chan bool
	broadcast       chan WSMessage
	bulkMaxMsgs     int
	bulkMaxDelay    time.Duration
	eventBuffer     []WSMessage
	eventBufferLock sync.RWMutex
	wg              sync.WaitGroup
	running         atomic.Value
	eventHandlers   []WSSpeakerEventHandler
	clients         []WSSpeaker
}

// WSSpeakerPool represents a pool of WebSocket clients
type WSClientPool struct {
	*WSPool
}

// wsIncomerPool represents a pool of WebSocket conections
type wsIncomerPool struct {
	*WSPool
}

func (s *WSPool) OnConnected(c WSSpeaker) {
	s.RLock()
	for _, h := range s.eventHandlers {
		h.OnConnected(c)
	}
	s.RUnlock()
}

// OnDisconnected event
func (s *WSPool) OnDisconnected(c WSSpeaker) {
	s.RLock()
	for _, h := range s.eventHandlers {
		h.OnDisconnected(c)
	}
	s.RUnlock()
}

// OnDisconnected event
func (s *wsIncomerPool) OnDisconnected(c WSSpeaker) {
	s.WSPool.OnDisconnected(c)

	s.Lock()
	s.removeClient(c)
	s.Unlock()
}

// AddClient a new client
func (s *WSPool) AddClient(c WSSpeaker) error {
	s.Lock()
	s.clients = append(s.clients, c)
	s.Unlock()

	// This is to call WSSpeakerPool.On{Message,Disconnected}
	c.AddEventHandler(s)

	return nil
}

// OnMessage event
func (s *WSPool) OnMessage(c WSSpeaker, m WSMessage) {
	s.RLock()
	for _, h := range s.eventHandlers {
		h.OnMessage(c, m)
	}
	s.RUnlock()
}

func (s *WSPool) removeClient(c WSSpeaker) {
	for i, ic := range s.clients {
		if ic.GetHost() == c.GetHost() {
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			return
		}
	}
}

// GetClients returns all the clients
func (s *WSPool) GetClients() (clients []WSSpeaker) {
	s.RLock()
	clients = append(clients, s.clients...)
	s.RUnlock()
	return
}

func (s *WSPool) GetConnectedClient() WSSpeaker {
	s.RLock()
	defer s.RUnlock()

	length := len(s.clients)
	if length == 0 {
		return nil
	}

	index := rand.Intn(length)
	for i := 0; i != length; i++ {
		if c := s.clients[index]; c != nil && c.IsConnected() {
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

// DisconnectAll disconnect all clients
func (s *WSPool) DisconnectAll() {
	s.Lock()
	s.eventHandlers = s.eventHandlers[:0]
	s.Unlock()

	s.RLock()
	for _, c := range s.clients {
		c.Disconnect()
	}
	s.RUnlock()
}

func (s *WSPool) broadcastMessage(m WSMessage) {
	s.RLock()
	defer s.RUnlock()

	for _, c := range s.clients {
		c.Send(m)
	}
}

// GetClientsByType returns the clients with the specified type
func (s *WSPool) GetClientsByType(clientType common.ServiceType) (clients []WSSpeaker) {
	s.RLock()
	for _, c := range s.clients {
		if c.GetClientType() == clientType {
			clients = append(clients, c)
		}
	}
	s.RUnlock()
	return
}

// GetClient returns client for a given host
func (s *WSPool) GetClientByHost(host string) WSSpeaker {
	for _, c := range s.clients {
		if c.GetHost() == host {
			return c
		}
	}
	return nil
}

// SendMessageTo sends message to specific host
func (s *WSPool) SendMessageTo(m WSMessage, host string) error {
	c := s.GetClientByHost(host)
	if c == nil {
		return common.ErrNotFound
	}

	c.Send(m)
	return nil
}

// BroadcastMessage broadcasts the given message
func (s *WSPool) BroadcastMessage(m WSMessage) {
	s.broadcast <- m
}

func (s *WSPool) flushMessages() (ms []WSMessage) {
	ms = make([]WSMessage, len(s.eventBuffer))
	copy(ms, s.eventBuffer)
	s.eventBuffer = s.eventBuffer[:0]
	return
}

// QueueBroadcastMessage enqueues a broadcast message
func (s *WSPool) QueueBroadcastMessage(m WSMessage) {
	s.eventBufferLock.Lock()
	s.eventBuffer = append(s.eventBuffer, m)
	if len(s.eventBuffer) == s.bulkMaxMsgs {
		ms := s.flushMessages()
		defer s.broadcastMessages(ms)
	}
	s.eventBufferLock.Unlock()
}

func (s *WSPool) broadcastMessages(ms []WSMessage) {
	for _, m := range ms {
		s.BroadcastMessage(m)
	}
}

// AddEventHandler registers a new event handler
func (s *WSPool) AddEventHandler(h WSSpeakerEventHandler) {
	s.Lock()
	s.eventHandlers = append(s.eventHandlers, h)
	s.Unlock()
}

// Run starts the pool
func (s *WSPool) Run() {
	s.wg.Add(1)
	defer s.wg.Done()

	s.running.Store(true)

	bulkTicker := time.NewTicker(s.bulkMaxDelay)
	defer bulkTicker.Stop()

	for {
		select {
		case <-s.quit:
			s.DisconnectAll()
			return
		case m := <-s.broadcast:
			s.broadcastMessage(m)
		case <-bulkTicker.C:
			s.eventBufferLock.Lock()
			ms := s.flushMessages()
			s.eventBufferLock.Unlock()
			if len(ms) > 0 {
				s.broadcastMessages(ms)
			}
		}
	}
}

// Start starts the pool in a goroutine
func (s *WSPool) Start() {
	go s.Run()
}

// Stop stops the pool and wait until stopped
func (s *WSPool) Stop() {
	s.quit <- true
	if s.running.Load() == true {
		s.wg.Wait()
	}
	s.running.Store(false)
}

func (s *WSClientPool) ConnectAll() {
	s.RLock()
	// shuffle connections to avoid election of the same client as master
	indexes := rand.Perm(len(s.clients))
	for _, i := range indexes {
		s.clients[i].Connect()
	}
	s.RUnlock()
}

// NewWSSpeakerPool a new pool of WebSocket clients
func newWSPool() *WSPool {
	bulkMaxMsgs := config.GetConfig().GetInt("ws_bulk_maxmsgs")
	bulkMaxDelay := config.GetConfig().GetInt("ws_bulk_maxdelay")

	return &WSPool{
		broadcast:    make(chan WSMessage, 100000),
		quit:         make(chan bool, 2),
		bulkMaxMsgs:  bulkMaxMsgs,
		bulkMaxDelay: time.Duration(bulkMaxDelay) * time.Second,
	}
}

func newWSIncomerPool() *wsIncomerPool {
	return &wsIncomerPool{
		WSPool: newWSPool(),
	}
}

// NewWSSpeakerPool a new pool of WebSocket clients
func NewWSClientPool() *WSClientPool {
	return &WSClientPool{
		WSPool: newWSPool(),
	}
}
