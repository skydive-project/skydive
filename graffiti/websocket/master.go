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

import "github.com/safchain/insanelock"

// MasterEventHandler is the interface to be implemented by master election listeners.
type MasterEventHandler interface {
	OnNewMaster(c Speaker)
}

// MasterElection provides a mechanism based on etcd to elect a master from a
// SpeakerPool.
type MasterElection struct {
	insanelock.RWMutex
	DefaultSpeakerEventHandler
	pool          SpeakerPool
	master        Speaker
	eventHandlers []MasterEventHandler
}

func (a *MasterElection) selectMaster() {
	a.master = a.pool.PickConnectedSpeaker()
	return
}

// GetMaster returns the current master.
func (a *MasterElection) GetMaster() Speaker {
	a.RLock()
	defer a.RUnlock()

	return a.master
}

// SendMessageToMaster sends a message to the master.
func (a *MasterElection) SendMessageToMaster(m Message) {
	a.RLock()
	if a.master != nil {
		defer a.master.SendMessage(m)
	}
	a.RUnlock()
}

// OnConnected is triggered when a new Speaker get connected. If no master
// was elected this Speaker will be chosen as master.
func (a *MasterElection) OnConnected(c Speaker) error {
	a.Lock()
	if a.master == nil {
		master := c.(*Client)
		a.master = master
		defer a.notifyNewMaster(master)
	}
	a.Unlock()
	return nil
}

// OnDisconnected is triggered when a new Speaker get disconnected. If it was
// the master a new election is triggered.
func (a *MasterElection) OnDisconnected(c Speaker) {
	a.Lock()
	if a.master != nil && a.master.GetRemoteHost() == c.GetRemoteHost() {
		a.selectMaster()
		defer func() {
			a.notifyNewMaster(a.master)
		}()
	}
	a.Unlock()
}

func (a *MasterElection) notifyNewMaster(c Speaker) {
	a.RLock()
	for _, h := range a.eventHandlers {
		defer h.OnNewMaster(c)
	}
	a.RUnlock()
}

// AddEventHandler a new MasterEventHandler event handler.
func (a *MasterElection) AddEventHandler(eventHandler MasterEventHandler) {
	a.Lock()
	a.eventHandlers = append(a.eventHandlers, eventHandler)
	a.Unlock()
}

// NewMasterElection returns a new MasterElection.
func NewMasterElection(pool SpeakerPool) *MasterElection {
	me := &MasterElection{pool: pool}
	pool.AddEventHandler(me)
	return me
}
