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
	"os"
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
	HTTPServer      *shttp.Server
	WSServer        *shttp.WSJSONServer
	TopologyServer  *TopologyServer
	AlertServer     *alert.AlertServer
	OnDemandClient  *ondemand.OnDemandProbeClient
	MetadataManager *metadata.UserMetadataManager
	FlowServer      *FlowServer
	ProbeBundle     *probe.ProbeBundle
	Storage         storage.Storage
	EmbeddedEtcd    *etcd.EmbeddedEtcd
	EtcdClient      *etcd.EtcdClient
	wgServers       sync.WaitGroup
	wgFlowsHandlers sync.WaitGroup
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
	for _, peer := range s.TopologyServer.peers {
		if peer.wsclient != nil && peer.host != config.GetConfig().GetString("host_id") {
			peers[peer.host] = peer.wsclient.GetStatus()
		}
	}

	return &AnalyzerStatus{
		Clients:  s.WSServer.GetStatus(),
		Peers:    peers,
		Alerts:   ElectionStatus{IsMaster: s.AlertServer.IsMaster()},
		Captures: ElectionStatus{IsMaster: s.OnDemandClient.IsMaster()},
	}
}

func (s *Server) initialize() (err error) {
	embedEtcd := config.GetConfig().GetBool("etcd.embedded")

	if s.HTTPServer, err = shttp.NewServerFromConfig(common.AnalyzerService); err != nil {
		return
	}

	s.WSServer = shttp.NewWSJSONServer(shttp.NewWSServerFromConfig(s.HTTPServer, "/ws"))

	if s.TopologyServer, err = NewTopologyServer(s.WSServer, NewAnalyzerAuthenticationOpts()); err != nil {
		return
	}

	if s.ProbeBundle, err = NewTopologyProbeBundleFromConfig(s.TopologyServer.Graph); err != nil {
		return
	}

	if embedEtcd {
		if s.EmbeddedEtcd, err = etcd.NewEmbeddedEtcdFromConfig(); err != nil {
			return
		}
	}

	if s.EtcdClient, err = etcd.NewEtcdClientFromConfig(); err != nil {
		return
	}

	// wait for etcd to be ready
	for {
		host := config.GetConfig().GetString("host_id")
		if err = s.EtcdClient.SetInt64(fmt.Sprintf("/analyzer:%s/start-time", host), time.Now().Unix()); err != nil {
			logging.GetLogger().Errorf("Etcd server not ready: %s", err.Error())
			time.Sleep(time.Second)
		} else {
			break
		}
	}

	var apiServer *api.Server
	if apiServer, err = api.NewAPI(s.HTTPServer, s.EtcdClient.KeysAPI, common.AnalyzerService); err != nil {
		return
	}

	var captureAPIHandler *api.CaptureAPIHandler
	if captureAPIHandler, err = api.RegisterCaptureAPI(apiServer, s.TopologyServer.Graph); err != nil {
		return
	}

	var metadataAPIHandler *api.UserMetadataAPIHandler
	if metadataAPIHandler, err = api.RegisterUserMetadataAPI(apiServer, s.TopologyServer.Graph); err != nil {
		return
	}

	var alertAPIHandler *api.AlertAPIHandler
	if alertAPIHandler, err = api.RegisterAlertAPI(apiServer); err != nil {
		return
	}

	s.OnDemandClient = ondemand.NewOnDemandProbeClient(s.TopologyServer.Graph, captureAPIHandler, s.WSServer, s.EtcdClient)

	s.MetadataManager = metadata.NewUserMetadataManager(s.TopologyServer.Graph, metadataAPIHandler)

	tableClient := flow.NewTableClient(s.WSServer)

	if s.Storage, err = storage.NewStorageFromConfig(); err != nil {
		return
	}

	if s.FlowServer, err = NewFlowServer(s.HTTPServer, s.TopologyServer.Graph, s.Storage, s.ProbeBundle); err != nil {
		return
	}

	tr := traversal.NewGremlinTraversalParser(s.TopologyServer.Graph)
	tr.AddTraversalExtension(topology.NewTopologyTraversalExtension())
	tr.AddTraversalExtension(ftraversal.NewFlowTraversalExtension(tableClient, s.Storage))

	s.AlertServer = alert.NewAlertServer(alertAPIHandler, s.WSServer, tr, s.EtcdClient)

	piClient := packet_injector.NewPacketInjectorClient(s.WSServer)

	api.RegisterTopologyAPI(s.HTTPServer, tr)

	if config.GetConfig().GetBool("analyzer.inject_enabled") {
		api.RegisterPacketInjectorAPI(piClient, s.TopologyServer.Graph, s.HTTPServer)
	}
	api.RegisterPcapAPI(s.HTTPServer, s.Storage)

	api.RegisterConfigAPI(s.HTTPServer)

	api.RegisterStatusAPI(s.HTTPServer, s)

	return s.HTTPServer.Listen()
}

// Start the analyzer server
func (s *Server) Start() {
	if err := s.initialize(); err != nil {
		logging.GetLogger().Error(err)
		os.Exit(1)
	}

	if s.Storage != nil {
		s.Storage.Start()
	}

	s.TopologyServer.ConnectPeers()

	s.ProbeBundle.Start()
	s.OnDemandClient.Start()
	s.AlertServer.Start()
	s.MetadataManager.Start()

	s.wgServers.Add(2)
	go func() {
		defer s.wgServers.Done()
		s.HTTPServer.Serve()
	}()

	go func() {
		defer s.wgServers.Done()
		s.WSServer.Run()
	}()

	s.FlowServer.Start()
}

// Stop the analyzer server
func (s *Server) Stop() {
	s.FlowServer.Stop()
	s.WSServer.Stop()
	s.HTTPServer.Stop()
	if s.EmbeddedEtcd != nil {
		s.EmbeddedEtcd.Stop()
	}
	if s.Storage != nil {
		s.Storage.Stop()
	}
	s.ProbeBundle.Stop()
	s.OnDemandClient.Stop()
	s.AlertServer.Stop()
	s.MetadataManager.Stop()
	s.EtcdClient.Stop()
	s.wgServers.Wait()
	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
}

// NewServerFromConfig creates a new empty server
func NewServerFromConfig() *Server {
	return &Server{}
}

// NewAnalyzerAuthenticationOpts returns an object to authenticate to the analyzer
func NewAnalyzerAuthenticationOpts() *shttp.AuthenticationOpts {
	return &shttp.AuthenticationOpts{
		Username: config.GetConfig().GetString("auth.analyzer_username"),
		Password: config.GetConfig().GetString("auth.analyzer_password"),
	}
}
