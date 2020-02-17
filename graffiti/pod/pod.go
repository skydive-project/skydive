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
	scommon "github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/validator"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/websocket"
)

// Opts defines pod server options
type Opts struct {
	WebsocketOpts      websocket.ServerOpts
	Validator          validator.Validator
	APIAuthBackend     shttp.AuthenticationBackend
	ClusterAuthOptions *shttp.AuthenticationOpts
	Logger             logging.Logger
}

// Pod describes a graph pod. It maintains a local graph
// in memory and forward any event to graph hubs
type Pod struct {
	httpServer         *shttp.Server
	apiServer          *api.Server
	subscriberWSServer *websocket.StructServer
	publisherWSServer  *websocket.StructServer
	forwarder          *common.Forwarder
	clientPool         *websocket.StructClientPool
	traversalParser    *traversal.GremlinTraversalParser
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
func (p *Pod) Start() error {
	p.httpServer.Start()
	p.subscriberWSServer.Start()
	p.publisherWSServer.Start()

	return nil
}

// Stop the pod
func (p *Pod) Stop() {
	p.httpServer.Stop()
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
func (p *Pod) Forwarder() *common.Forwarder {
	return p.forwarder
}

// HTTPServer returns the pod HTTP server
func (p *Pod) HTTPServer() *shttp.Server {
	return p.httpServer
}

// APIServer returns the pod API server
func (p *Pod) APIServer() *api.Server {
	return p.apiServer
}

// GremlinTraversalParser returns the pod Gremlin traversal parser
func (p *Pod) GremlinTraversalParser() *traversal.GremlinTraversalParser {
	return p.traversalParser
}

// NewPod returns a new pod
func NewPod(id string, serviceType scommon.ServiceType, listen string, clientPool *websocket.StructClientPool, g *graph.Graph, opts Opts) (*Pod, error) {
	service := scommon.Service{ID: id, Type: serviceType}

	sa, err := scommon.ServiceAddressFromString(listen)
	if err != nil {
		return nil, err
	}

	if opts.Logger == nil {
		opts.Logger = logging.GetLogger()
	}

	httpServer := shttp.NewServer(id, serviceType, sa.Addr, sa.Port, nil, opts.Logger)

	apiServer, err := api.NewAPI(httpServer, nil, service, opts.APIAuthBackend)
	if err != nil {
		return nil, err
	}

	newWSServer := func(endpoint string, authBackend shttp.AuthenticationBackend) *websocket.Server {
		return websocket.NewServer(httpServer, endpoint, opts.WebsocketOpts)
	}

	subscriberWSServer := websocket.NewStructServer(newWSServer("/ws/subscriber", opts.APIAuthBackend))
	tr := traversal.NewGremlinTraversalParser()
	common.NewSubscriberEndpoint(subscriberWSServer, g, tr)

	forwarder := common.NewForwarder(g, clientPool)

	publisherWSServer := websocket.NewStructServer(newWSServer("/ws/publisher", opts.APIAuthBackend))
	if _, err := common.NewPublisherEndpoint(publisherWSServer, g, opts.Validator); err != nil {
		return nil, err
	}

	api.RegisterTopologyAPI(httpServer, g, tr, opts.APIAuthBackend)

	return &Pod{
		httpServer:         httpServer,
		apiServer:          apiServer,
		subscriberWSServer: subscriberWSServer,
		publisherWSServer:  publisherWSServer,
		forwarder:          forwarder,
		clientPool:         clientPool,
		traversalParser:    tr,
	}, nil
}
