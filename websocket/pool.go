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
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

// SpeakerPool is the interface that Speaker pools have to implement.
type SpeakerPool interface {
	AddClient(c Speaker) error
	RemoveClient(c Speaker) bool
	AddEventHandler(h SpeakerEventHandler)
	GetSpeakers() []Speaker
	GetSpeakerByRemoteHost(host string) Speaker
	PickConnectedSpeaker() Speaker
	BroadcastMessage(m Message)
	SendMessageTo(m Message, host string) error
}

// PoolOpts defines pool options
type PoolOpts struct {
	Logger logging.Logger
}

// Pool is a connection container. It embed a list of Speaker.
type Pool struct {
	common.RWMutex
	name              string
	wg                sync.WaitGroup
	running           atomic.Value
	eventHandlers     []SpeakerEventHandler
	eventHandlersLock common.RWMutex
	speakers          []Speaker
	opts              PoolOpts
}

// ClientPool is a pool of out going Speaker meaning connection to a remote
// Server.
type ClientPool struct {
	*Pool
}

// incomerPool is used to store incoming Speaker meaning remote client connected
// to a local Speaker.
type incomerPool struct {
	*Pool
}

// OnConnected forwards the OnConnected event to event listeners of the pool.
func (s *Pool) OnConnected(c Speaker) {
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
func (s *Pool) OnDisconnected(c Speaker) {
	s.opts.Logger.Debugf("OnDisconnected %s for pool %s ", c.GetRemoteHost(), s.GetName())
	s.eventHandlersLock.RLock()
	for _, h := range s.eventHandlers {
		h.OnDisconnected(c)
	}
	s.eventHandlersLock.RUnlock()
}

// OnDisconnected forwards the OnConnected event to event listeners of the pool.
func (s *incomerPool) OnDisconnected(c Speaker) {
	s.Pool.OnDisconnected(c)

	s.RemoveClient(c)
}

// AddClient adds the given Speaker to the pool.
func (s *Pool) AddClient(c Speaker) error {
	s.opts.Logger.Debugf("AddClient for pool %s", s.GetName())
	s.Lock()
	s.speakers = append(s.speakers, c)
	s.Unlock()

	// This is to call SpeakerPool.On{Message,Disconnected}
	c.AddEventHandler(s)

	return nil
}

// AddClient adds the given Speaker to the incomerPool.
func (s *incomerPool) AddClient(c Speaker) error {
	s.opts.Logger.Debugf("AddClient %s for pool %s", c.GetRemoteHost(), s.GetName())
	s.Lock()
	s.speakers = append(s.speakers, c)
	s.Unlock()

	// This is to call SpeakerPool.On{Message,Disconnected}
	c.AddEventHandler(s)

	return nil
}

// OnMessage forwards the OnMessage event to event listeners of the pool.
func (s *Pool) OnMessage(c Speaker, m Message) {
	s.eventHandlersLock.RLock()
	for _, h := range s.eventHandlers {
		h.OnMessage(c, m)
	}
	s.eventHandlersLock.RUnlock()
}

// RemoveClient removes client from the pool
func (s *Pool) RemoveClient(c Speaker) bool {
	s.Lock()
	defer s.Unlock()

	host := c.GetRemoteHost()
	for i, ic := range s.speakers {
		if ic.GetRemoteHost() == host {
			s.opts.Logger.Debugf("Successfully removed client %s for pool %s", host, s.GetName())
			s.speakers = append(s.speakers[:i], s.speakers[i+1:]...)
			return true
		}
	}
	s.opts.Logger.Debugf("Failed to remove client %s for pool %s", host, s.GetName())

	return false
}

// GetStatus returns the states of the WebSocket clients
func (s *Pool) GetStatus() map[string]ConnStatus {
	clients := make(map[string]ConnStatus)
	for _, client := range s.GetSpeakers() {
		clients[client.GetRemoteHost()] = client.GetStatus()
	}
	return clients
}

// GetName returns the name of the pool
func (s *Pool) GetName() string {
	return s.name + " type : [" + (reflect.TypeOf(s).String()) + "]"
}

// GetSpeakers returns the Speakers of the pool.
func (s *Pool) GetSpeakers() (speakers []Speaker) {
	s.RLock()
	speakers = append(speakers, s.speakers...)
	s.RUnlock()
	return
}

// PickConnectedSpeaker returns randomly a connected Speaker
func (s *Pool) PickConnectedSpeaker() Speaker {
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

// DisconnectAll disconnects all the Speaker
func (s *Pool) DisconnectAll() {
	s.eventHandlersLock.Lock()
	s.eventHandlers = s.eventHandlers[:0]
	s.eventHandlersLock.Unlock()

	var wg sync.WaitGroup

	s.RLock()
	for _, c := range s.speakers {
		wg.Add(1)
		go func(c Speaker) {
			c.StopAndWait()
			wg.Done()
		}(c)
	}
	s.RUnlock()

	wg.Wait()
}

// GetSpeakersByType returns Speakers matching the given type.
func (s *Pool) GetSpeakersByType(serviceType common.ServiceType) (speakers []Speaker) {
	s.RLock()
	for _, c := range s.speakers {
		if c.GetServiceType() == serviceType {
			speakers = append(speakers, c)
		}
	}
	s.RUnlock()
	return
}

// GetSpeakerByRemoteHost returns the Speaker for the given remote host.
func (s *Pool) GetSpeakerByRemoteHost(host string) Speaker {
	for _, c := range s.speakers {
		if c.GetRemoteHost() == host {
			return c
		}
	}
	return nil
}

// SendMessageTo sends message to Speaker for the given remote host.
func (s *Pool) SendMessageTo(m Message, host string) error {
	c := s.GetSpeakerByRemoteHost(host)
	if c == nil {
		return common.ErrNotFound
	}

	return c.SendMessage(m)
}

// BroadcastMessage broadcasts the given message.
func (s *Pool) BroadcastMessage(m Message) {
	s.RLock()
	defer s.RUnlock()

	for _, c := range s.speakers {
		r, err := m.Bytes(c.GetClientProtocol())
		if err != nil {
			s.opts.Logger.Errorf("Unable to send raw message: %s", err)
		}

		if err := c.SendRaw(r); err != nil {
			s.opts.Logger.Errorf("Unable to send raw message: %s", err)
		}
	}
}

// AddEventHandler registers a new event handler.
func (s *Pool) AddEventHandler(h SpeakerEventHandler) {
	s.eventHandlersLock.Lock()
	s.eventHandlers = append(s.eventHandlers, h)
	s.eventHandlersLock.Unlock()
}

// Start starts the pool in a goroutine.
func (s *Pool) Start() {
}

// Stop stops the pool and wait until stopped.
func (s *Pool) Stop() {
	s.DisconnectAll()
}

// ConnectAll calls connect to all the wSSpeakers of the pool.
func (s *ClientPool) ConnectAll() {
	s.RLock()
	// shuffle connections to avoid election of the same client as master
	indexes := rand.Perm(len(s.speakers))
	for _, i := range indexes {
		s.speakers[i].Start()
	}
	s.RUnlock()
}

func newPool(name string, opts PoolOpts) *Pool {
	return &Pool{
		name: name,
		opts: opts,
	}
}

func newIncomerPool(name string, opts PoolOpts) *incomerPool {
	return &incomerPool{
		Pool: newPool(name, opts),
	}
}

// NewClientPool returns a new ClientPool meaning a pool of outgoing Client.
func NewClientPool(name string, opts PoolOpts) *ClientPool {
	if opts.Logger == nil {
		opts.Logger = logging.GetLogger()
	}

	s := &ClientPool{
		Pool: newPool(name, opts),
	}

	s.Start()

	return s
}
