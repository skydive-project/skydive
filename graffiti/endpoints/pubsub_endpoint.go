/*
 * Copyright (C) 2020 Sylvain Baubeau
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

package endpoints

import (
	"github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/logging"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// PubSubEndpoint describes a WebSocket endpoint that can be used for both
// publishing and subscribing
type PubSubEndpoint struct {
	publisherEndpoint  *PublisherEndpoint
	subscriberEndpoint *SubscriberEndpoint
}

// NewPubSubEndpoint returns a new PubSub endpoint
func NewPubSubEndpoint(pool ws.StructSpeakerPool, validator server.Validator, g *graph.Graph, tr *traversal.GremlinTraversalParser, logger logging.Logger) *PubSubEndpoint {
	return &PubSubEndpoint{
		publisherEndpoint:  NewPublisherEndpoint(pool, g, validator, logger),
		subscriberEndpoint: NewSubscriberEndpoint(pool, g, tr, logger),
	}
}
