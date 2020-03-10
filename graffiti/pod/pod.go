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
	"crypto/tls"

	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/schema"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/graffiti/websocket"
	shttp "github.com/skydive-project/skydive/http"
)

// Opts defines pod server options
type Opts struct {
	Version             string
	Hubs                []service.Address
	WebsocketOpts       websocket.ServerOpts
	WebsocketClientOpts websocket.ClientOpts
	APIValidator        api.Validator
	GraphValidator      schema.Validator
	TopologyMarshallers api.TopologyMarshallers
	StatusReporter      api.StatusReporter
	TLSConfig           *tls.Config
	APIAuthBackend      shttp.AuthenticationBackend
	Logger              logging.Logger
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

	// everything is ready, then initiate the websocket connection
	go p.clientPool.ConnectAll()

	return nil
}

// Stop the pod
func (p *Pod) Stop() {
	p.clientPool.Stop()
	p.httpServer.Stop()
	p.subscriberWSServer.Stop()
	p.publisherWSServer.Stop()
}

// GetStatus returns the status of the pod
func (p *Pod) GetStatus() interface{} {
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

// ClientPool returns the WebSocket client pool
func (p *Pod) ClientPool() *websocket.StructClientPool {
	return p.clientPool
}

// GremlinTraversalParser returns the pod Gremlin traversal parser
func (p *Pod) GremlinTraversalParser() *traversal.GremlinTraversalParser {
	return p.traversalParser
}

// NewPod returns a new pod
func NewPod(id string, serviceType service.Type, listen string, podEndpoint string, g *graph.Graph, opts Opts) (*Pod, error) {
	sa, err := service.AddressFromString(listen)
	if err != nil {
		return nil, err
	}

	if opts.Logger == nil {
		opts.Logger = logging.GetLogger()
	}

	// Creates a new http WebSocket client pool with authentication
	clientPool := websocket.NewStructClientPool("HubClientPool", websocket.PoolOpts{})
	clientOpts := opts.WebsocketClientOpts
	clientOpts.Protocol = websocket.ProtobufProtocol
	for _, sa := range opts.Hubs {
		url := shttp.MakeURL("ws", sa.Addr, sa.Port, podEndpoint, clientOpts.TLSConfig != nil)
		client := websocket.NewClient(id, serviceType, url, clientOpts)
		clientPool.AddClient(client)
	}

	httpServer := shttp.NewServer(id, serviceType, sa.Addr, sa.Port, opts.TLSConfig, opts.Logger)

	svc := service.Service{ID: id, Type: serviceType}
	apiServer, err := api.NewAPI(httpServer, nil, opts.Version, svc, opts.APIAuthBackend, opts.APIValidator)
	if err != nil {
		return nil, err
	}

	newWSServer := func(endpoint string, authBackend shttp.AuthenticationBackend) *websocket.Server {
		return websocket.NewServer(httpServer, endpoint, opts.WebsocketOpts)
	}

	subscriberWSServer := websocket.NewStructServer(newWSServer("/ws/subscriber", opts.APIAuthBackend))
	tr := traversal.NewGremlinTraversalParser()
	common.NewSubscriberEndpoint(subscriberWSServer, g, tr, opts.Logger)

	forwarder := common.NewForwarder(g, clientPool, logging.GetLogger())

	publisherWSServer := websocket.NewStructServer(newWSServer("/ws/publisher", opts.APIAuthBackend))
	if _, err := common.NewPublisherEndpoint(publisherWSServer, g, opts.GraphValidator, opts.Logger); err != nil {
		return nil, err
	}

	api.RegisterTopologyAPI(httpServer, g, tr, opts.APIAuthBackend, opts.TopologyMarshallers)

	pod := &Pod{
		httpServer:         httpServer,
		apiServer:          apiServer,
		subscriberWSServer: subscriberWSServer,
		publisherWSServer:  publisherWSServer,
		forwarder:          forwarder,
		clientPool:         clientPool,
		traversalParser:    tr,
	}

	if opts.StatusReporter == nil {
		opts.StatusReporter = pod
	}
	api.RegisterStatusAPI(httpServer, opts.StatusReporter, opts.APIAuthBackend)

	return pod, nil
}
