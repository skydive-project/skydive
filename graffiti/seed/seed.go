/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package seed

import (
	"fmt"
	"net/url"

	"github.com/skydive-project/skydive/graffiti/endpoints"
	"github.com/skydive-project/skydive/graffiti/forwarder"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/hub"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/messages"
	"github.com/skydive-project/skydive/graffiti/service"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// Service defines the seed service type
const Service service.Type = "seed"

// EventHandler is the interface to be implemented by event handler
type EventHandler interface {
	OnSynchronized()
}

// Seed is a service with its own graph. The seed synchronizes its
// graph by subscribing to the agent using WebSocket. It forwards
// all its graph events to the agent. A filter can be used to
// subscribe only to a part of the agent graph.
type Seed struct {
	ws.DefaultSpeakerEventHandler
	forwarder  *forwarder.Forwarder
	clientPool *ws.StructClientPool
	publisher  *ws.Client
	subscriber *hub.Subscriber
	g          *graph.Graph
	logger     logging.Logger
	listeners  []EventHandler
}

// OnConnected websocket listener
func (s *Seed) OnConnected(c ws.Speaker) error {
	s.logger.Infof("connected to %s", c.GetHost())
	return s.subscriber.SendMessage(messages.NewStructMessage(messages.SyncRequestMsgType, messages.SyncRequestMsg{}))
}

// AddEventHandler register an event handler
func (s *Seed) AddEventHandler(handler EventHandler) {
	s.listeners = append(s.listeners, handler)
}

// RemoveEventHandler unregister an event handler
func (s *Seed) RemoveEventHandler(handler EventHandler) {
	for i, el := range s.listeners {
		if handler == el {
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			break
		}
	}
}

// Start the seed
func (s *Seed) Start() {
	s.subscriber.Start()
	s.publisher.Start()
}

// Stop the seed
func (s *Seed) Stop() {
	s.publisher.Stop()
	s.subscriber.Stop()
}

// NewSeed returns a new seed
func NewSeed(g *graph.Graph, clientType service.Type, address, filter string, wsOpts ws.ClientOpts, logger logging.Logger) (*Seed, error) {
	wsOpts.Headers.Add("X-Websocket-Namespace", messages.Namespace)

	if len(address) == 0 {
		address = "127.0.0.1:8081"
	}

	url, err := url.Parse("ws://" + address + "/ws/publisher")
	if err != nil {
		return nil, fmt.Errorf("unable to parse the Address: %s, please check the configuration file", address)
	}

	pubClient := ws.NewClient(g.GetHost(), clientType, url, wsOpts)

	pool := ws.NewStructClientPool("publisher", ws.PoolOpts{Logger: wsOpts.Logger})
	if err := pool.AddClient(pubClient); err != nil {
		return nil, fmt.Errorf("failed to add client: %s", err)
	}

	forwarder.NewForwarder(g, pool, logger)

	if url, err = url.Parse("ws://" + address + "/ws/subscriber"); err != nil {
		return nil, fmt.Errorf("unable to parse the Address: %s, please check the configuration file", address)
	}

	wsOpts.Headers.Add("X-Gremlin-Filter", filter)
	wsOpts.Headers.Add("X-Update-Policy", endpoints.PartialUpdates)

	subClient := ws.NewClient(g.GetHost(), clientType, url, wsOpts)
	subscriber := hub.NewSubscriber(subClient, g, wsOpts.Logger)

	s := &Seed{
		g:          g,
		publisher:  pubClient,
		subscriber: subscriber,
		logger:     wsOpts.Logger,
	}

	return s, nil
}
