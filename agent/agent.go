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
	Graph               *graph.Graph
	WSServer            *shttp.WSJSONServer
	AnalyzerClientPool  *shttp.WSJSONClientPool
	TopologyServer      *TopologyServer
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

// Start the agent services
func (a *Agent) Start() {
	var err error

	go a.HTTPServer.ListenAndServe()
	a.WSServer.Start()

	pool, err := NewAnalyzerWSJSONClientPool()
	if err != nil {
		logging.GetLogger().Error(err)
		os.Exit(1)
	}
	a.AnalyzerClientPool = pool

	NewTopologyForwarderFromConfig(a.Graph, pool)

	a.TopologyProbeBundle, err = NewTopologyProbeBundleFromConfig(a.Graph, a.Root)
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
	flow.NewServer(a.FlowTableAllocator, pool)

	packet_injector.NewServer(a.Graph, pool)

	a.FlowClientPool = analyzer.NewFlowClientPool(pool)

	a.FlowProbeBundle = fprobes.NewFlowProbeBundle(a.TopologyProbeBundle, a.Graph, a.FlowTableAllocator, a.FlowClientPool)
	a.FlowProbeBundle.Start()

	if a.OnDemandProbeServer, err = ondemand.NewOnDemandProbeServer(a.FlowProbeBundle, a.Graph, pool); err != nil {
		logging.GetLogger().Errorf("Unable to start on-demand flow probe %s", err.Error())
		os.Exit(1)
	}
	a.OnDemandProbeServer.Start()

	// everything is ready, then initiate the websocket connection
	go pool.ConnectAll()
}

// Stop agent services
func (a *Agent) Stop() {
	if a.FlowProbeBundle != nil {
		a.FlowProbeBundle.Stop()
	}
	a.AnalyzerClientPool.Destroy()
	a.TopologyProbeBundle.Stop()
	a.HTTPServer.Stop()
	a.WSServer.Stop()

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

	wsServer := shttp.NewWSJSONServer(shttp.NewWSServerFromConfig(hserver, "/ws"))

	tr := traversal.NewGremlinTraversalParser(g)
	tr.AddTraversalExtension(topology.NewTopologyTraversalExtension())

	root := CreateRootNode(g)
	api.RegisterTopologyAPI(hserver, tr)

	tserver := NewTopologyServer(g, wsServer)

	return &Agent{
		Graph:          g,
		WSServer:       wsServer,
		TopologyServer: tserver,
		Root:           root,
		HTTPServer:     hserver,
		TIDMapper:      tm,
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
