/*
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package analyzer

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/skydive-project/dede/dede"
	"github.com/skydive-project/skydive/alert"
	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/flow"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/client"
	"github.com/skydive-project/skydive/flow/storage"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/packet_injector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/enhancers"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

// Server describes an Analyzer servers mechanism like http, websocket, topology, ondemand probes, ...
type Server struct {
	httpServer          *shttp.Server
	agentWSServer       *shttp.WSJSONServer
	publisherWSServer   *shttp.WSJSONServer
	replicationWSServer *shttp.WSJSONServer
	subscriberWSServer  *shttp.WSJSONServer
	replicationEndpoint *TopologyReplicationEndpoint
	alertServer         *alert.AlertServer
	onDemandClient      *ondemand.OnDemandProbeClient
	piClient            *packet_injector.PacketInjectorClient
	metadataManager     *metadata.UserMetadataManager
	flowServer          *FlowServer
	probeBundle         *probe.ProbeBundle
	storage             storage.Storage
	embeddedEtcd        *etcd.EmbeddedEtcd
	etcdClient          *etcd.Client
	wgServers           sync.WaitGroup
}

// GetStatus returns the status of an analyzer
func (s *Server) GetStatus() interface{} {
	peersStatus := types.PeersStatus{
		Incomers: make(map[string]shttp.WSConnStatus),
		Outgoers: make(map[string]shttp.WSConnStatus),
	}
	for host, peer := range s.replicationEndpoint.conns {
		if host == peer.GetHost() {
			peersStatus.Incomers[host] = peer.GetStatus()
		} else {
			peersStatus.Outgoers[host] = peer.GetStatus()
		}
	}

	return &types.AnalyzerStatus{
		Agents:      s.agentWSServer.GetStatus(),
		Peers:       peersStatus,
		Publishers:  s.publisherWSServer.GetStatus(),
		Subscribers: s.subscriberWSServer.GetStatus(),
		Alerts:      types.ElectionStatus{IsMaster: s.alertServer.IsMaster()},
		Captures:    types.ElectionStatus{IsMaster: s.onDemandClient.IsMaster()},
		Probes:      s.probeBundle.ActiveProbes(),
	}
}

// Start the analyzer server
func (s *Server) Start() error {
	if s.storage != nil {
		s.storage.Start()
	}

	if err := s.httpServer.Listen(); err != nil {
		return err
	}

	s.replicationEndpoint.ConnectPeers()

	s.probeBundle.Start()
	s.onDemandClient.Start()
	s.piClient.Start()
	s.alertServer.Start()
	s.metadataManager.Start()
	s.flowServer.Start()
	s.agentWSServer.Start()
	s.publisherWSServer.Start()
	s.replicationWSServer.Start()
	s.subscriberWSServer.Start()

	s.wgServers.Add(1)
	go func() {
		defer s.wgServers.Done()
		s.httpServer.Serve()
	}()

	return nil
}

// Stop the analyzer server
func (s *Server) Stop() {
	s.flowServer.Stop()
	s.agentWSServer.Stop()
	s.publisherWSServer.Stop()
	s.replicationWSServer.Stop()
	s.subscriberWSServer.Stop()
	s.httpServer.Stop()
	if s.embeddedEtcd != nil {
		s.embeddedEtcd.Stop()
	}
	if s.storage != nil {
		s.storage.Stop()
	}
	s.probeBundle.Stop()
	s.onDemandClient.Stop()
	s.piClient.Stop()
	s.alertServer.Stop()
	s.metadataManager.Stop()
	s.etcdClient.Stop()
	s.wgServers.Wait()
	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
}

// NewServerFromConfig creates a new empty server
func NewServerFromConfig() (*Server, error) {
	embedEtcd := config.GetBool("etcd.embedded")

	hserver, err := shttp.NewServerFromConfig(common.AnalyzerService)
	if err != nil {
		return nil, err
	}

	persistent, err := graph.BackendFromConfig()
	if err != nil {
		return nil, err
	}

	cached, err := graph.NewCachedBackend(persistent)
	if err != nil {
		return nil, err
	}

	g := graph.NewGraphFromConfig(cached)

	authOptions := NewAnalyzerAuthenticationOpts()

	agentWSServer := shttp.NewWSJSONServer(shttp.NewWSServer(hserver, "/ws/agent"))
	_, err = NewTopologyAgentEndpoint(agentWSServer, authOptions, cached, g)
	if err != nil {
		return nil, err
	}

	publisherWSServer := shttp.NewWSJSONServer(shttp.NewWSServer(hserver, "/ws/publisher"))
	_, err = NewTopologyPublisherEndpoint(publisherWSServer, authOptions, g)
	if err != nil {
		return nil, err
	}

	replicationWSServer := shttp.NewWSJSONServer(shttp.NewWSServer(hserver, "/ws/replication"))
	replicationEndpoint, err := NewTopologyReplicationEndpoint(replicationWSServer, authOptions, cached, g)
	if err != nil {
		return nil, err
	}

	subscriberWSServer := shttp.NewWSJSONServer(shttp.NewWSServer(hserver, "/ws/subscriber"))
	topology.NewTopologySubscriberEndpoint(subscriberWSServer, authOptions, g)

	probeBundle, err := NewTopologyProbeBundleFromConfig(g)
	if err != nil {
		return nil, err
	}

	var embeddedEtcd *etcd.EmbeddedEtcd
	if embedEtcd {
		if embeddedEtcd, err = etcd.NewEmbeddedEtcdFromConfig(); err != nil {
			return nil, err
		}
	}

	etcdClient, err := etcd.NewClientFromConfig()
	if err != nil {
		return nil, err
	}

	// wait for etcd to be ready
	for {
		host := config.GetString("host_id")
		if err = etcdClient.SetInt64(fmt.Sprintf("/analyzer:%s/start-time", host), time.Now().Unix()); err != nil {
			logging.GetLogger().Errorf("Etcd server not ready: %s", err.Error())
			time.Sleep(time.Second)
		} else {
			break
		}
	}

	apiServer, err := api.NewAPI(hserver, etcdClient.KeysAPI, common.AnalyzerService)
	if err != nil {
		return nil, err
	}

	captureAPIHandler, err := api.RegisterCaptureAPI(apiServer, g)
	if err != nil {
		return nil, err
	}

	metadataAPIHandler, err := api.RegisterUserMetadataAPI(apiServer, g)
	if err != nil {
		return nil, err
	}

	piAPIHandler, err := api.RegisterPacketInjectorAPI(g, apiServer)
	if err != nil {
		return nil, err
	}

	piClient := packet_injector.NewPacketInjectorClient(agentWSServer, etcdClient, piAPIHandler, g)
	alertAPIHandler, err := api.RegisterAlertAPI(apiServer)
	if err != nil {
		return nil, err
	}

	onDemandClient := ondemand.NewOnDemandProbeClient(g, captureAPIHandler, agentWSServer, subscriberWSServer, etcdClient)

	metadataManager := metadata.NewUserMetadataManager(g, metadataAPIHandler)

	tableClient := flow.NewTableClient(agentWSServer)

	storage, err := storage.NewStorageFromConfig()
	if err != nil {
		return nil, err
	}

	flowServer, err := NewFlowServer(hserver, g, storage, probeBundle)
	if err != nil {
		return nil, err
	}

	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewMetricsTraversalExtension())
	tr.AddTraversalExtension(ge.NewFlowTraversalExtension(tableClient, storage))

	alertServer := alert.NewAlertServer(alertAPIHandler, subscriberWSServer, g, tr, etcdClient)

	s := &Server{
		httpServer:          hserver,
		agentWSServer:       agentWSServer,
		publisherWSServer:   publisherWSServer,
		replicationWSServer: replicationWSServer,
		subscriberWSServer:  subscriberWSServer,
		replicationEndpoint: replicationEndpoint,
		probeBundle:         probeBundle,
		embeddedEtcd:        embeddedEtcd,
		etcdClient:          etcdClient,
		onDemandClient:      onDemandClient,
		piClient:            piClient,
		metadataManager:     metadataManager,
		storage:             storage,
		flowServer:          flowServer,
		alertServer:         alertServer,
	}

	api.RegisterTopologyAPI(hserver, g, tr)
	api.RegisterPcapAPI(hserver, storage)
	api.RegisterConfigAPI(hserver)
	api.RegisterStatusAPI(hserver, s)

	dede.RegisterHandler("terminal", "/dede", hserver.Router)

	return s, nil
}

// NewAnalyzerAuthenticationOpts returns an object to authenticate to the analyzer
func NewAnalyzerAuthenticationOpts() *shttp.AuthenticationOpts {
	return &shttp.AuthenticationOpts{
		Username: config.GetString("auth.analyzer_username"),
		Password: config.GetString("auth.analyzer_password"),
	}
}
