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

import "sync"

// Interface to be implemented by master election listeners
type WSMasterEventHandler interface {
	OnNewMaster(c WSSpeaker)
}

// Defines a master election of a pool
type WSMasterElection struct {
	sync.RWMutex
	DefaultWSSpeakerEventHandler
	pool          WSSpeakerPool
	master        WSSpeaker
	eventHandlers []WSMasterEventHandler
}

func (a *WSMasterElection) selectMaster() {
	a.master = a.pool.GetConnectedClient()
	return
}

// Send a message to the master
func (a *WSMasterElection) SendMessageToMaster(m WSMessage) {
	a.RLock()
	if a.master != nil {
		defer a.master.Send(m)
	}
	a.RUnlock()
}

// OnConnected event
func (a *WSMasterElection) OnConnected(c WSSpeaker) {
	a.Lock()
	if a.master == nil {
		master := c.(*WSClient)
		a.master = master
		defer a.notifyNewMaster(master)
	}
	a.Unlock()
}

// OnDisconnected event
func (a *WSMasterElection) OnDisconnected(c WSSpeaker) {
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
func (a *WSMasterElection) notifyNewMaster(c WSSpeaker) {
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
func NewWSMasterElection(pool WSSpeakerPool) *WSMasterElection {
	me := &WSMasterElection{pool: pool}
	pool.AddEventHandler(me)
	return me
}
