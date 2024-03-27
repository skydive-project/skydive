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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	etcd "go.etcd.io/etcd/client/v2"

	"github.com/skydive-project/skydive/graffiti/alert"
	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/assets"
	"github.com/skydive-project/skydive/graffiti/clients"
	"github.com/skydive-project/skydive/graffiti/endpoints"
	etcdclient "github.com/skydive-project/skydive/graffiti/etcd/client"
	etcdserver "github.com/skydive-project/skydive/graffiti/etcd/server"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/schema"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/graffiti/websocket"
)

const (
	etcPodPongPath = "/ws-pong/pods"
)

// Opts Hub options
type Opts struct {
	Hostname            string
	Version             string
	ClusterName         string
	WebsocketOpts       websocket.ServerOpts
	WebsocketClientOpts websocket.ClientOpts
	APIValidator        api.Validator
	GraphValidator      schema.Validator
	TopologyMarshallers api.TopologyMarshallers
	StatusReporter      api.StatusReporter
	APIAuthBackend      shttp.AuthenticationBackend
	ClusterAuthBackend  shttp.AuthenticationBackend
	ReplicationPeers    []service.Address
	ClusterPeers        map[string]*PeeringOpts
	TLSConfig           *tls.Config
	EtcdClient          *etcdclient.Client
	EtcdServerOpts      *etcdserver.EmbeddedServerOpts
	Logger              logging.Logger
	Assets              assets.Assets
}

type PeeringOpts struct {
	Endpoints           []service.Address
	WebsocketClientOpts websocket.ClientOpts
	PublisherFilter     string
	SubscriptionFilter  string
}

// Hub describes a graph hub that accepts incoming connections
// from pods, other hubs, subscribers or external publishers
type Hub struct {
	Graph                *graph.Graph
	cached               *graph.CachedBackend
	logger               logging.Logger
	httpServer           *shttp.Server
	apiServer            *api.Server
	alertServer          *alert.Server
	embeddedEtcd         *etcdserver.EmbeddedServer
	etcdClient           *etcdclient.Client
	podWSServer          *websocket.StructServer
	publisherWSServer    *websocket.StructServer
	replicationWSServer  *websocket.StructServer
	replicationEndpoint  *endpoints.ReplicationEndpoint
	subscriberWSServer   *websocket.StructServer
	traversalParser      *traversal.GremlinTraversalParser
	expirationDelay      time.Duration
	quit                 chan bool
	originMasterElection etcdclient.MasterElection
	clusterPeerings      map[string]*clusterPeering
}

// ElectionStatus describes the status of an election
type ElectionStatus struct {
	IsMaster bool
}

// PeersStatus describes the state of a peer
type PeersStatus struct {
	Incomers map[string]websocket.ConnStatus
	Outgoers map[string]websocket.ConnStatus
}

// PeeredClustersStatus describes the state of peering with an other cluster
type PeeredClustersStatus struct {
	Election ElectionStatus
	Outgoers []websocket.ConnStatus
}

// Status describes the status of a hub
type Status struct {
	Alerts         ElectionStatus
	Pods           map[string]websocket.ConnStatus
	Peers          PeersStatus
	Publishers     map[string]websocket.ConnStatus
	Subscribers    map[string]websocket.ConnStatus
	PeeredClusters map[string]PeeredClustersStatus
}

// GetStatus returns the status of a hub
func (h *Hub) GetStatus() interface{} {
	peersStatus := PeersStatus{
		Incomers: make(map[string]websocket.ConnStatus),
		Outgoers: make(map[string]websocket.ConnStatus),
	}

	for _, speaker := range h.replicationEndpoint.GetIncomingSpeakers() {
		peersStatus.Incomers[speaker.GetRemoteHost()] = speaker.GetStatus()
	}

	for _, speaker := range h.replicationEndpoint.GetOutgoingSpeakers() {
		peersStatus.Outgoers[speaker.GetRemoteHost()] = speaker.GetStatus()
	}

	peeredClusters := make(map[string]PeeredClustersStatus)
	for cluster, peering := range h.clusterPeerings {
		outgoers := make([]websocket.ConnStatus, len(peering.peers.GetSpeakers()))
		for i, speaker := range peering.peers.GetSpeakers() {
			outgoers[i] = speaker.GetStatus()
		}
		peeredClusters[cluster] = PeeredClustersStatus{
			Election: ElectionStatus{
				IsMaster: peering.masterElection.IsMaster(),
			},
			Outgoers: outgoers,
		}
	}

	return &Status{
		Pods:           h.podWSServer.GetStatus(),
		Peers:          peersStatus,
		Publishers:     h.publisherWSServer.GetStatus(),
		Subscribers:    h.subscriberWSServer.GetStatus(),
		Alerts:         ElectionStatus{IsMaster: h.alertServer.IsMaster()},
		PeeredClusters: peeredClusters,
	}
}

// OnStarted - Persistent backend listener
func (h *Hub) OnStarted() {
	go h.watchOrigins()

	if err := h.httpServer.Start(); err != nil {
		h.logger.Errorf("Error while starting http server: %s", err)
		return
	}

	h.alertServer.Start()
	h.podWSServer.Start()
	h.replicationWSServer.Start()
	h.replicationEndpoint.ConnectPeers()
	h.publisherWSServer.Start()
	h.subscriberWSServer.Start()
}

// Start the hub
func (h *Hub) Start() error {
	if h.embeddedEtcd != nil {
		if err := h.embeddedEtcd.Start(); err != nil {
			return err
		}
	}

	h.originMasterElection.StartAndWait()

	for _, peering := range h.clusterPeerings {
		peering.masterElection.StartAndWait()
	}

	if err := h.cached.Start(); err != nil {
		return err
	}

	return nil
}

// Stop the hub
func (h *Hub) Stop() {
	h.httpServer.Stop()
	h.podWSServer.Stop()
	h.replicationWSServer.Stop()
	h.publisherWSServer.Stop()
	h.subscriberWSServer.Stop()
	h.alertServer.Stop()
	h.cached.Stop()
	h.originMasterElection.Stop()
	for _, peering := range h.clusterPeerings {
		peering.masterElection.Stop()
	}
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

// OnPong handles pong messages and store the last pong timestamp in etcd
func (h *Hub) OnPong(speaker websocket.Speaker) {
	key := fmt.Sprintf("%s/%s", etcPodPongPath, graph.ClientOrigin(speaker))
	if err := h.etcdClient.SetInt64(key, time.Now().Unix()); err != nil {
		h.logger.Errorf("Error while recording Pod pong time: %s", err)
	}
}

func (h *Hub) watchOrigins() {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			if !h.originMasterElection.IsMaster() {
				break
			}

			resp, err := h.etcdClient.KeysAPI.Get(context.Background(), etcPodPongPath, &etcd.GetOptions{Recursive: true})
			if err != nil {
				continue
			}

			for _, node := range resp.Node.Nodes {
				t, _ := strconv.ParseInt(node.Value, 10, 64)

				h.logger.Infof("TTL of pod of origin %s is %d", node.Key, t)

				if t+int64(h.expirationDelay.Seconds()) < time.Now().Unix() {
					origin := strings.TrimPrefix(node.Key, etcPodPongPath+"/")

					h.logger.Infof("pod of origin %s expired, removing resources", origin)

					h.Graph.Lock()
					graph.DelSubGraphOfOrigin(h.Graph, origin)
					h.Graph.Unlock()

					if _, err := h.etcdClient.KeysAPI.Delete(context.Background(), node.Key, &etcd.DeleteOptions{}); err != nil {
						h.logger.Infof("unable to delete pod entry %s: %s", node.Key, err)
					}
				}
			}
		case <-h.quit:
			return
		}
	}
}

// NewHub returns a new hub
func NewHub(id string, serviceType service.Type, listen string, g *graph.Graph, cached *graph.CachedBackend, podEndpoint string, opts Opts) (*Hub, error) {
	sa, err := service.AddressFromString(listen)
	if err != nil {
		return nil, err
	}

	if len(opts.ClusterPeers) > 0 && opts.ClusterName == "" {
		return nil, errors.New("peering was requested but analyzer has no cluster name")
	}

	tr := traversal.NewGremlinTraversalParser()

	if opts.Logger == nil {
		opts.Logger = logging.GetLogger()
	}

	if opts.WebsocketClientOpts.Logger == nil {
		opts.WebsocketClientOpts.Logger = opts.Logger
	}

	hub := &Hub{
		Graph:           g,
		cached:          cached,
		logger:          opts.Logger,
		expirationDelay: opts.WebsocketOpts.PongTimeout * 5,
		quit:            make(chan bool),
	}
	cached.AddListener(hub)

	if opts.EtcdServerOpts != nil {
		embeddedEtcd, err := etcdserver.NewEmbeddedServer(*opts.EtcdServerOpts)
		if err != nil {
			return nil, err
		}
		hub.embeddedEtcd = embeddedEtcd
	}

	httpServer := shttp.NewServer(id, serviceType, sa.Addr, sa.Port, opts.TLSConfig, opts.Logger)

	podOpts := opts.WebsocketOpts
	podOpts.AuthBackend = opts.ClusterAuthBackend
	podOpts.PongListeners = []websocket.PongListener{hub}
	podWSServer := websocket.NewStructServer(websocket.NewServer(httpServer, podEndpoint, podOpts))
	endpoints.NewPublisherEndpoint(podWSServer, g, nil, opts.Logger, nil)

	pubOpts := opts.WebsocketOpts
	pubOpts.AuthBackend = opts.APIAuthBackend
	publisherWSServer := websocket.NewStructServer(websocket.NewServer(httpServer, "/ws/publisher", pubOpts))
	endpoints.NewPublisherEndpoint(publisherWSServer, g, opts.GraphValidator, opts.Logger, nil)

	repOpts := opts.WebsocketOpts
	repOpts.AuthBackend = opts.ClusterAuthBackend
	repOpts.PongListeners = []websocket.PongListener{hub}
	replicationWSServer := websocket.NewStructServer(websocket.NewServer(httpServer, "/ws/replication", repOpts))
	replicationEndpoint := endpoints.NewReplicationEndpoint(replicationWSServer, &opts.WebsocketClientOpts, cached, g, opts.ReplicationPeers, opts.Logger)

	subOpts := opts.WebsocketOpts
	subOpts.AuthBackend = opts.APIAuthBackend
	subscriberWSServer := websocket.NewStructServer(websocket.NewServer(httpServer, "/ws/subscriber", subOpts))
	endpoints.NewSubscriberEndpoint(subscriberWSServer, g, tr, opts.Logger)

	pubsubOpts := opts.WebsocketOpts
	pubsubOpts.AuthBackend = opts.APIAuthBackend
	pubsubWSServer := websocket.NewStructServer(websocket.NewServer(httpServer, "/ws/pubsub", pubsubOpts))
	endpoints.NewPubSubEndpoint(pubsubWSServer, opts.GraphValidator, g, tr, opts.Logger)

	apiServer, err := api.NewAPI(httpServer, opts.EtcdClient, opts.Version, id, serviceType, opts.APIAuthBackend, opts.APIValidator)
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

	election := hub.etcdClient.NewElection("/elections/hub-origin-watcher")
	hub.originMasterElection = election

	hub.clusterPeerings = make(map[string]*clusterPeering)
	for remoteCluster, peeringOpts := range opts.ClusterPeers {
		opts.Logger.Debugf("Peering with cluster %s and endpoints %+v", remoteCluster, peeringOpts.Endpoints)

		clientPool := websocket.NewClientPool("HubPeering-"+remoteCluster, websocket.PoolOpts{Logger: opts.WebsocketClientOpts.Logger})

		peering := &clusterPeering{
			clusterName: remoteCluster,
			peers:       clientPool,
			logger:      opts.Logger,
		}

		peering.masterElection = hub.etcdClient.NewElection("/elections/hub-peering/" + opts.ClusterName + "/" + remoteCluster)

		for _, peer := range peeringOpts.Endpoints {
			var client websocket.Speaker
			switch {
			case peeringOpts.SubscriptionFilter != "" && peeringOpts.PublisherFilter != "":
				var subscriptionFilter, publisherFilter string
				if peeringOpts.SubscriptionFilter != "*" {
					subscriptionFilter = peeringOpts.SubscriptionFilter
				}
				if peeringOpts.PublisherFilter != "*" {
					publisherFilter = peeringOpts.PublisherFilter
				}

				client, err = clients.NewSeed(g, serviceType, fmt.Sprintf("%s:%d", peer.Addr, peer.Port), subscriptionFilter, publisherFilter, peeringOpts.WebsocketClientOpts, opts.Logger)
				if err != nil {
					return nil, err
				}
			case peeringOpts.SubscriptionFilter != "":
				url, _ := shttp.MakeURL("ws", peer.Addr, peer.Port, "/ws/subscriber", peeringOpts.WebsocketClientOpts.TLSConfig != nil)
				wsClient := websocket.NewClient(id, serviceType, url, peeringOpts.WebsocketClientOpts)
				if peeringOpts.SubscriptionFilter != "*" {
					peeringOpts.WebsocketClientOpts.Headers.Add("X-Gremlin-Filter", peeringOpts.SubscriptionFilter)
				}
				client = clients.NewSubscriber(wsClient, g, opts.Logger, nil)
			default:
				url, _ := shttp.MakeURL("ws", peer.Addr, peer.Port, "/ws/publisher", peeringOpts.WebsocketClientOpts.TLSConfig != nil)
				client = websocket.NewClient(id, serviceType, url, peeringOpts.WebsocketClientOpts)
			}
			clientPool.AddClient(client)
		}

		if peeringOpts.SubscriptionFilter == "" {
			opts.Logger.Debugf("Creating new forwarder for peering")

			var metadataFilter *filters.Filter
			if peeringOpts.PublisherFilter != "*" {
				publishMetadata := graph.Metadata{}
				_, err := graph.DefToMetadata(peeringOpts.PublisherFilter, publishMetadata)
				if err != nil {
					return nil, err
				}

				if metadataFilter, err = publishMetadata.Filter(); err != nil {
					return nil, fmt.Errorf("failed to create publish filter: %w", err)
				}
			}
			clients.NewForwarder(g, clientPool, metadataFilter, opts.Logger)
		}

		hub.clusterPeerings[remoteCluster] = peering

		peering.masterElection.AddEventListener(peering)
	}

	if opts.StatusReporter == nil {
		opts.StatusReporter = hub
	}

	api.RegisterStatusAPI(httpServer, opts.StatusReporter, opts.APIAuthBackend)
	api.RegisterTopologyAPI(httpServer, g, tr, opts.APIAuthBackend, opts.TopologyMarshallers)
	api.RegisterNodeAPI(apiServer, g, opts.APIAuthBackend)
	api.RegisterEdgeAPI(apiServer, g, opts.APIAuthBackend)
	api.RegisterAlertAPI(apiServer, opts.APIAuthBackend)

	if _, err := api.RegisterWorkflowAPI(apiServer, g, tr, opts.Assets, opts.APIAuthBackend); err != nil {
		return nil, err
	}

	hub.alertServer, err = alert.NewServer(apiServer, subscriberWSServer, g, tr, opts.EtcdClient, opts.Assets)
	if err != nil {
		return nil, err
	}

	return hub, nil
}
