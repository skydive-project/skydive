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

package hub

import (
	"crypto/tls"
	"fmt"
	"time"

	api "github.com/skydive-project/skydive/graffiti/api/server"
	gc "github.com/skydive-project/skydive/graffiti/common"
	etcdclient "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/schema"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/graffiti/websocket"
)

// Opts Hub options
type Opts struct {
	Hostname            string
	Version             string
	WebsocketOpts       websocket.ServerOpts
	WebsocketClientOpts websocket.ClientOpts
	APIValidator        api.Validator
	GraphValidator      schema.Validator
	TopologyMarshallers api.TopologyMarshallers
	StatusReporter      api.StatusReporter
	APIAuthBackend      shttp.AuthenticationBackend
	ClusterAuthBackend  shttp.AuthenticationBackend
	Peers               []service.Address
	TLSConfig           *tls.Config
	EtcdClient          *etcdclient.Client
	Logger              logging.Logger
}

// Hub describes a graph hub that accepts incoming connections
// from pods, other hubs, subscribers or external publishers
type Hub struct {
	httpServer          *shttp.Server
	apiServer           *api.Server
	etcdClient          *etcdclient.Client
	podWSServer         *websocket.StructServer
	publisherWSServer   *websocket.StructServer
	replicationWSServer *websocket.StructServer
	replicationEndpoint *ReplicationEndpoint
	subscriberWSServer  *websocket.StructServer
	traversalParser     *traversal.GremlinTraversalParser
}

// PeersStatus describes the state of a peer
type PeersStatus struct {
	Incomers map[string]websocket.ConnStatus
	Outgoers map[string]websocket.ConnStatus
}

// Status describes the status of a hub
type Status struct {
	Pods        map[string]websocket.ConnStatus
	Peers       PeersStatus
	Publishers  map[string]websocket.ConnStatus
	Subscribers map[string]websocket.ConnStatus
}

// GetStatus returns the status of a hub
func (h *Hub) GetStatus() interface{} {
	peersStatus := PeersStatus{
		Incomers: make(map[string]websocket.ConnStatus),
		Outgoers: make(map[string]websocket.ConnStatus),
	}

	for _, speaker := range h.replicationEndpoint.in.GetSpeakers() {
		peersStatus.Incomers[speaker.GetRemoteHost()] = speaker.GetStatus()
	}

	for _, speaker := range h.replicationEndpoint.out.GetSpeakers() {
		peersStatus.Outgoers[speaker.GetRemoteHost()] = speaker.GetStatus()
	}

	return &Status{
		Pods:        h.podWSServer.GetStatus(),
		Peers:       peersStatus,
		Publishers:  h.publisherWSServer.GetStatus(),
		Subscribers: h.subscriberWSServer.GetStatus(),
	}
}

// Start the hub
func (h *Hub) Start() error {
	if err := h.httpServer.Start(); err != nil {
		return err
	}

	h.podWSServer.Start()
	h.replicationWSServer.Start()
	h.replicationEndpoint.ConnectPeers()
	h.publisherWSServer.Start()
	h.subscriberWSServer.Start()

	return nil
}

// Stop the hub
func (h *Hub) Stop() {
	h.httpServer.Stop()
	h.podWSServer.Stop()
	h.replicationWSServer.Stop()
	h.publisherWSServer.Stop()
	h.subscriberWSServer.Stop()
}

// HTTPServer returns the hub HTTP server
func (h *Hub) HTTPServer() *shttp.Server {
	return h.httpServer
}

// APIServer returns the hub API server
func (h *Hub) APIServer() *api.Server {
	return h.apiServer
}

// PodServer returns the websocket server dedicated to pods
func (h *Hub) PodServer() *websocket.StructServer {
	return h.podWSServer
}

// SubscriberServer returns the websocket server dedicated to subscribers
func (h *Hub) SubscriberServer() *websocket.StructServer {
	return h.subscriberWSServer
}

// GremlinTraversalParser returns the hub Gremlin traversal parser
func (h *Hub) GremlinTraversalParser() *traversal.GremlinTraversalParser {
	return h.traversalParser
}

// OnPong handles pong messages
func (h *Hub) OnPong(speaker websocket.Speaker) {
	key := fmt.Sprintf("/pods/%s/last-pong", gc.ClientOrigin(speaker))
	if err := h.etcdClient.SetInt64(key, time.Now().Unix()); err != nil {
		logging.GetLogger().Errorf("Error while recording agent pong time: %s", err)
	}
}

// NewHub returns a new hub
func NewHub(id string, serviceType service.Type, listen string, g *graph.Graph, cached *graph.CachedBackend, podEndpoint string, opts Opts) (*Hub, error) {
	sa, err := service.AddressFromString(listen)
	if err != nil {
		return nil, err
	}

	tr := traversal.NewGremlinTraversalParser()

	if opts.Logger == nil {
		opts.Logger = logging.GetLogger()
	}

	hub := &Hub{}

	httpServer := shttp.NewServer(id, serviceType, sa.Addr, sa.Port, opts.TLSConfig, opts.Logger)

	podOpts := opts.WebsocketOpts
	podOpts.AuthBackend = opts.ClusterAuthBackend
	podOpts.PongListeners = []websocket.PongListener{hub}
	podWSServer := websocket.NewStructServer(websocket.NewServer(httpServer, podEndpoint, podOpts))
	if _, err = gc.NewPublisherEndpoint(podWSServer, g, nil, opts.Logger); err != nil {
		return nil, err
	}

	pubOpts := opts.WebsocketOpts
	pubOpts.AuthBackend = opts.APIAuthBackend
	publisherWSServer := websocket.NewStructServer(websocket.NewServer(httpServer, "/ws/publisher", pubOpts))
	_, err = gc.NewPublisherEndpoint(publisherWSServer, g, opts.GraphValidator, opts.Logger)
	if err != nil {
		return nil, err
	}

	repOpts := opts.WebsocketOpts
	repOpts.AuthBackend = opts.ClusterAuthBackend
	replicationWSServer := websocket.NewStructServer(websocket.NewServer(httpServer, "/ws/replication", repOpts))
	replicationEndpoint, err := NewReplicationEndpoint(replicationWSServer, &opts.WebsocketClientOpts, cached, g, opts.Peers, opts.Logger)
	if err != nil {
		return nil, err
	}

	subOpts := opts.WebsocketOpts
	subOpts.AuthBackend = opts.APIAuthBackend
	subscriberWSServer := websocket.NewStructServer(websocket.NewServer(httpServer, "/ws/subscriber", subOpts))
	gc.NewSubscriberEndpoint(subscriberWSServer, g, tr, opts.Logger)

	service := service.Service{ID: id, Type: serviceType}
	apiServer, err := api.NewAPI(httpServer, opts.EtcdClient.KeysAPI, opts.Version, service, opts.APIAuthBackend, opts.APIValidator)
	if err != nil {
		return nil, err
	}

	hub.httpServer = httpServer
	hub.apiServer = apiServer
	hub.podWSServer = podWSServer
	hub.replicationEndpoint = replicationEndpoint
	hub.replicationWSServer = replicationWSServer
	hub.publisherWSServer = publisherWSServer
	hub.subscriberWSServer = subscriberWSServer
	hub.traversalParser = tr
	hub.etcdClient = opts.EtcdClient

	if opts.StatusReporter == nil {
		opts.StatusReporter = hub
	}

	api.RegisterStatusAPI(httpServer, opts.StatusReporter, opts.APIAuthBackend)
	api.RegisterTopologyAPI(httpServer, g, tr, opts.APIAuthBackend, opts.TopologyMarshallers)

	return hub, nil
}
