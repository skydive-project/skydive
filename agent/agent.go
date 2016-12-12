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

	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/flow"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/server"
	fprobes "github.com/skydive-project/skydive/flow/probes"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/packet_injector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

type Agent struct {
	shttp.DefaultWSClientEventHandler
	Graph               *graph.Graph
	WSAsyncClientPool   *shttp.WSAsyncClientPool
	WSServer            *shttp.WSServer
	GraphServer         *graph.GraphServer
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

	// at least one analyzer, so try to get info from etcd
	if _, err := config.GetOneAnalyzerServiceAddress(); err == nil {
		a.EtcdClient, err = etcd.NewEtcdClientFromConfig()
		if err != nil {
			logging.GetLogger().Errorf("Unable to start etcd client %s", err.Error())
			os.Exit(1)
		}

		// waiting for some config coming from etcd
		var flowtableUpdate, flowtableExpire int64
		for {
			if flowtableUpdate, err = a.EtcdClient.GetInt64("/agent/config/flowtable_update"); err != nil {
				time.Sleep(time.Second)
				continue
			}
			if flowtableExpire, err = a.EtcdClient.GetInt64("/agent/config/flowtable_expire"); err != nil {
				time.Sleep(time.Second)
				continue
			}
			break
		}

		updateTime := time.Duration(flowtableUpdate) * time.Second
		expireTime := time.Duration(flowtableExpire) * time.Second
		a.FlowTableAllocator = flow.NewTableAllocator(updateTime, expireTime)

		// expose a flow server through the client connections
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
}

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

	_, err = api.NewApi(hserver, nil)
	if err != nil {
		panic(err)
	}

	wsServer := shttp.NewWSServerFromConfig(common.AgentService, hserver, "/ws")

	root := CreateRootNode(g)
	api.RegisterTopologyApi(g, hserver, nil, nil)

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
	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(hostID))
	return g.NewNode(graph.Identifier(u.String()), m)
}
