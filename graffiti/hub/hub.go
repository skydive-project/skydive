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
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/pod"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/websocket"
)

// Hub describes a graph hub that accepts incoming connections
// from pods, other hubs, subscribers or external publishers
type Hub struct {
	server                 *shttp.Server
	apiAuthBackend         shttp.AuthenticationBackend
	clusterAuthBackend     shttp.AuthenticationBackend
	writeCompression       bool
	queueSize              int
	pingDelay, pongTimeout time.Duration
	podWSServer            *websocket.StructServer
	publisherWSServer      *websocket.StructServer
	replicationWSServer    *websocket.StructServer
	replicationEndpoint    *TopologyReplicationEndpoint
	subscriberWSServer     *websocket.StructServer
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
func (h *Hub) Start() {
	h.podWSServer.Start()
	h.replicationWSServer.Start()
	h.replicationEndpoint.ConnectPeers()
	h.publisherWSServer.Start()
	h.subscriberWSServer.Start()
}

// Stop the hub
func (h *Hub) Stop() {
	h.podWSServer.Stop()
	h.replicationWSServer.Stop()
	h.publisherWSServer.Stop()
	h.subscriberWSServer.Stop()
}

// PodServer returns the websocket server dedicated to pods
func (h *Hub) PodServer() *websocket.StructServer {
	return h.podWSServer
}

// SubscriberServer returns the websocket server dedicated to subcribers
func (h *Hub) SubscriberServer() *websocket.StructServer {
	return h.subscriberWSServer
}

// NewHub returns a new hub
func NewHub(server *shttp.Server, g *graph.Graph, cached *graph.CachedBackend, apiAuthBackend, clusterAuthBackend shttp.AuthenticationBackend, clusterAuthOptions *shttp.AuthenticationOpts, podEndpoint string, writeCompression bool, queueSize int, pingDelay, pongTimeout time.Duration) (*Hub, error) {
	newWSServer := func(endpoint string, authBackend shttp.AuthenticationBackend) *websocket.Server {
		return websocket.NewServer(server, endpoint, authBackend, writeCompression, queueSize, pingDelay, pongTimeout)
	}

	podWSServer := websocket.NewStructServer(newWSServer(podEndpoint, clusterAuthBackend))
	_, err := NewTopologyPodEndpoint(podWSServer, cached, g)
	if err != nil {
		return nil, err
	}

	publisherWSServer := websocket.NewStructServer(newWSServer("/ws/publisher", apiAuthBackend))
	_, err = NewTopologyPublisherEndpoint(publisherWSServer, g)
	if err != nil {
		return nil, err
	}

	replicationWSServer := websocket.NewStructServer(newWSServer("/ws/replication", clusterAuthBackend))
	replicationEndpoint, err := NewTopologyReplicationEndpoint(replicationWSServer, clusterAuthOptions, cached, g)
	if err != nil {
		return nil, err
	}

	// declare all extension available through API and filtering
	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())

	subscriberWSServer := websocket.NewStructServer(newWSServer("/ws/subscriber", apiAuthBackend))
	pod.NewTopologySubscriberEndpoint(subscriberWSServer, g, tr)

	return &Hub{
		server:              server,
		apiAuthBackend:      apiAuthBackend,
		clusterAuthBackend:  clusterAuthBackend,
		writeCompression:    writeCompression,
		queueSize:           queueSize,
		pingDelay:           pingDelay,
		pongTimeout:         pongTimeout,
		podWSServer:         podWSServer,
		replicationEndpoint: replicationEndpoint,
		replicationWSServer: replicationWSServer,
		publisherWSServer:   publisherWSServer,
		subscriberWSServer:  subscriberWSServer,
	}, nil
}
