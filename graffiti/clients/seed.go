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

package clients

import (
	"fmt"
	"net/url"

	"github.com/skydive-project/skydive/graffiti/endpoints"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/messages"
	"github.com/skydive-project/skydive/graffiti/service"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// SeedService defines the seed service type
const SeedService service.Type = "seed"

// EventHandler is the interface to be implemented by event handler
type EventHandler interface {
	OnSynchronized()
}

// Seed is a service with its own graph. The seed synchronizes its
// graph by subscribing to the agent using WebSocket. It forwards
// all its graph events to the agent. A filter can be used to
// subscribe only to a part of the agent graph.
type Seed struct {
	*ws.Client
	ws.DefaultSpeakerEventHandler
	subscriber *Subscriber
	g          *graph.Graph
	logger     logging.Logger
}

// OnConnected websocket listener
func (s *Seed) OnConnected(c ws.Speaker) error {
	s.logger.Infof("connected to %s", c.GetHost())
	return s.subscriber.SendMessage(messages.NewStructMessage(messages.SyncRequestMsgType, messages.SyncRequestMsg{}))
}

// Start the seed
func (s *Seed) Start() {
	s.subscriber.Start()
	s.Client.Start()
}

// Stop the seed
func (s *Seed) Stop() {
	s.Client.Stop()
	s.subscriber.Stop()
}

// NewSeed returns a new seed
func NewSeed(g *graph.Graph, clientType service.Type, address, subscribeFilter, publishFilter string, wsOpts ws.ClientOpts, logger logging.Logger) (*Seed, error) {
	wsOpts.Headers.Add("X-Websocket-Namespace", messages.Namespace)

	if len(address) == 0 {
		address = "127.0.0.1:8081"
	}

	url, err := url.Parse("ws://" + address + "/ws/pubsub")
	if err != nil {
		return nil, fmt.Errorf("unable to parse the Address: %s, please check the configuration file", address)
	}

	wsOpts.Headers.Add("X-Gremlin-Filter", subscribeFilter)
	wsOpts.Headers.Add("X-Update-Policy", endpoints.PartialUpdates)

	pubsubClient := ws.NewClient(g.GetHost(), clientType, url, wsOpts)

	pool := ws.NewStructClientPool("pubsub", ws.PoolOpts{Logger: wsOpts.Logger})
	if err := pool.AddClient(pubsubClient); err != nil {
		return nil, fmt.Errorf("failed to add client: %w", err)
	}

	var metadataFilter *filters.Filter
	if publishFilter != "" {
		publishMetadata := graph.Metadata{}
		if _, err := graph.DefToMetadata(publishFilter, publishMetadata); err != nil {
			return nil, fmt.Errorf("failed to create publish filter: %w", err)
		}

		if metadataFilter, err = publishMetadata.Filter(); err != nil {
			return nil, fmt.Errorf("failed to create publish filter: %w", err)
		}
	}

	forwarder := NewForwarder(g, pool, metadataFilter, logger)
	subscriber := NewSubscriber(pubsubClient, g, wsOpts.Logger, forwarder)

	s := &Seed{
		Client:     pubsubClient,
		g:          g,
		subscriber: subscriber,
		logger:     wsOpts.Logger,
	}

	return s, nil
}
