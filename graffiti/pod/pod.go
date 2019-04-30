/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package pod

import (
	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/graffiti/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/validator"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/websocket"
)

// Opts defines pod server options
type Opts struct {
	ServerOpts websocket.ServerOpts
	Validator  validator.Validator
}

// Pod describes a graph pod. It maintains a local graph
// in memory and forward any event to graph hubs
type Pod struct {
	subscriberWSServer *websocket.StructServer
	publisherWSServer  *websocket.StructServer
	forwarder          *Forwarder
	clientPool         *websocket.StructClientPool
}

// Status describes the status of a pod
type Status struct {
	Subscribers map[string]websocket.ConnStatus
	Hubs        map[string]ConnStatus
}

// ConnStatus represents the status of a connection to a hub
type ConnStatus struct {
	websocket.ConnStatus
	IsMaster bool
}

// Start the pod
func (p *Pod) Start() {
	p.subscriberWSServer.Start()
	p.publisherWSServer.Start()
}

// Stop the pod
func (p *Pod) Stop() {
	p.subscriberWSServer.Stop()
	p.publisherWSServer.Stop()
}

// GetStatus returns the status of the pod
func (p *Pod) GetStatus() *Status {
	var masterAddr string
	var masterPort int
	if master := p.forwarder.GetMaster(); master != nil {
		masterAddr, masterPort = master.GetAddrPort()
	}

	hubs := make(map[string]ConnStatus)
	for id, status := range p.clientPool.GetStatus() {
		hubs[id] = ConnStatus{
			ConnStatus: status,
			IsMaster:   status.Addr == masterAddr && status.Port == masterPort,
		}
	}

	return &Status{
		Subscribers: p.subscriberWSServer.GetStatus(),
		Hubs:        hubs,
	}
}

// SubscriberServer returns the websocket server dedicated to subscribers
func (p *Pod) SubscriberServer() *websocket.StructServer {
	return p.subscriberWSServer
}

// Forwarder returns the pod topology forwarder
func (p *Pod) Forwarder() *Forwarder {
	return p.forwarder
}

// NewPod returns a new pod
func NewPod(server *api.Server, clientPool *websocket.StructClientPool, g *graph.Graph, apiAuthBackend shttp.AuthenticationBackend, clusterAuthOptions *shttp.AuthenticationOpts, tr *traversal.GremlinTraversalParser, opts Opts) (*Pod, error) {
	newWSServer := func(endpoint string, authBackend shttp.AuthenticationBackend) *websocket.Server {
		return websocket.NewServer(server.HTTPServer, endpoint, authBackend, opts.ServerOpts)
	}

	subscriberWSServer := websocket.NewStructServer(newWSServer("/ws/subscriber", apiAuthBackend))
	common.NewSubscriberEndpoint(subscriberWSServer, g, tr)

	forwarder := NewForwarder(server.HTTPServer.Host, g, clientPool)

	publisherWSServer := websocket.NewStructServer(newWSServer("/ws/publisher", apiAuthBackend))
	if _, err := NewPublisherEndpoint(publisherWSServer, g, opts.Validator); err != nil {
		return nil, err
	}

	return &Pod{
		subscriberWSServer: subscriberWSServer,
		publisherWSServer:  publisherWSServer,
		forwarder:          forwarder,
		clientPool:         clientPool,
	}, nil
}
