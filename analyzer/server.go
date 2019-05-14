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
	"github.com/skydive-project/skydive/flow/server"
	"github.com/skydive-project/skydive/flow/storage"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/hub"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/packetinjector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/sflow"
	"github.com/skydive-project/skydive/topology"
	usertopology "github.com/skydive-project/skydive/topology/enhancers"
	"github.com/skydive-project/skydive/topology/probes/docker"
	"github.com/skydive-project/skydive/topology/probes/libvirt"
	"github.com/skydive-project/skydive/topology/probes/lldp"
	"github.com/skydive-project/skydive/topology/probes/lxd"
	"github.com/skydive-project/skydive/topology/probes/netlink"
	"github.com/skydive-project/skydive/topology/probes/neutron"
	"github.com/skydive-project/skydive/topology/probes/opencontrail"
	"github.com/skydive-project/skydive/topology/probes/ovn"
	"github.com/skydive-project/skydive/topology/probes/ovsdb"
	"github.com/skydive-project/skydive/topology/probes/runc"
	"github.com/skydive-project/skydive/ui"
	"github.com/skydive-project/skydive/websocket"
	ws "github.com/skydive-project/skydive/websocket"
)

// ElectionStatus describes the status of an election
type ElectionStatus struct {
	IsMaster bool
}

// Status describes the status of an analyzer
type Status struct {
	Agents      map[string]ws.ConnStatus
	Peers       hub.PeersStatus
	Publishers  map[string]ws.ConnStatus
	Subscribers map[string]ws.ConnStatus
	Alerts      ElectionStatus
	Captures    ElectionStatus
	Probes      []string
}

// Server describes an Analyzer servers mechanism like http, websocket, topology, ondemand probes, ...
type Server struct {
	httpServer      *shttp.Server
	uiServer        *ui.Server
	hub             *hub.Hub
	alertServer     *alert.Server
	onDemandClient  *ondemand.OnDemandProbeClient
	piClient        *packetinjector.Client
	topologyManager *usertopology.TopologyManager
	flowServer      *server.FlowServer
	probeBundle     *probe.Bundle
	storage         storage.Storage
	embeddedEtcd    *etcd.EmbeddedEtcd
	etcdClient      *etcd.Client
	wgServers       sync.WaitGroup
}

// GetStatus returns the status of an analyzer
func (s *Server) GetStatus() interface{} {
	hubStatus := s.hub.GetStatus()

	return &Status{
		Agents:      hubStatus.Pods,
		Peers:       hubStatus.Peers,
		Publishers:  hubStatus.Publishers,
		Subscribers: hubStatus.Subscribers,
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

	s.hub.Start()
	s.probeBundle.Start()
	s.onDemandClient.Start()
	s.piClient.Start()
	s.alertServer.Start()
	s.topologyManager.Start()
	s.flowServer.Start()

	s.wgServers.Add(1)
	go func() {
		defer s.wgServers.Done()
		s.httpServer.Serve()
	}()

	return nil
}

// Stop the analyzer server
func (s *Server) Stop() {
	s.hub.Stop()
	s.flowServer.Stop()
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
	host := config.GetString("host_id")

	var embeddedEtcd *etcd.EmbeddedEtcd
	var err error
	if embedEtcd {
		name := config.GetString("etcd.name")
		dataDir := config.GetString("etcd.data_dir")
		listen := config.GetString("etcd.listen")
		maxWalFiles := uint(config.GetInt("etcd.max_wal_files"))
		maxSnapFiles := uint(config.GetInt("etcd.max_snap_files"))
		debug := config.GetBool("etcd.debug")
		peers := config.GetStringMapString("etcd.peers")

		if embeddedEtcd, err = etcd.NewEmbeddedEtcd(name, listen, peers, dataDir, maxWalFiles, maxSnapFiles, debug); err != nil {
			return nil, err
		}
	}

	service := common.Service{ID: host, Type: common.AnalyzerService}

	etcdServers := config.GetEtcdServerAddrs()
	etcdTimeout := config.GetInt("etcd.client_timeout")
	etcdClient, err := etcd.NewClient(service, etcdServers, time.Duration(etcdTimeout)*time.Second)
	if err != nil {
		return nil, err
	}

	// wait for etcd to be ready
	for {
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

	hserver, err := config.NewHTTPServer(service.Type)
	if err != nil {
		return nil, err
	}

	uiServer := ui.NewServer(hserver, config.GetString("ui.extra_assets"))

	// add some global vars
	uiServer.AddGlobalVar("ui", config.Get("ui"))
	uiServer.AddGlobalVar("flow-metric-keys", (&flow.FlowMetric{}).GetFieldKeys())
	uiServer.AddGlobalVar("interface-metric-keys", (&topology.InterfaceMetric{}).GetFieldKeys())
	uiServer.AddGlobalVar("sflow-metric-keys", (&sflow.SFMetric{}).GetFieldKeys())
	uiServer.AddGlobalVar("probes", config.Get("analyzer.topology.probes"))

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

	uiServer.RegisterLoginRoute(apiAuthBackend)

	peers, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err)
	}

	validator, err := topology.NewSchemaValidator()
	if err != nil {
		return nil, fmt.Errorf("Unable to instantiate a schema validator: %s", err)
	}

	opts := hub.Opts{
		ServerOpts: websocket.ServerOpts{
			WriteCompression: true,
			QueueSize:        10000,
			PingDelay:        2 * time.Second,
			PongTimeout:      5 * time.Second,
		},
		Validator: validator,
	}

	clusterAuthOptions := ClusterAuthenticationOpts()
	hub, err := hub.NewHub(hserver, g, cached, apiAuthBackend, clusterAuthBackend, clusterAuthOptions, "/ws/agent/topology", peers, opts)
	if err != nil {
		return nil, err
	}

	tableClient := flow.NewWSTableClient(hub.PodServer())

	storage, err := newFlowBackendFromConfig(etcdClient)
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
	tr.AddTraversalExtension(ge.NewNextHopTraversalExtension())

	probeBundle, err := NewTopologyProbeBundleFromConfig(g)
	if err != nil {
		return nil, err
	}

	// new flow subscriber endpoints
	flowSubscriberWSServer := ws.NewStructServer(config.NewWSServer(hserver, "/ws/subscriber/flow", apiAuthBackend))
	flowSubscriberEndpoint := server.NewFlowSubscriberEndpoint(flowSubscriberWSServer)

	apiServer, err := api.NewAPI(hserver, etcdClient.KeysAPI, service, apiAuthBackend)
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
	piClient := packetinjector.NewClient(hub.PodServer(), etcdClient, piAPIHandler, g)

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

	onDemandClient := ondemand.NewOnDemandProbeClient(g, captureAPIHandler, hub.PodServer(), hub.SubscriberServer(), etcdClient)

	flowServer, err := server.NewFlowServer(hserver, g, storage, flowSubscriberEndpoint, probeBundle, clusterAuthBackend)
	if err != nil {
		return nil, err
	}

	alertServer, err := alert.NewServer(apiServer, hub.SubscriberServer(), g, tr, etcdClient)
	if err != nil {
		return nil, err
	}

	s := &Server{
		httpServer:      hserver,
		hub:             hub,
		probeBundle:     probeBundle,
		embeddedEtcd:    embeddedEtcd,
		etcdClient:      etcdClient,
		onDemandClient:  onDemandClient,
		piClient:        piClient,
		topologyManager: topologyManager,
		storage:         storage,
		flowServer:      flowServer,
		alertServer:     alertServer,
	}

	s.createStartupCapture(captureAPIHandler)

	api.RegisterTopologyAPI(hserver, g, tr, apiAuthBackend)
	api.RegisterPcapAPI(hserver, storage, apiAuthBackend)
	api.RegisterConfigAPI(hserver, apiAuthBackend)
	api.RegisterStatusAPI(hserver, s, apiAuthBackend)
	api.RegisterWorkflowCallAPI(hserver, apiAuthBackend, apiServer, g, tr)

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

func init() {
	// add decoders for specific metadata keys, this aims to keep the same
	// object type between the agent and the analyzer
	// Decoder will be used while unmarshal the metadata
	graph.NodeMetadataDecoders["RoutingTables"] = netlink.RoutingTablesMetadataDecoder
	graph.NodeMetadataDecoders["FDB"] = netlink.NeighborMetadataDecoder
	graph.NodeMetadataDecoders["Neighbors"] = netlink.NeighborMetadataDecoder
	graph.NodeMetadataDecoders["Vfs"] = netlink.VFSMetadataDecoder
	graph.NodeMetadataDecoders["Metric"] = topology.InterfaceMetricMetadataDecoder
	graph.NodeMetadataDecoders["LastUpdateMetric"] = topology.InterfaceMetricMetadataDecoder
	graph.NodeMetadataDecoders["SFlow"] = sflow.SFMetadataDecoder
	graph.NodeMetadataDecoders["Ovs"] = ovsdb.OvsMetadataDecoder
	graph.NodeMetadataDecoders["LLDP"] = lldp.MetadataDecoder
	graph.NodeMetadataDecoders["Docker"] = docker.MetadataDecoder
	graph.NodeMetadataDecoders["Lxd"] = lxd.MetadataDecoder
	graph.NodeMetadataDecoders["Runc"] = runc.MetadataDecoder
	graph.NodeMetadataDecoders["Libvirt"] = libvirt.MetadataDecoder
	graph.NodeMetadataDecoders["OVN"] = ovn.MetadataDecoder
	graph.NodeMetadataDecoders["Neutron"] = neutron.MetadataDecoder
	graph.NodeMetadataDecoders["Contrail"] = opencontrail.MetadataDecoder
}
