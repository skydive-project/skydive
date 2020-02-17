/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package analyzer

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/skydive-project/dede/dede"
	"github.com/skydive-project/skydive/alert"
	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/client"
	"github.com/skydive-project/skydive/flow/server"
	"github.com/skydive-project/skydive/flow/storage"
	etcdclient "github.com/skydive-project/skydive/graffiti/etcd/client"
	etcdserver "github.com/skydive-project/skydive/graffiti/etcd/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/hub"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/ondemand/client"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/packetinjector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/sflow"
	"github.com/skydive-project/skydive/topology"
	usertopology "github.com/skydive-project/skydive/topology/enhancers"
	"github.com/skydive-project/skydive/topology/probes/blockdev"
	"github.com/skydive-project/skydive/ui"
)

// ElectionStatus describes the status of an election
type ElectionStatus struct {
	IsMaster bool
}

// Status analyzer object
//
// Status describes the status of an analyzer
//
// swagger:model AnalyzerStatus
type Status struct {
	Agents      map[string]ws.ConnStatus
	Peers       hub.PeersStatus
	Publishers  map[string]ws.ConnStatus
	Subscribers map[string]ws.ConnStatus
	Alerts      ElectionStatus
	Captures    ElectionStatus
	Probes      map[string]interface{}
}

// Server describes an Analyzer servers mechanism like http, websocket, topology, ondemand probes, ...
type Server struct {
	uiServer        *ui.Server
	hub             *hub.Hub
	alertServer     *alert.Server
	onDemandClient  *client.OnDemandClient
	piClient        *client.OnDemandClient
	topologyManager *usertopology.TopologyManager
	flowServer      *server.FlowServer
	probeBundle     *probe.Bundle
	storage         storage.Storage
	etcdClient      *etcdclient.Client
}

// GetStatus returns the status of an analyzer
func (s *Server) GetStatus() interface{} {
	hubStatus := s.hub.GetStatus().(*hub.Status)

	return &Status{
		Agents:      hubStatus.Pods,
		Peers:       hubStatus.Peers,
		Publishers:  hubStatus.Publishers,
		Subscribers: hubStatus.Subscribers,
		Alerts:      ElectionStatus{IsMaster: s.alertServer.IsMaster()},
		Captures:    ElectionStatus{IsMaster: s.onDemandClient.IsMaster()},
		Probes:      s.probeBundle.GetStatus(),
	}
}

// createStartupCapture creates capture based on preconfigured selected SubGraph
func (s *Server) createStartupCapture(ch *api.CaptureAPIHandler) error {
	gremlin := config.GetString("analyzer.startup.capture_gremlin")
	if gremlin == "" {
		return nil
	}

	bpf := config.GetString("analyzer.startup.capture_bpf")
	captureType := config.GetString("analyzer.startup.capture_type")
	logging.GetLogger().Infof("Invoke capturing of type '%s' from startup with gremlin: %s and BPF: %s", captureType, gremlin, bpf)
	capture := types.NewCapture(gremlin, bpf)
	capture.Type = captureType
	return ch.Create(capture, nil)
}

// Start the analyzer server
func (s *Server) Start() error {
	if s.storage != nil {
		s.storage.Start()
	}

	if err := s.hub.Start(); err != nil {
		return err
	}

	s.etcdClient.Start()

	if err := s.probeBundle.Start(); err != nil {
		return err
	}

	s.onDemandClient.Start()
	s.piClient.Start()
	s.alertServer.Start()
	s.topologyManager.Start()
	s.flowServer.Start()

	return nil
}

// Stop the analyzer server
func (s *Server) Stop() {
	s.hub.Stop()
	s.flowServer.Stop()
	s.probeBundle.Stop()
	s.onDemandClient.Stop()
	s.piClient.Stop()
	s.alertServer.Stop()
	s.topologyManager.Stop()
	s.etcdClient.Stop()
	if s.storage != nil {
		s.storage.Stop()
	}
	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
}

// NewServerFromConfig creates a new empty server
func NewServerFromConfig() (*Server, error) {
	embedEtcd := config.GetBool("etcd.embedded")
	host := config.GetString("host_id")
	service := common.Service{ID: host, Type: common.AnalyzerService}

	var etcdServerOpts *etcdserver.EmbeddedServerOpts
	var err error
	if embedEtcd {
		etcdServerOpts = &etcdserver.EmbeddedServerOpts{
			Name:         config.GetString("etcd.name"),
			Listen:       config.GetString("etcd.listen"),
			DataDir:      config.GetString("etcd.data_dir"),
			MaxWalFiles:  uint(config.GetInt("etcd.max_wal_files")),
			MaxSnapFiles: uint(config.GetInt("etcd.max_snap_files")),
			Debug:        config.GetBool("etcd.debug"),
			Peers:        config.GetStringMapString("etcd.peers"),
		}
	}

	etcdClientOpts := etcdclient.ClientOpts{
		Servers: config.GetEtcdServerAddrs(),
		Timeout: time.Duration(config.GetInt("etcd.client_timeout")) * time.Second,
	}

	etcdClient, err := etcdclient.NewClient(service, etcdClientOpts)
	if err != nil {
		return nil, err
	}

	if err := config.InitRBAC(etcdClient.KeysAPI); err != nil {
		return nil, err
	}

	var tlsConfig *tls.Config
	if config.IsTLSEnabled() {
		tlsConfig, err = config.GetTLSServerConfig(true)
		if err != nil {
			return nil, err
		}
	}

	persistent, err := newGraphBackendFromConfig(etcdClient)
	if err != nil {
		return nil, err
	}

	cached, err := graph.NewCachedBackend(persistent)
	if err != nil {
		return nil, err
	}

	g := graph.NewGraph(host, cached, service.Type)

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

	peers, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err)
	}

	validator, err := topology.NewSchemaValidator()
	if err != nil {
		return nil, fmt.Errorf("Unable to instantiate a schema validator: %s", err)
	}

	opts := hub.Opts{
		WebsocketOpts: ws.ServerOpts{
			WriteCompression: true,
			QueueSize:        10000,
			PingDelay:        2 * time.Second,
			PongTimeout:      5 * time.Second,
		},
		APIAuthBackend:     apiAuthBackend,
		ClusterAuthBackend: clusterAuthBackend,
		ClusterAuthOptions: ClusterAuthenticationOpts(),
		Validator:          validator,
		TLSConfig:          tlsConfig,
		Peers:              peers,
		EtcdKeysAPI:        etcdClient.KeysAPI,
		EtcdServerOpts:     etcdServerOpts,
	}

	listenAddr := config.GetString("analyzer.listen")
	hub, err := hub.NewHub(host, common.AnalyzerService, listenAddr, g, cached, "/ws/agent/topology", opts)
	if err != nil {
		return nil, err
	}

	// Instantiate Web UI
	uiServer := ui.NewServer(hub.HTTPServer(), config.GetString("ui.extra_assets"))

	// add some global vars
	uiServer.AddGlobalVar("ui", config.Get("ui"))
	uiServer.AddGlobalVar("flow-metric-keys", (&flow.FlowMetric{}).GetFieldKeys())
	uiServer.AddGlobalVar("interface-metric-keys", (&topology.InterfaceMetric{}).GetFieldKeys())
	uiServer.AddGlobalVar("blockdev-metric-keys", (&blockdev.BlockMetric{}).GetFieldKeys())
	uiServer.AddGlobalVar("sflow-metric-keys", (&sflow.SFMetric{}).GetFieldKeys())
	uiServer.AddGlobalVar("probes", config.Get("analyzer.topology.probes"))

	storage, err := newFlowBackendFromConfig(etcdClient)
	if err != nil {
		return nil, err
	}

	tableClient := flow.NewWSTableClient(hub.PodServer())

	// declare all extension available through API and filtering
	tr := hub.GremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewMetricsTraversalExtension())
	tr.AddTraversalExtension(ge.NewRawPacketsTraversalExtension())
	tr.AddTraversalExtension(ge.NewSocketsTraversalExtension())
	tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())
	tr.AddTraversalExtension(ge.NewNextHopTraversalExtension())
	tr.AddTraversalExtension(ge.NewGroupTraversalExtension())

	tr.AddTraversalExtension(ge.NewFlowTraversalExtension(tableClient, storage))

	probeBundle, err := NewTopologyProbeBundleFromConfig(g)
	if err != nil {
		return nil, err
	}

	// new flow subscriber endpoints
	flowSubscriberWSServer := ws.NewStructServer(config.NewWSServer(hub.HTTPServer(), "/ws/subscriber/flow", apiAuthBackend))
	flowSubscriberEndpoint := server.NewFlowSubscriberEndpoint(flowSubscriberWSServer)

	captureAPIHandler, err := api.RegisterCaptureAPI(hub.APIServer(), g, apiAuthBackend)
	if err != nil {
		return nil, err
	}

	apiServer := hub.APIServer()
	piAPIHandler, err := api.RegisterPacketInjectorAPI(g, apiServer, apiAuthBackend)
	if err != nil {
		return nil, err
	}

	piClient := packetinjector.NewOnDemandInjectionClient(g, piAPIHandler, hub.PodServer(), hub.SubscriberServer(), etcdClient)

	nodeRuleAPIHandler, err := api.RegisterNodeRuleAPI(apiServer, g, apiAuthBackend)
	if err != nil {
		return nil, err
	}

	edgeRuleAPIHandler, err := api.RegisterEdgeRuleAPI(apiServer, g, apiAuthBackend)
	if err != nil {
		return nil, err
	}

	topologyManager := usertopology.NewTopologyManager(etcdClient, nodeRuleAPIHandler, edgeRuleAPIHandler, g)

	if _, err = api.RegisterAlertAPI(apiServer, apiAuthBackend); err != nil {
		return nil, err
	}

	if _, err := api.RegisterWorkflowAPI(apiServer, apiAuthBackend); err != nil {
		return nil, err
	}

	onDemandClient := ondemand.NewOnDemandFlowProbeClient(g, captureAPIHandler, hub.PodServer(), hub.SubscriberServer(), etcdClient)

	flowServer, err := server.NewFlowServer(hub.HTTPServer(), g, storage, flowSubscriberEndpoint, probeBundle, clusterAuthBackend)
	if err != nil {
		return nil, err
	}

	alertServer, err := alert.NewServer(apiServer, hub.SubscriberServer(), g, tr, etcdClient)
	if err != nil {
		return nil, err
	}

	s := &Server{
		hub:             hub,
		probeBundle:     probeBundle,
		etcdClient:      etcdClient,
		onDemandClient:  onDemandClient,
		piClient:        piClient,
		topologyManager: topologyManager,
		storage:         storage,
		flowServer:      flowServer,
		alertServer:     alertServer,
	}

	s.createStartupCapture(captureAPIHandler)

	httpServer := hub.HTTPServer()
	api.RegisterPcapAPI(httpServer, storage, apiAuthBackend)
	api.RegisterConfigAPI(httpServer, apiAuthBackend)
	api.RegisterStatusAPI(httpServer, s, apiAuthBackend)
	api.RegisterWorkflowCallAPI(httpServer, apiAuthBackend, apiServer, g, tr)

	if config.GetBool("analyzer.ssh_enabled") {
		if err := dede.RegisterHandler("terminal", "/dede", httpServer.Router); err != nil {
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
