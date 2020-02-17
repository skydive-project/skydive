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
	"github.com/skydive-project/skydive/common"
	gc "github.com/skydive-project/skydive/graffiti/common"
	etcdserver "github.com/skydive-project/skydive/graffiti/etcd/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/validator"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/websocket"
)

// Opts Hub options
type Opts struct {
	ServerOpts     websocket.ServerOpts
	Validator      validator.Validator
	EtcdServerOpts *etcdserver.EmbeddedServerOpts
}

// Hub describes a graph hub that accepts incoming connections
// from pods, other hubs, subscribers or external publishers
type Hub struct {
	server              *shttp.Server
	apiAuthBackend      shttp.AuthenticationBackend
	clusterAuthBackend  shttp.AuthenticationBackend
	podWSServer         *websocket.StructServer
	publisherWSServer   *websocket.StructServer
	replicationWSServer *websocket.StructServer
	replicationEndpoint *ReplicationEndpoint
	subscriberWSServer  *websocket.StructServer
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
func (h *Hub) GetStatus() *Status {
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

	h.podWSServer.Start()
	h.replicationWSServer.Start()
	h.replicationEndpoint.ConnectPeers()
	h.publisherWSServer.Start()
	h.subscriberWSServer.Start()

	return nil
}

// Stop the hub
func (h *Hub) Stop() {
	h.podWSServer.Stop()
	h.replicationWSServer.Stop()
	h.publisherWSServer.Stop()
	h.subscriberWSServer.Stop()
	if h.embeddedEtcd != nil {
		h.embeddedEtcd.Stop()
	}
}

// PodServer returns the websocket server dedicated to pods
func (h *Hub) PodServer() *websocket.StructServer {
	return h.podWSServer
}

// SubscriberServer returns the websocket server dedicated to subscribers
func (h *Hub) SubscriberServer() *websocket.StructServer {
	return h.subscriberWSServer
}

// NewHub returns a new hub
func NewHub(server *shttp.Server, g *graph.Graph, cached *graph.CachedBackend, apiAuthBackend, clusterAuthBackend shttp.AuthenticationBackend, clusterAuthOptions *shttp.AuthenticationOpts, podEndpoint string, peers []common.ServiceAddress, opts Opts) (*Hub, error) {
	var embeddedEtcd *etcdserver.EmbeddedServer
	var err error
	if opts.EtcdServerOpts != nil {
		if embeddedEtcd, err = etcdserver.NewEmbeddedServer(*opts.EtcdServerOpts); err != nil {
			return nil, err
		}
	}

	newWSServer := func(endpoint string, authBackend shttp.AuthenticationBackend) *websocket.Server {
		return websocket.NewServer(server, endpoint, authBackend, opts.ServerOpts)
	}

	podWSServer := websocket.NewStructServer(newWSServer(podEndpoint, clusterAuthBackend))
	_, err = gc.NewPublisherEndpoint(podWSServer, g, nil)
	if err != nil {
		return nil, err
	}

	publisherWSServer := websocket.NewStructServer(newWSServer("/ws/publisher", apiAuthBackend))
	_, err = gc.NewPublisherEndpoint(publisherWSServer, g, opts.Validator)
	if err != nil {
		return nil, err
	}

	replicationWSServer := websocket.NewStructServer(newWSServer("/ws/replication", clusterAuthBackend))
	replicationEndpoint, err := NewReplicationEndpoint(replicationWSServer, clusterAuthOptions, cached, g, peers)
	if err != nil {
		return nil, err
	}

	// declare all extension available through API and filtering
	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())

	subscriberWSServer := websocket.NewStructServer(newWSServer("/ws/subscriber", apiAuthBackend))
	gc.NewSubscriberEndpoint(subscriberWSServer, g, tr)

	return &Hub{
		server:              server,
		apiAuthBackend:      apiAuthBackend,
		clusterAuthBackend:  clusterAuthBackend,
		podWSServer:         podWSServer,
		replicationEndpoint: replicationEndpoint,
		replicationWSServer: replicationWSServer,
		publisherWSServer:   publisherWSServer,
		subscriberWSServer:  subscriberWSServer,
		embeddedEtcd:        embeddedEtcd,
	}, nil
}
