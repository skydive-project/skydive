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
	"time"

	"github.com/skydive-project/skydive/analyzer"
	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/enhancers"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/server"
	fprobes "github.com/skydive-project/skydive/flow/probes"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
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
	wsServer            *shttp.WSStructServer
	analyzerClientPool  *shttp.WSStructClientPool
	topologyEndpoint    *topology.TopologySubscriberEndpoint
	rootNode            *graph.Node
	topologyProbeBundle *probe.ProbeBundle
	flowProbeBundle     *probe.ProbeBundle
	flowPipeline        *flow.EnhancerPipeline
	flowTableAllocator  *flow.TableAllocator
	flowClientPool      *analyzer.FlowClientPool
	onDemandProbeServer *ondemand.OnDemandProbeServer
	httpServer          *shttp.Server
	tidMapper           *topology.TIDMapper
	topologyForwarder   *TopologyForwarder
}

// NewAnalyzerWSStructClientPool creates a new http WebSocket client Pool
// with authentification
func NewAnalyzerWSStructClientPool(authOptions *shttp.AuthenticationOpts) (*shttp.WSStructClientPool, error) {
	pool := shttp.NewWSStructClientPool("AnalyzerClientPool")

	addresses, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err.Error())
	}

	if len(addresses) == 0 {
		logging.GetLogger().Info("Agent is running in standalone mode")
		return pool, nil
	}

	for _, sa := range addresses {
		authClient := shttp.NewAuthenticationClient(config.GetURL("http", sa.Addr, sa.Port, ""), authOptions)
		c := shttp.NewWSClientFromConfig(common.AgentService, config.GetURL("ws", sa.Addr, sa.Port, "/ws/agent"), authClient, nil)
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
	Clients        map[string]shttp.WSConnStatus
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
			WSConnStatus: status,
			IsMaster:     status.Addr == masterAddr && status.Port == masterPort,
		}
	}

	return &AgentStatus{
		Clients:        a.wsServer.GetStatus(),
		Analyzers:      analyzers,
		TopologyProbes: a.topologyProbeBundle.ActiveProbes(),
		FlowProbes:     a.flowProbeBundle.ActiveProbes(),
	}
}

// Start the agent services
func (a *Agent) Start() {
	go a.httpServer.Serve()

	a.flowPipeline.Start()
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
	a.flowPipeline.Stop()

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

	if err := hserver.Listen(); err != nil {
		return nil, err
	}

	if _, err = api.NewAPI(hserver, nil, common.AgentService); err != nil {
		return nil, err
	}

	wsServer := shttp.NewWSStructServer(shttp.NewWSServer(hserver, "/ws/subscriber"))

	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewMetricsTraversalExtension())
	tr.AddTraversalExtension(ge.NewRawPacketsTraversalExtension())

	rootNode, err := createRootNode(g)
	if err != nil {
		return nil, err
	}

	api.RegisterTopologyAPI(hserver, g, tr)

	authOptions := analyzer.NewAnalyzerAuthenticationOpts()

	topologyEndpoint := topology.NewTopologySubscriberEndpoint(wsServer, authOptions, g)

	analyzerClientPool, err := NewAnalyzerWSStructClientPool(authOptions)
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

	pipeline := flow.NewEnhancerPipeline(enhancers.NewGraphFlowEnhancer(g))

	// check that the neutron probe if loaded if so add the neutron flow enhancer
	if topologyProbeBundle.GetProbe("neutron") != nil {
		pipeline.AddEnhancer(enhancers.NewNeutronFlowEnhancer(g))
	}

	flowTableAllocator := flow.NewTableAllocator(updateTime, expireTime, pipeline)

	// exposes a flow server through the client connections
	flow.NewServer(flowTableAllocator, analyzerClientPool)

	packet_injector.NewServer(g, analyzerClientPool)

	flowClientPool := analyzer.NewFlowClientPool(analyzerClientPool)

	flowProbeBundle := fprobes.NewFlowProbeBundle(topologyProbeBundle, g, flowTableAllocator, flowClientPool)

	onDemandProbeServer, err := ondemand.NewOnDemandProbeServer(flowProbeBundle, g, analyzerClientPool)
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize on-demand flow probe %s", err.Error())
	}

	agent := &Agent{
		graph:               g,
		wsServer:            wsServer,
		analyzerClientPool:  analyzerClientPool,
		topologyEndpoint:    topologyEndpoint,
		rootNode:            rootNode,
		topologyProbeBundle: topologyProbeBundle,
		flowProbeBundle:     flowProbeBundle,
		flowPipeline:        pipeline,
		flowTableAllocator:  flowTableAllocator,
		flowClientPool:      flowClientPool,
		onDemandProbeServer: onDemandProbeServer,
		httpServer:          hserver,
		tidMapper:           tm,
		topologyForwarder:   tforwarder,
	}

	api.RegisterStatusAPI(hserver, agent)

	return agent, nil
}
