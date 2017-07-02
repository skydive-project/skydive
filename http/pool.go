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

// WSClientPool represents a pool of WebSocket clients
type WSClientPool struct {
	sync.RWMutex
	quit              chan bool
	broadcast         chan Message
	clients           []WSClient
	eventHandlers     []WSClientEventHandler
	eventHandlersLock sync.RWMutex
	bulkMaxMsgs       int
	bulkMaxDelay      time.Duration
	eventBuffer       []Message
	eventBufferLock   sync.RWMutex
	wg                sync.WaitGroup
	running           atomic.Value
}

// OnConnected event
func (s *WSClientPool) OnConnected(c WSClient) {
	s.RLock()
	for _, h := range s.eventHandlers {
		h.OnConnected(c)
	}
	s.RUnlock()
}

// OnDisconnected event
func (s *WSClientPool) OnDisconnected(c WSClient) {
	s.RLock()
	for _, h := range s.eventHandlers {
		h.OnDisconnected(c)
	}
	s.RUnlock()

	s.Lock()
	s.RemoveClient(c)
	s.Unlock()
}

// OnMessage event
func (s *WSClientPool) OnMessage(c WSClient, m Message) {
	s.RLock()
	for _, h := range s.eventHandlers {
		h.OnMessage(c, m)
	}
	s.RUnlock()
}

// Register a new client
func (a *WSClientPool) AddClient(client WSClient) {
	a.Lock()
	a.clients = append(a.clients, client)
	a.Unlock()

	// This is to call WSClientPool.On{Message,Connected,Disconnected}
	client.AddEventHandler(a)
}

// Unregister a client
func (a *WSClientPool) RemoveClient(client WSClient) {
	for i, c := range a.clients {
		if c.GetHost() == client.GetHost() {
			a.clients = append(a.clients[:i], a.clients[i+1:]...)
			return
		}
	}
}

// Connect all clients
func (a *WSClientPool) ConnectAll() {
	a.RLock()
	// shuffle connections to avoid election of the same client as master
	indexes := rand.Perm(len(a.clients))
	for _, i := range indexes {
		a.clients[i].Connect()
	}
	a.RUnlock()
}

// Disconnect all clients
func (a *WSClientPool) DisconnectAll() {
	a.eventHandlersLock.Lock()
	a.eventHandlers = a.eventHandlers[:0]
	a.eventHandlersLock.Unlock()

	a.RLock()
	for _, client := range a.clients {
		client.Disconnect()
	}
	a.RUnlock()
}

func (a *WSClientPool) broadcastMessage(m Message) {
	a.RLock()
	defer a.RUnlock()

	for _, c := range a.clients {
		c.Send(m)
	}
}

// Return the clients with the specified type
func (s *WSClientPool) GetClientsByType(clientType common.ServiceType) (clients []WSClient) {
	s.RLock()
	for _, client := range s.clients {
		if client.GetClientType() == clientType {
			clients = append(clients, client)
		}
	}
	s.RUnlock()
	return clients
}

func (a *WSClientPool) GetClient(host string) WSClient {
	for _, c := range a.clients {
		if c.GetHost() == host {
			return c
		}
	}
	return nil
}

func (a *WSClientPool) SendMessageTo(msg Message, host string) error {
	client := a.GetClient(host)
	if client == nil {
		return common.ErrNotFound
	}

	client.Send(msg)
	return nil
}

func (a *WSClientPool) BroadcastMessage(msg Message) {
	a.broadcast <- msg
}

func (a *WSClientPool) flushMessages() (msgs []Message) {
	msgs = make([]Message, len(a.eventBuffer))
	copy(msgs, a.eventBuffer)
	a.eventBuffer = a.eventBuffer[:0]
	return
}

func (a *WSClientPool) QueueBroadcastMessage(msg Message) {
	a.eventBufferLock.Lock()
	a.eventBuffer = append(a.eventBuffer, msg)
	if len(a.eventBuffer) == a.bulkMaxMsgs {
		msgs := a.flushMessages()
		defer a.broadcastMessages(msgs)
	}
	a.eventBufferLock.Unlock()
}

func (a *WSClientPool) broadcastMessages(msgs []Message) {
	for _, msg := range msgs {
		a.BroadcastMessage(msg)
	}
}

// Register a new event handler
func (a *WSClientPool) AddEventHandler(h WSClientEventHandler) {
	a.eventHandlersLock.Lock()
	a.eventHandlers = append(a.eventHandlers, h)
	a.eventHandlersLock.Unlock()
}

func (a *WSClientPool) Run() {
	a.wg.Add(1)
	defer a.wg.Done()

	a.running.Store(true)

	bulkTicker := time.NewTicker(a.bulkMaxDelay)
	defer bulkTicker.Stop()

	i := 1
	for {
		select {
		case <-a.quit:
			a.DisconnectAll()
			return
		case m := <-a.broadcast:
			a.broadcastMessage(m)
		case <-bulkTicker.C:
			a.eventBufferLock.Lock()
			msgs := a.flushMessages()
			a.eventBufferLock.Unlock()
			i++
			if len(msgs) > 0 {
				a.broadcastMessages(msgs)
			}
		}
	}
}

func (a *WSClientPool) Start() {
	go a.Run()
}

func (a *WSClientPool) Stop() {
	a.quit <- true
	if a.running.Load() == true {
		a.wg.Wait()
	}
	a.running.Store(false)
}

// NewWSClientPool a new pool of WebSocket clients
func NewWSClientPool() *WSClientPool {
	bulkMaxMsgs := config.GetConfig().GetInt("ws_bulk_maxmsgs")
	bulkMaxDelay := config.GetConfig().GetInt("ws_bulk_maxdelay")

	return &WSClientPool{
		broadcast:    make(chan Message, 100000),
		quit:         make(chan bool, 2),
		bulkMaxMsgs:  bulkMaxMsgs,
		bulkMaxDelay: time.Duration(bulkMaxDelay) * time.Second,
	}
}
