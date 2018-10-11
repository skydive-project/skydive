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
	"github.com/skydive-project/skydive/packetinjector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/enhancers"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	"github.com/skydive-project/skydive/ui"
	ws "github.com/skydive-project/skydive/websocket"
)

// ElectionStatus describes the status of an election
type ElectionStatus struct {
	IsMaster bool
}

// PeersStatus describes the state of a peer
type PeersStatus struct {
	Incomers map[string]ws.ConnStatus
	Outgoers map[string]ws.ConnStatus
}

// Status describes the status of an analyzer
type Status struct {
	Agents      map[string]ws.ConnStatus
	Peers       PeersStatus
	Publishers  map[string]ws.ConnStatus
	Subscribers map[string]ws.ConnStatus
	Alerts      ElectionStatus
	Captures    ElectionStatus
	Probes      []string
}

// Server describes an Analyzer servers mechanism like http, websocket, topology, ondemand probes, ...
type Server struct {
	httpServer          *shttp.Server
	uiServer            *ui.Server
	agentWSServer       *ws.StructServer
	publisherWSServer   *ws.StructServer
	replicationWSServer *ws.StructServer
	subscriberWSServer  *ws.StructServer
	replicationEndpoint *TopologyReplicationEndpoint
	alertServer         *alert.Server
	onDemandClient      *ondemand.OnDemandProbeClient
	piClient            *packetinjector.Client
	topologyManager     *usertopology.TopologyManager
	flowServer          *FlowServer
	probeBundle         *probe.Bundle
	storage             storage.Storage
	embeddedEtcd        *etcd.EmbeddedEtcd
	etcdClient          *etcd.Client
	wgServers           sync.WaitGroup
}

// GetStatus returns the status of an analyzer
func (s *Server) GetStatus() interface{} {
	peersStatus := PeersStatus{
		Incomers: make(map[string]ws.ConnStatus),
		Outgoers: make(map[string]ws.ConnStatus),
	}

	for _, speaker := range s.replicationEndpoint.in.GetSpeakers() {
		peersStatus.Incomers[speaker.GetRemoteHost()] = speaker.GetStatus()
	}

	for _, speaker := range s.replicationEndpoint.out.GetSpeakers() {
		peersStatus.Outgoers[speaker.GetRemoteHost()] = speaker.GetStatus()
	}

	return &Status{
		Agents:      s.agentWSServer.GetStatus(),
		Peers:       peersStatus,
		Publishers:  s.publisherWSServer.GetStatus(),
		Subscribers: s.subscriberWSServer.GetStatus(),
		Alerts:      ElectionStatus{IsMaster: s.alertServer.IsMaster()},
		Captures:    ElectionStatus{IsMaster: s.onDemandClient.IsMaster()},
		Probes:      s.probeBundle.ActiveProbes(),
	}
}

// createStartupCapture creates capture based on preconfigured selected SubGraph
func (s *Server) createStartupCapture(ch *api.CaptureAPIHandler) error {
	gremlin := config.GetString("analyzer.startup.capture_gremlin")
	if gremlin == "" {
		return nil
	}

	bpf := config.GetString("analyzer.startup.capture_bpf")
	logging.GetLogger().Infof("Invoke capturing from the startup with gremlin: %s and BPF: %s", gremlin, bpf)
	capture := types.NewCapture(gremlin, bpf)
	capture.Type = "pcap"
	return ch.Create(capture)
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
	s.topologyManager.Start()
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
	s.topologyManager.Stop()
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

	var embeddedEtcd *etcd.EmbeddedEtcd
	var err error
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
			logging.GetLogger().Errorf("Etcd server not ready: %s", err)
			time.Sleep(time.Second)
		} else {
			break
		}
	}

	if err := config.InitRBAC(etcdClient.KeysAPI); err != nil {
		return nil, err
	}

	hserver, err := config.NewHTTPServer(common.AnalyzerService)
	if err != nil {
		return nil, err
	}

	uiServer := ui.NewServer(hserver, config.GetString("ui.extra_assets"))

	// add some global vars
	uiServer.AddGlobalVar("ui", config.Get("ui"))
	uiServer.AddGlobalVar("flow-metric-keys", (&flow.FlowMetric{}).GetFields())
	uiServer.AddGlobalVar("interface-metric-keys", (&topology.InterfaceMetric{}).GetFields())
	uiServer.AddGlobalVar("probes", config.Get("analyzer.topology.probes"))

	name := config.GetString("analyzer.topology.backend")
	if len(name) == 0 {
		name = "memory"
	}

	persistent, err := graph.NewBackendByName(name, etcdClient)
	if err != nil {
		return nil, err
	}

	cached, err := graph.NewCachedBackend(persistent)
	if err != nil {
		return nil, err
	}

	g := graph.NewGraphFromConfig(cached, common.AnalyzerService)

	clusterAuthOptions := ClusterAuthenticationOpts()

	clusterAuthBackendName := config.GetString("analyzer.auth.cluster.backend")
	clusterAuthBackend, err := config.NewAuthenticationBackendByName(clusterAuthBackendName)
	if err != nil {
		return nil, err
	}
	// force admin user for the cluster backend to ensure that all the user connection through
	// "cluster" endpoints will be admin
	clusterAuthBackend.SetDefaultUserRole("admin")

	apiAuthBackendName := config.GetString("analyzer.auth.api.backend")
	apiAuthBackend, err := config.NewAuthenticationBackendByName(apiAuthBackendName)
	if err != nil {
		return nil, err
	}

	uiServer.RegisterLoginRoute(apiAuthBackend)

	agentWSServer := ws.NewStructServer(config.NewWSServer(hserver, "/ws/agent", clusterAuthBackend))
	_, err = NewTopologyAgentEndpoint(agentWSServer, cached, g)
	if err != nil {
		return nil, err
	}

	publisherWSServer := ws.NewStructServer(config.NewWSServer(hserver, "/ws/publisher", apiAuthBackend))
	_, err = NewTopologyPublisherEndpoint(publisherWSServer, g)
	if err != nil {
		return nil, err
	}

	tableClient := flow.NewWSTableClient(agentWSServer)

	storage, err := storage.NewStorageFromConfig(etcdClient)

	replicationWSServer := ws.NewStructServer(config.NewWSServer(hserver, "/ws/replication", clusterAuthBackend))
	replicationEndpoint, err := NewTopologyReplicationEndpoint(replicationWSServer, clusterAuthOptions, cached, g)
	if err != nil {
		return nil, err
	}

	// declare all extension available through API and filtering
	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewMetricsTraversalExtension())
	tr.AddTraversalExtension(ge.NewRawPacketsTraversalExtension())
	tr.AddTraversalExtension(ge.NewFlowTraversalExtension(tableClient, storage))
	tr.AddTraversalExtension(ge.NewSocketsTraversalExtension())
	tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())

	subscriberWSServer := ws.NewStructServer(config.NewWSServer(hserver, "/ws/subscriber", apiAuthBackend))
	topology.NewSubscriberEndpoint(subscriberWSServer, g, tr)

	probeBundle, err := NewTopologyProbeBundleFromConfig(g)
	if err != nil {
		return nil, err
	}

	apiServer, err := api.NewAPI(hserver, etcdClient.KeysAPI, common.AnalyzerService, apiAuthBackend)
	if err != nil {
		return nil, err
	}

	captureAPIHandler, err := api.RegisterCaptureAPI(apiServer, g, apiAuthBackend)
	if err != nil {
		return nil, err
	}

	piAPIHandler, err := api.RegisterPacketInjectorAPI(g, apiServer, apiAuthBackend)
	if err != nil {
		return nil, err
	}
	piClient := packetinjector.NewClient(agentWSServer, etcdClient, piAPIHandler, g)

	nodeAPIHandler, err := api.RegisterNodeRuleAPI(apiServer, g, apiAuthBackend)
	if err != nil {
		return nil, err
	}
	edgeAPIHandler, err := api.RegisterEdgeRuleAPI(apiServer, g, apiAuthBackend)
	if err != nil {
		return nil, err
	}
	topologyManager := usertopology.NewTopologyManager(etcdClient, nodeAPIHandler, edgeAPIHandler, g)

	if _, err = api.RegisterAlertAPI(apiServer, apiAuthBackend); err != nil {
		return nil, err
	}

	if _, err := api.RegisterWorkflowAPI(apiServer, apiAuthBackend); err != nil {
		return nil, err
	}

	onDemandClient := ondemand.NewOnDemandProbeClient(g, captureAPIHandler, agentWSServer, subscriberWSServer, etcdClient)

	flowServer, err := NewFlowServer(hserver, g, storage, probeBundle, clusterAuthBackend)
	if err != nil {
		return nil, err
	}

	alertServer, err := alert.NewServer(apiServer, subscriberWSServer, g, tr, etcdClient)
	if err != nil {
		return nil, err
	}

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
		topologyManager:     topologyManager,
		storage:             storage,
		flowServer:          flowServer,
		alertServer:         alertServer,
	}

	s.createStartupCapture(captureAPIHandler)

	api.RegisterTopologyAPI(hserver, g, tr, apiAuthBackend)
	api.RegisterPcapAPI(hserver, storage, apiAuthBackend)
	api.RegisterConfigAPI(hserver, apiAuthBackend)
	api.RegisterStatusAPI(hserver, s, apiAuthBackend)

	if config.GetBool("analyzer.ssh_enabled") {
		if err := dede.RegisterHandler("terminal", "/dede", hserver.Router); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// ClusterAuthenticationOpts returns auth info to connect to an analyzer
// from the configuration
func ClusterAuthenticationOpts() *shttp.AuthenticationOpts {
	return &shttp.AuthenticationOpts{
		Username: config.GetString("analyzer.auth.cluster.username"),
		Password: config.GetString("analyzer.auth.cluster.password"),
		Cookie:   config.GetStringMapString("http.cookie"),
	}
}
