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

	etcd "github.com/coreos/etcd/client"

	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/common"
	gc "github.com/skydive-project/skydive/graffiti/common"
	etcdserver "github.com/skydive-project/skydive/graffiti/etcd/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/validator"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/websocket"
)

// Opts Hub options
type Opts struct {
	Hostname           string
	WebsocketOpts      websocket.ServerOpts
	Validator          validator.Validator
	APIAuthBackend     shttp.AuthenticationBackend
	ClusterAuthBackend shttp.AuthenticationBackend
	ClusterAuthOptions *shttp.AuthenticationOpts
	Peers              []common.ServiceAddress
	TLSConfig          *tls.Config
	EtcdKeysAPI        etcd.KeysAPI
	EtcdServerOpts     *etcdserver.EmbeddedServerOpts
	Logger             logging.Logger
}

// Hub describes a graph hub that accepts incoming connections
// from pods, other hubs, subscribers or external publishers
type Hub struct {
	httpServer          *shttp.Server
	apiServer           *api.Server
	podWSServer         *websocket.StructServer
	publisherWSServer   *websocket.StructServer
	replicationWSServer *websocket.StructServer
	replicationEndpoint *ReplicationEndpoint
	subscriberWSServer  *websocket.StructServer
	traversalParser     *traversal.GremlinTraversalParser
	embeddedEtcd        *etcdserver.EmbeddedServer
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
	if h.embeddedEtcd != nil {
		if err := h.embeddedEtcd.Start(); err != nil {
			return err
		}
	}

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
	if h.embeddedEtcd != nil {
		h.embeddedEtcd.Stop()
	}
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

// NewHub returns a new hub
func NewHub(id string, serviceType common.ServiceType, listen string, g *graph.Graph, cached *graph.CachedBackend, podEndpoint string, opts Opts) (*Hub, error) {
	if opts.Logger == nil {
		opts.Logger = logging.GetLogger()
	}

	service := common.Service{ID: id, Type: serviceType}

	sa, err := common.ServiceAddressFromString(listen)
	if err != nil {
		return nil, err
	}

	var embeddedEtcd *etcdserver.EmbeddedServer
	if opts.EtcdServerOpts != nil {
		if embeddedEtcd, err = etcdserver.NewEmbeddedServer(*opts.EtcdServerOpts); err != nil {
			return nil, err
		}
	}

	tr := traversal.NewGremlinTraversalParser()

	httpServer := shttp.NewServer(service.ID, service.Type, sa.Addr, sa.Port, nil, opts.Logger)

	apiServer, err := api.NewAPI(httpServer, opts.EtcdKeysAPI, service, opts.APIAuthBackend)
	if err != nil {
		return nil, err
	}

	api.RegisterTopologyAPI(httpServer, g, tr, opts.APIAuthBackend)

	if _, err = api.RegisterNodeAPI(apiServer, g, opts.APIAuthBackend); err != nil {
		return nil, err
	}

	if _, err = api.RegisterEdgeAPI(apiServer, g, opts.APIAuthBackend); err != nil {
		return nil, err
	}

	newWSServer := func(endpoint string, authBackend shttp.AuthenticationBackend) *websocket.Server {
		opts := opts.WebsocketOpts
		opts.AuthBackend = authBackend
		return websocket.NewServer(httpServer, endpoint, opts)
	}

	podWSServer := websocket.NewStructServer(newWSServer(podEndpoint, opts.ClusterAuthBackend))
	if _, err = gc.NewPublisherEndpoint(podWSServer, g, nil); err != nil {
		return nil, err
	}

	publisherWSServer := websocket.NewStructServer(newWSServer("/ws/publisher", opts.APIAuthBackend))
	_, err = gc.NewPublisherEndpoint(publisherWSServer, g, opts.Validator)
	if err != nil {
		return nil, err
	}

	replicationWebsocketOpts := &websocket.ClientOpts{
		AuthOpts:         opts.ClusterAuthOptions,
		QueueSize:        opts.WebsocketOpts.QueueSize,
		WriteCompression: opts.WebsocketOpts.WriteCompression,
		TLSConfig:        opts.TLSConfig,
		Logger:           opts.Logger,
	}
	replicationWSServer := websocket.NewStructServer(newWSServer("/ws/replication", opts.ClusterAuthBackend))
	replicationEndpoint, err := NewReplicationEndpoint(replicationWSServer, replicationWebsocketOpts, cached, g, opts.Peers)
	if err != nil {
		return nil, err
	}

	subscriberWSServer := websocket.NewStructServer(newWSServer("/ws/subscriber", opts.APIAuthBackend))
	gc.NewSubscriberEndpoint(subscriberWSServer, g, tr)

	return &Hub{
		httpServer:          httpServer,
		apiServer:           apiServer,
		podWSServer:         podWSServer,
		replicationEndpoint: replicationEndpoint,
		replicationWSServer: replicationWSServer,
		publisherWSServer:   publisherWSServer,
		subscriberWSServer:  subscriberWSServer,
		traversalParser:     tr,
		embeddedEtcd:        embeddedEtcd,
	}, nil
}
