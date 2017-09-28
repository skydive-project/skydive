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

package agent

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pmylund/go-cache"
	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/enhancers"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/server"
	fprobes "github.com/skydive-project/skydive/flow/probes"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/packet_injector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

// Agent object started on each hosts/namespaces
type Agent struct {
	shttp.DefaultWSSpeakerEventHandler
	graph               *graph.Graph
	wsServer            *shttp.WSJSONServer
	analyzerClientPool  *shttp.WSJSONClientPool
	topologyServer      *TopologyServer
	rootNode            *graph.Node
	topologyProbeBundle *probe.ProbeBundle
	flowProbeBundle     *fprobes.FlowProbeBundle
	flowTableAllocator  *flow.TableAllocator
	flowClientPool      *analyzer.FlowClientPool
	onDemandProbeServer *ondemand.OnDemandProbeServer
	httpServer          *shttp.Server
	etcdClient          *etcd.EtcdClient
	tidMapper           *topology.TIDMapper
	topologyForwarder   *TopologyForwarder
}

// NewAnalyzerWSJSONClientPool creates a new http WebSocket client Pool
// with authentification
func NewAnalyzerWSJSONClientPool() (*shttp.WSJSONClientPool, error) {
	pool := shttp.NewWSJSONClientPool()
	authOptions := analyzer.NewAnalyzerAuthenticationOpts()

	addresses, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err)
	}

	for _, sa := range addresses {
		authClient := shttp.NewAuthenticationClient(sa.Addr, sa.Port, authOptions)
		c := shttp.NewWSClientFromConfig(common.AgentService, sa.Addr, sa.Port, "/ws", authClient)
		pool.AddClient(c)
	}

	return pool, nil
}

// AnalyzerConnStatus represents the status of a connection to an analyzer
type AnalyzerConnStatus struct {
	shttp.WSConnStatus
	IsMaster bool
}

// AgentStatus represents the status of an agent
type AgentStatus struct {
	Clients   map[string]shttp.WSConnStatus
	Analyzers map[string]AnalyzerConnStatus
}

// GetStatus returns the status of an agent
func (a *Agent) GetStatus() interface{} {
	var masterAddr string
	var masterPort int
	if master := a.topologyForwarder.GetMaster(); master != nil {
		masterAddr, masterPort = master.GetAddrPort()
	}

	analyzers := make(map[string]AnalyzerConnStatus)
	for id, status := range a.analyzerClientPool.GetStatus() {
		analyzers[id] = AnalyzerConnStatus{
			WSConnStatus: status,
			IsMaster:     status.Addr == masterAddr && status.Port == masterPort,
		}
	}

	return &AgentStatus{
		Clients:   a.wsServer.GetStatus(),
		Analyzers: analyzers,
	}
}

// Start the agent services
func (a *Agent) Start() {
	var err error

	if err := a.httpServer.Listen(); err != nil {
		logging.GetLogger().Error(err)
		os.Exit(1)
	}

	go a.httpServer.Serve()
	a.wsServer.Start()

	a.topologyProbeBundle, err = NewTopologyProbeBundleFromConfig(a.graph, a.rootNode)
	if err != nil {
		logging.GetLogger().Errorf("Unable to instantiate topology probes: %s", err.Error())
		os.Exit(1)
	}
	a.topologyProbeBundle.Start()

	updateTime := time.Duration(config.GetConfig().GetInt("flow.update")) * time.Second
	expireTime := time.Duration(config.GetConfig().GetInt("flow.expire")) * time.Second
	cleanup := time.Duration(config.GetConfig().GetInt("cache.cleanup")) * time.Second

	cache := cache.New(expireTime*2, cleanup)

	pipeline := flow.NewEnhancerPipeline(
		enhancers.NewGraphFlowEnhancer(a.graph, cache),
		enhancers.NewSocketInfoEnhancer(expireTime*2, cleanup),
	)

	// check that the neutron probe if loaded if so add the neutron flow enhancer
	if a.topologyProbeBundle.GetProbe("neutron") != nil {
		pipeline.AddEnhancer(enhancers.NewNeutronFlowEnhancer(a.graph, cache))
	}

	a.flowTableAllocator = flow.NewTableAllocator(updateTime, expireTime, pipeline)

	// exposes a flow server through the client connections
	flow.NewServer(a.flowTableAllocator, a.analyzerClientPool)

	packet_injector.NewServer(a.graph, a.analyzerClientPool)

	a.flowClientPool = analyzer.NewFlowClientPool(a.analyzerClientPool)

	a.flowProbeBundle = fprobes.NewFlowProbeBundle(a.topologyProbeBundle, a.graph, a.flowTableAllocator, a.flowClientPool)
	a.flowProbeBundle.Start()

	if a.onDemandProbeServer, err = ondemand.NewOnDemandProbeServer(a.flowProbeBundle, a.graph, a.analyzerClientPool); err != nil {
		logging.GetLogger().Errorf("Unable to start on-demand flow probe %s", err.Error())
		os.Exit(1)
	}
	a.onDemandProbeServer.Start()

	// everything is ready, then initiate the websocket connection
	go a.analyzerClientPool.ConnectAll()
}

// Stop agent services
func (a *Agent) Stop() {
	if a.flowProbeBundle != nil {
		a.flowProbeBundle.Stop()
	}
	a.analyzerClientPool.Stop()
	a.topologyProbeBundle.Stop()
	a.httpServer.Stop()
	a.wsServer.Stop()

	if a.flowClientPool != nil {
		a.flowClientPool.Close()
	}
	if a.onDemandProbeServer != nil {
		a.onDemandProbeServer.Stop()
	}
	if a.etcdClient != nil {
		a.etcdClient.Stop()
	}
	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
	a.tidMapper.Stop()
}

// NewAgent instanciates a new Agent aiming to launch probes (topology and flow)
func NewAgent() (*Agent, error) {
	backend, err := graph.NewMemoryBackend()
	if err != nil {
		return nil, err
	}

	g := graph.NewGraphFromConfig(backend)

	tm := topology.NewTIDMapper(g)
	tm.Start()

	hserver, err := shttp.NewServerFromConfig(common.AgentService)
	if err != nil {
		return nil, err
	}

	if _, err = api.NewAPI(hserver, nil, common.AgentService); err != nil {
		return nil, err
	}

	wsServer := shttp.NewWSJSONServer(shttp.NewWSServerFromConfig(hserver, "/ws"))

	tr := traversal.NewGremlinTraversalParser(g)
	tr.AddTraversalExtension(topology.NewTopologyTraversalExtension())

	rootNode := CreateRootNode(g)
	api.RegisterTopologyAPI(hserver, tr)

	tserver := NewTopologyServer(g, wsServer)

	analyzerClientPool, err := NewAnalyzerWSJSONClientPool()
	if err != nil {
		return nil, err
	}

	tforwarder := NewTopologyForwarderFromConfig(g, analyzerClientPool)

	agent := &Agent{
		graph:              g,
		wsServer:           wsServer,
		topologyServer:     tserver,
		rootNode:           rootNode,
		httpServer:         hserver,
		tidMapper:          tm,
		topologyForwarder:  tforwarder,
		analyzerClientPool: analyzerClientPool,
	}

	api.RegisterStatusAPI(hserver, agent)

	return agent, nil
}

// CreateRootNode creates a graph.Node based on the host properties and aims to have an unique ID
func CreateRootNode(g *graph.Graph) *graph.Node {
	hostID := config.GetConfig().GetString("host_id")
	m := graph.Metadata{"Name": hostID, "Type": "host"}
	if config.GetConfig().IsSet("agent.metadata") {
		subtree := config.GetConfig().Sub("agent.metadata")
		for key, value := range subtree.AllSettings() {
			m[key] = value
		}
	}
	buffer, err := ioutil.ReadFile("/var/lib/cloud/data/instance-id")
	if err == nil {
		m["InstanceID"] = strings.TrimSpace(string(buffer))
	}
	return g.NewNode(graph.GenID(), m)
}
