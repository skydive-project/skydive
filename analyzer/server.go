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

	"github.com/skydive-project/skydive/alert"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/flow"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/client"
	"github.com/skydive-project/skydive/flow/storage"
	ftraversal "github.com/skydive-project/skydive/flow/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/packet_injector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/enhancers"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

// Server describes an Analyzer servers mechanism like http, websocket, topology, ondemand probes, ...
type Server struct {
	httpServer      *shttp.Server
	wsServer        *shttp.WSJSONServer
	topologyServer  *TopologyServer
	alertServer     *alert.AlertServer
	onDemandClient  *ondemand.OnDemandProbeClient
	metadataManager *metadata.UserMetadataManager
	flowServer      *FlowServer
	probeBundle     *probe.ProbeBundle
	storage         storage.Storage
	embeddedEtcd    *etcd.EmbeddedEtcd
	etcdClient      *etcd.EtcdClient
	wgServers       sync.WaitGroup
}

// ElectionStatus describes the status of an election
type ElectionStatus struct {
	IsMaster bool
}

// AnalyzerStatus describes the status of an analyzer
type AnalyzerStatus struct {
	Clients  map[string]shttp.WSConnStatus
	Peers    map[string]shttp.WSConnStatus
	Alerts   ElectionStatus
	Captures ElectionStatus
}

// GetStatus returns the status of an analyzer
func (s *Server) GetStatus() interface{} {
	peers := make(map[string]shttp.WSConnStatus)
	for _, peer := range s.topologyServer.peers {
		if peer.wsclient != nil && peer.host != config.GetConfig().GetString("host_id") {
			peers[peer.host] = peer.wsclient.GetStatus()
		}
	}

	return &AnalyzerStatus{
		Clients:  s.wsServer.GetStatus(),
		Peers:    peers,
		Alerts:   ElectionStatus{IsMaster: s.alertServer.IsMaster()},
		Captures: ElectionStatus{IsMaster: s.onDemandClient.IsMaster()},
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

	s.topologyServer.ConnectPeers()

	s.probeBundle.Start()
	s.onDemandClient.Start()
	s.alertServer.Start()
	s.metadataManager.Start()
	s.flowServer.Start()

	s.wgServers.Add(2)
	go func() {
		defer s.wgServers.Done()
		s.httpServer.Serve()
	}()

	go func() {
		defer s.wgServers.Done()
		s.wsServer.Run()
	}()

	return nil
}

// Stop the analyzer server
func (s *Server) Stop() {
	s.flowServer.Stop()
	s.wsServer.Stop()
	s.httpServer.Stop()
	if s.embeddedEtcd != nil {
		s.embeddedEtcd.Stop()
	}
	if s.storage != nil {
		s.storage.Stop()
	}
	s.probeBundle.Stop()
	s.onDemandClient.Stop()
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
	embedEtcd := config.GetConfig().GetBool("etcd.embedded")

	hserver, err := shttp.NewServerFromConfig(common.AnalyzerService)
	if err != nil {
		return nil, err
	}

	wsServer := shttp.NewWSJSONServer(shttp.NewWSServer(hserver, "/ws"))

	topologyServer, err := NewTopologyServer(wsServer, NewAnalyzerAuthenticationOpts())
	if err != nil {
		return nil, err
	}
	g := topologyServer.Graph

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

	etcdClient, err := etcd.NewEtcdClientFromConfig()
	if err != nil {
		return nil, err
	}

	// wait for etcd to be ready
	for {
		host := config.GetConfig().GetString("host_id")
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

	alertAPIHandler, err := api.RegisterAlertAPI(apiServer)
	if err != nil {
		return nil, err
	}

	onDemandClient := ondemand.NewOnDemandProbeClient(g, captureAPIHandler, wsServer, etcdClient)

	metadataManager := metadata.NewUserMetadataManager(g, metadataAPIHandler)

	tableClient := flow.NewTableClient(wsServer)

	storage, err := storage.NewStorageFromConfig()
	if err != nil {
		return nil, err
	}

	flowServer, err := NewFlowServer(hserver, g, storage, probeBundle)
	if err != nil {
		return nil, err
	}

	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(topology.NewTopologyTraversalExtension())
	tr.AddTraversalExtension(ftraversal.NewFlowTraversalExtension(tableClient, storage))

	alertServer := alert.NewAlertServer(alertAPIHandler, wsServer, g, tr, etcdClient)

	piClient := packet_injector.NewPacketInjectorClient(wsServer)

	s := &Server{
		httpServer:      hserver,
		wsServer:        wsServer,
		topologyServer:  topologyServer,
		probeBundle:     probeBundle,
		embeddedEtcd:    embeddedEtcd,
		etcdClient:      etcdClient,
		onDemandClient:  onDemandClient,
		metadataManager: metadataManager,
		storage:         storage,
		flowServer:      flowServer,
		alertServer:     alertServer,
	}

	api.RegisterTopologyAPI(hserver, g, tr)
	api.RegisterPacketInjectorAPI(piClient, g, hserver)
	api.RegisterPcapAPI(hserver, storage)
	api.RegisterConfigAPI(hserver)
	api.RegisterStatusAPI(hserver, s)

	return s, nil
}

// NewAnalyzerAuthenticationOpts returns an object to authenticate to the analyzer
func NewAnalyzerAuthenticationOpts() *shttp.AuthenticationOpts {
	return &shttp.AuthenticationOpts{
		Username: config.GetConfig().GetString("auth.analyzer_username"),
		Password: config.GetConfig().GetString("auth.analyzer_password"),
	}
}
