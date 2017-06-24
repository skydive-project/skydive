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
	shttp.DefaultWSClientEventHandler
	Graph               *graph.Graph
	WSAsyncClientPool   *shttp.WSAsyncClientPool
	WSServer            *shttp.WSServer
	GraphServer         *graph.Server
	Root                *graph.Node
	TopologyProbeBundle *probe.ProbeBundle
	FlowProbeBundle     *fprobes.FlowProbeBundle
	FlowTableAllocator  *flow.TableAllocator
	FlowClientPool      *analyzer.FlowClientPool
	OnDemandProbeServer *ondemand.OnDemandProbeServer
	HTTPServer          *shttp.Server
	EtcdClient          *etcd.EtcdClient
	TIDMapper           *topology.TIDMapper
}

// NewAnalyzerWSClientPool creates a new http WebSocket client Pool
// with authentification
func NewAnalyzerWSClientPool() *shttp.WSAsyncClientPool {
	wspool := shttp.NewWSAsyncClientPool()

	authOptions := &shttp.AuthenticationOpts{
		Username: config.GetConfig().GetString("auth.analyzer_username"),
		Password: config.GetConfig().GetString("auth.analyzer_password"),
	}

	addresses, err := config.GetAnalyzerServiceAddresses()
	if err != nil {
		logging.GetLogger().Warningf("Unable to get the analyzers list: %s", err.Error())
		return nil
	}

	for _, sa := range addresses {
		authClient := shttp.NewAuthenticationClient(sa.Addr, sa.Port, authOptions)
		wsclient := shttp.NewWSAsyncClientFromConfig(common.AgentService, sa.Addr, sa.Port, "/ws", authClient)

		wspool.AddWSAsyncClient(wsclient)
	}

	return wspool
}

// Start the agent services
func (a *Agent) Start() {
	var err error

	go a.HTTPServer.ListenAndServe()
	go a.WSServer.ListenAndServe()

	a.WSAsyncClientPool = NewAnalyzerWSClientPool()
	if a.WSAsyncClientPool == nil {
		os.Exit(1)
	}

	NewTopologyForwarderFromConfig(a.Graph, a.WSAsyncClientPool)

	a.TopologyProbeBundle, err = NewTopologyProbeBundleFromConfig(a.Graph, a.Root, a.WSAsyncClientPool)
	if err != nil {
		logging.GetLogger().Errorf("Unable to instantiate topology probes: %s", err.Error())
		os.Exit(1)
	}
	a.TopologyProbeBundle.Start()

	updateTime := time.Duration(config.GetConfig().GetInt("flow.update")) * time.Second
	expireTime := time.Duration(config.GetConfig().GetInt("flow.expire")) * time.Second
	cleanup := time.Duration(config.GetConfig().GetInt("cache.cleanup")) * time.Second

	cache := cache.New(expireTime*2, cleanup)

	pipeline := flow.NewEnhancerPipeline(enhancers.NewGraphFlowEnhancer(a.Graph, cache))

	// check that the neutron probe if loaded if so add the neutron flow enhancer
	if a.TopologyProbeBundle.GetProbe("neutron") != nil {
		pipeline.AddEnhancer(enhancers.NewNeutronFlowEnhancer(a.Graph, cache))
	}

	a.FlowTableAllocator = flow.NewTableAllocator(updateTime, expireTime, pipeline)

	// exposes a flow server through the client connections
	flow.NewServer(a.FlowTableAllocator, a.WSAsyncClientPool)

	packet_injector.NewServer(a.WSAsyncClientPool, a.Graph)

	a.FlowClientPool = analyzer.NewFlowClientPool(a.WSAsyncClientPool)

	a.FlowProbeBundle = fprobes.NewFlowProbeBundleFromConfig(a.TopologyProbeBundle, a.Graph, a.FlowTableAllocator, a.FlowClientPool)
	a.FlowProbeBundle.Start()

	if a.OnDemandProbeServer, err = ondemand.NewOnDemandProbeServer(a.FlowProbeBundle, a.Graph, a.WSAsyncClientPool); err != nil {
		logging.GetLogger().Errorf("Unable to start on-demand flow probe %s", err.Error())
		os.Exit(1)
	}
	a.OnDemandProbeServer.Start()

	// everything is ready, then initiate the websocket connection
	go a.WSAsyncClientPool.ConnectAll()
}

// Stop agent services
func (a *Agent) Stop() {
	if a.FlowProbeBundle != nil {
		a.FlowProbeBundle.UnregisterAllProbes()
		a.FlowProbeBundle.Stop()
	}
	a.TopologyProbeBundle.Stop()
	a.HTTPServer.Stop()
	a.WSServer.Stop()
	a.WSAsyncClientPool.DisconnectAll()
	if a.FlowClientPool != nil {
		a.FlowClientPool.Close()
	}
	if a.OnDemandProbeServer != nil {
		a.OnDemandProbeServer.Stop()
	}
	if a.EtcdClient != nil {
		a.EtcdClient.Stop()
	}
	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
	a.TIDMapper.Stop()
}

// NewAgent instanciates a new Agent aiming to launch probes (topology and flow)
func NewAgent() *Agent {
	backend, err := graph.NewMemoryBackend()
	if err != nil {
		panic(err)
	}

	g := graph.NewGraphFromConfig(backend)

	tm := topology.NewTIDMapper(g)
	tm.Start()

	hserver, err := shttp.NewServerFromConfig(common.AgentService)
	if err != nil {
		panic(err)
	}

	if _, err = api.NewAPI(hserver, nil, common.AgentService); err != nil {
		panic(err)
	}

	wsServer := shttp.NewWSServerFromConfig(hserver, "/ws")

	tr := traversal.NewGremlinTraversalParser(g)
	tr.AddTraversalExtension(topology.NewTopologyTraversalExtension())

	root := CreateRootNode(g)
	api.RegisterTopologyAPI(hserver, tr)

	gserver := graph.NewServer(g, wsServer)

	return &Agent{
		Graph:       g,
		WSServer:    wsServer,
		GraphServer: gserver,
		Root:        root,
		HTTPServer:  hserver,
		TIDMapper:   tm,
	}
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
