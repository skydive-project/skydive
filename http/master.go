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
)

// Interface to be implemented by master election listeners
type WSMasterEventHandler interface {
	OnNewMaster(c WSClient)
}

// Defines a master election of a pool
type WSMasterElection struct {
	sync.RWMutex
	DefaultWSClientEventHandler
	pool          *WSClientPool
	master        WSClient
	eventHandlers []WSMasterEventHandler
}

func (a *WSMasterElection) selectMaster() WSClient {
	a.master = nil

	a.pool.RLock()
	defer a.pool.RUnlock()

	length := len(a.pool.clients)
	if length == 0 {
		return nil
	}

	index := rand.Intn(length)
	for i := 0; i != length; i++ {
		if client := a.pool.clients[index]; client != nil && client.IsConnected() {
			a.master = client
			break
		}

		if index+1 >= length {
			index = 0
		} else {
			index++
		}
	}

	return a.master
}

// Send a message to the master
func (a *WSMasterElection) SendMessageToMaster(m Message) {
	a.RLock()
	if a.master != nil {
		defer a.master.Send(m)
	}
	a.RUnlock()
}

// OnConnected event
func (a *WSMasterElection) OnConnected(c WSClient) {
	a.Lock()
	if a.master == nil {
		a.master = c
		defer a.notifyNewMaster(c)
	}
	a.Unlock()
}

// OnDisconnected event
func (a *WSMasterElection) OnDisconnected(c WSClient) {
	a.Lock()
	if a.master != nil && a.master.GetHost() == c.GetHost() {
		a.selectMaster()
		defer func() {
			a.notifyNewMaster(a.master)
		}()
	}
	a.Unlock()
}

// Notify all the listeners that a new master has been elected
func (a *WSMasterElection) notifyNewMaster(c WSClient) {
	a.RLock()
	for _, h := range a.eventHandlers {
		defer h.OnNewMaster(c)
	}
	a.RUnlock()
}

// Register a new event handler
func (a *WSMasterElection) AddEventHandler(eventHandler WSMasterEventHandler) {
	a.Lock()
	a.eventHandlers = append(a.eventHandlers, eventHandler)
	a.Unlock()
}

// Returns a new master election
func NewWSMasterElection(pool *WSClientPool) *WSMasterElection {
	me := &WSMasterElection{pool: pool}
	pool.AddEventHandler(me)
	return me
}
