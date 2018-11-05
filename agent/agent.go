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
	"net/http"
	"os"
	"time"

	"github.com/skydive-project/skydive/analyzer"
	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/server"
	fprobes "github.com/skydive-project/skydive/flow/probes"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/packetinjector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	"github.com/skydive-project/skydive/ui"
	ws "github.com/skydive-project/skydive/websocket"
)

// Agent object started on each hosts/namespaces
type Agent struct {
	ws.DefaultSpeakerEventHandler
	graph               *graph.Graph
	wsServer            *ws.StructServer
	analyzerClientPool  *ws.StructClientPool
	topologyEndpoint    *topology.SubscriberEndpoint
	rootNode            *graph.Node
	topologyProbeBundle *probe.Bundle
	flowProbeBundle     *probe.Bundle
	flowTableAllocator  *flow.TableAllocator
	flowClientPool      *analyzer.FlowClientPool
	onDemandProbeServer *ondemand.OnDemandProbeServer
	httpServer          *shttp.Server
	tidMapper           *topology.TIDMapper
	topologyForwarder   *TopologyForwarder
}

// NewAnalyzerStructClientPool creates a new http WebSocket client Pool
// with authentification
func NewAnalyzerStructClientPool(authOptions *shttp.AuthenticationOpts) (*ws.StructClientPool, error) {
	pool := ws.NewStructClientPool("AnalyzerClientPool")

	addresses, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err)
	}

	if len(addresses) == 0 {
		logging.GetLogger().Info("Agent is running in standalone mode")
		return pool, nil
	}

	for _, sa := range addresses {
		c, err := config.NewWSClient(common.AgentService, config.GetURL("ws", sa.Addr, sa.Port, "/ws/agent"), authOptions, nil)
		if err != nil {
			return nil, err
		}
		pool.AddClient(c)
	}

	return pool, nil
}

// AnalyzerConnStatus represents the status of a connection to an analyzer
type AnalyzerConnStatus struct {
	ws.ConnStatus
	IsMaster bool
}

// Status represents the status of an agent
type Status struct {
	Clients        map[string]ws.ConnStatus
	Analyzers      map[string]AnalyzerConnStatus
	TopologyProbes []string
	FlowProbes     []string
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
			ConnStatus: status,
			IsMaster:   status.Addr == masterAddr && status.Port == masterPort,
		}
	}

	return &Status{
		Clients:        a.wsServer.GetStatus(),
		Analyzers:      analyzers,
		TopologyProbes: a.topologyProbeBundle.ActiveProbes(),
		FlowProbes:     a.flowProbeBundle.ActiveProbes(),
	}
}

// Start the agent services
func (a *Agent) Start() {
	if uid := os.Geteuid(); uid != 0 {
		logging.GetLogger().Warning("Agent needs root permissions for some feature like capture, network namespace introspection, some feature might not work as expected")
	}

	go a.httpServer.Serve()

	a.wsServer.Start()
	a.topologyProbeBundle.Start()
	a.flowProbeBundle.Start()
	a.onDemandProbeServer.Start()

	// everything is ready, then initiate the websocket connection
	go a.analyzerClientPool.ConnectAll()
}

// Stop agent services
func (a *Agent) Stop() {
	a.flowProbeBundle.Stop()
	a.analyzerClientPool.Stop()
	a.topologyProbeBundle.Stop()
	a.httpServer.Stop()
	a.wsServer.Stop()
	a.flowClientPool.Close()
	a.onDemandProbeServer.Stop()

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

	g := graph.NewGraphFromConfig(backend, common.AgentService)

	tm := topology.NewTIDMapper(g)
	tm.Start()

	apiAuthBackendName := config.GetString("agent.auth.api.backend")
	apiAuthBackend, err := config.NewAuthenticationBackendByName(apiAuthBackendName)
	if err != nil {
		return nil, err
	}

	hserver, err := config.NewHTTPServer(common.AgentService)
	if err != nil {

		return nil, err
	}

	uiServer := ui.NewServer(hserver, config.GetString("ui.extra_assets"))
	uiServer.RegisterLoginRoute(apiAuthBackend)

	if err = hserver.Listen(); err != nil {
		return nil, err
	}

	if _, err = api.NewAPI(hserver, nil, common.AgentService, apiAuthBackend); err != nil {
		return nil, err
	}

	wsServer := ws.NewStructServer(config.NewWSServer(hserver, "/ws/subscriber", apiAuthBackend))

	// declare all extension available throught API and filtering
	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewMetricsTraversalExtension())
	tr.AddTraversalExtension(ge.NewSocketsTraversalExtension())
	tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())

	rootNode, err := createRootNode(g)
	if err != nil {
		return nil, err
	}

	api.RegisterTopologyAPI(hserver, g, tr, apiAuthBackend)

	clusterAuthOptions := &shttp.AuthenticationOpts{
		Username: config.GetString("agent.auth.cluster.username"),
		Password: config.GetString("agent.auth.cluster.password"),
		Cookie:   config.GetStringMapString("http.cookie"),
	}

	topologyEndpoint := topology.NewSubscriberEndpoint(wsServer, g, tr)

	analyzerClientPool, err := NewAnalyzerStructClientPool(clusterAuthOptions)
	if err != nil {
		return nil, err
	}

	tforwarder := NewTopologyForwarderFromConfig(g, analyzerClientPool)

	topologyProbeBundle, err := NewTopologyProbeBundleFromConfig(g, rootNode)
	if err != nil {
		return nil, err
	}

	updateTime := time.Duration(config.GetInt("flow.update")) * time.Second
	expireTime := time.Duration(config.GetInt("flow.expire")) * time.Second

	flowTableAllocator := flow.NewTableAllocator(updateTime, expireTime)

	// exposes a flow server through the client connections
	flow.NewWSTableServer(flowTableAllocator, analyzerClientPool)

	packetinjector.NewServer(g, analyzerClientPool)

	flowClientPool := analyzer.NewFlowClientPool(analyzerClientPool, clusterAuthOptions)

	flowProbeBundle := fprobes.NewFlowProbeBundle(topologyProbeBundle, g, flowTableAllocator, flowClientPool)

	onDemandProbeServer, err := ondemand.NewOnDemandProbeServer(flowProbeBundle, g, analyzerClientPool)
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize on-demand flow probe %s", err)
	}

	agent := &Agent{
		graph:               g,
		wsServer:            wsServer,
		analyzerClientPool:  analyzerClientPool,
		topologyEndpoint:    topologyEndpoint,
		rootNode:            rootNode,
		topologyProbeBundle: topologyProbeBundle,
		flowProbeBundle:     flowProbeBundle,
		flowTableAllocator:  flowTableAllocator,
		flowClientPool:      flowClientPool,
		onDemandProbeServer: onDemandProbeServer,
		httpServer:          hserver,
		tidMapper:           tm,
		topologyForwarder:   tforwarder,
	}

	api.RegisterStatusAPI(hserver, agent, apiAuthBackend)

	return agent, nil
}
