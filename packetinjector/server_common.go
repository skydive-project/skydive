/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package packetinjector

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/graph"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
	"github.com/skydive-project/skydive/ondemand/server"
)

const (
	// Namespace PacketInjector
	Namespace    = "PacketInjector"
	resourceName = "PacketInjection"
)

type onDemandPacketInjectServer struct {
	sync.RWMutex
	graph *graph.Graph
}

// NewOnDemandInjectionServer creates a new Ondemand probes server based on graph and websocket
func NewOnDemandInjectionServer(g *graph.Graph, pool *ws.StructClientPool) (*server.OnDemandServer, error) {
	return server.NewOnDemandServer(g, pool, &onDemandPacketInjectServer{
		graph: g,
	})
}

func (o *onDemandPacketInjectServer) ResourceName() string {
	return resourceName
}

func (o *onDemandPacketInjectServer) DecodeMessage(msg json.RawMessage) (rest.Resource, error) {
	var params PacketInjectionRequest
	if err := json.Unmarshal(msg, &params); err != nil {
		return nil, fmt.Errorf("Unable to decode packet inject param message %v", msg)
	}
	return &params, nil
}
