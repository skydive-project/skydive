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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	cache "github.com/pmylund/go-cache"
	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/enhancers"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/server"
	fprobes "github.com/skydive-project/skydive/flow/probes"
	shttp "github.com/skydive-project/skydive/http"
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
		return nil, fmt.Errorf("Unable to get the analyzers list: %s", err.Error())
	}

	for _, sa := range addresses {
		authClient := shttp.NewAuthenticationClient(config.GetURL("http", sa.Addr, sa.Port, ""), authOptions)
		c := shttp.NewWSClientFromConfig(common.AgentService, config.GetURL("ws", sa.Addr, sa.Port, "/ws"), authClient, nil)
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

	wsServer := shttp.NewWSJSONServer(shttp.NewWSServer(hserver, "/ws"))

	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(topology.NewTopologyTraversalExtension())

	rootNode, err := createRootNode(g)
	if err != nil {
		return nil, err
	}

	api.RegisterTopologyAPI(hserver, g, tr)

	tserver := NewTopologyServer(g, wsServer)

	analyzerClientPool, err := NewAnalyzerWSJSONClientPool()
	if err != nil {
		return nil, err
	}

	tforwarder := NewTopologyForwarderFromConfig(g, analyzerClientPool)

	topologyProbeBundle, err := NewTopologyProbeBundleFromConfig(g, rootNode)
	if err != nil {
		return nil, err
	}

	updateTime := time.Duration(config.GetConfig().GetInt("flow.update")) * time.Second
	expireTime := time.Duration(config.GetConfig().GetInt("flow.expire")) * time.Second
	cleanup := time.Duration(config.GetConfig().GetInt("cache.cleanup")) * time.Second

	cache := cache.New(expireTime*2, cleanup)

	pipeline := flow.NewEnhancerPipeline(
		enhancers.NewGraphFlowEnhancer(g, cache),
		enhancers.NewSocketInfoEnhancer(expireTime*2, cleanup),
	)

	// check that the neutron probe if loaded if so add the neutron flow enhancer
	if topologyProbeBundle.GetProbe("neutron") != nil {
		pipeline.AddEnhancer(enhancers.NewNeutronFlowEnhancer(g, cache))
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
		topologyServer:      tserver,
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

	api.RegisterStatusAPI(hserver, agent)

	return agent, nil
}

// CreateRootNode creates a graph.Node based on the host properties and aims to have an unique ID
func createRootNode(g *graph.Graph) (*graph.Node, error) {
	hostID := config.GetConfig().GetString("host_id")
	m := graph.Metadata{"Name": hostID, "Type": "host"}

	// Fill the metadata from the configuration file
	if config.GetConfig().IsSet("agent.metadata") {
		configMetadata, ok := common.NormalizeValue(config.GetConfig().Get("agent.metadata")).(map[string]interface{})
		if !ok {
			return nil, errors.New("agent.metadata has wrong format")
		}
		for k, v := range configMetadata {
			m[k] = v
		}
	}

	// Retrieves the instance ID from cloud-init
	buffer, err := ioutil.ReadFile("/var/lib/cloud/data/instance-id")
	if err == nil {
		m["InstanceID"] = strings.TrimSpace(string(buffer))
	}

	return g.NewNode(graph.GenID(), m), nil
}
