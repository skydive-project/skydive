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
	"net/http"
	"os"
	"time"

	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/flow"
	fprobes "github.com/skydive-project/skydive/flow/probes"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	tprobes "github.com/skydive-project/skydive/topology/probes"
)

type Agent struct {
	Graph                 *graph.Graph
	WSClient              *shttp.WSAsyncClient
	WSServer              *shttp.WSServer
	GraphServer           *graph.GraphServer
	Root                  *graph.Node
	TopologyProbeBundle   *tprobes.TopologyProbeBundle
	FlowProbeBundle       *fprobes.FlowProbeBundle
	FlowTableAllocator    *flow.TableAllocator
	OnDemandProbeListener *fprobes.OnDemandProbeListener
	HTTPServer            *shttp.Server
	EtcdClient            *etcd.EtcdClient
	TIDMapper             *topology.TIDMapper
}

func (a *Agent) Start() {
	var err error

	go a.WSServer.ListenAndServe()

	addr, port, err := config.GetAnalyzerClientAddr()
	if err != nil {
		logging.GetLogger().Errorf("Unable to parse analyzer client %s", err.Error())
		os.Exit(1)
	}

	if addr != "" {
		authOptions := &shttp.AuthenticationOpts{
			Username: config.GetConfig().GetString("agent.analyzer_username"),
			Password: config.GetConfig().GetString("agent.analyzer_password"),
		}
		authClient := waitAnalyzer(addr, port, authOptions)
		a.WSClient, err = shttp.NewWSAsyncClientFromConfig("skydive-agent", addr, port, "/ws", authClient)
		if err != nil {
			logging.GetLogger().Errorf("Unable to instantiate analyzer client %s", err.Error())
			os.Exit(1)
		}

		graph.NewForwarder(a.WSClient, a.Graph, a.Root)
		a.WSClient.Connect()

		// send a first reset event to the analyzers
		a.Graph.DelSubGraph(a.Root)
	}

	a.TopologyProbeBundle = tprobes.NewTopologyProbeBundleFromConfig(a.Graph, a.Root)
	a.TopologyProbeBundle.Start()

	go a.HTTPServer.ListenAndServe()

	if addr != "" {
		a.EtcdClient, err = etcd.NewEtcdClientFromConfig()
		if err != nil {
			logging.GetLogger().Errorf("Unable to start etcd client %s", err.Error())
			os.Exit(1)
		}

		captureApiHandler := &api.CaptureApiHandler{
			BasicApiHandler: api.BasicApiHandler{
				ResourceHandler: &api.CaptureResourceHandler{},
				EtcdKeyAPI:      a.EtcdClient.KeysApi,
			},
			Graph: a.Graph,
		}

		for {
			flowtableUpdate, err := a.EtcdClient.GetInt64("/agent/config/flowtable_update")
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			flowtableExpire, err := a.EtcdClient.GetInt64("/agent/config/flowtable_expire")
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			updateTime := time.Duration(flowtableUpdate) * time.Second
			expireTime := time.Duration(flowtableExpire) * time.Second
			a.FlowTableAllocator = flow.NewTableAllocator(updateTime, expireTime)

			// expose a flow server through the client connection
			flow.NewServer(a.FlowTableAllocator, a.WSClient)

			a.FlowProbeBundle = fprobes.NewFlowProbeBundleFromConfig(a.TopologyProbeBundle, a.Graph, a.FlowTableAllocator)
			a.FlowProbeBundle.Start()

			l, err := fprobes.NewOnDemandProbeListener(a.FlowProbeBundle, a.Graph, captureApiHandler)
			if err != nil {
				logging.GetLogger().Errorf("Unable to start on-demand flow probe %s", err.Error())
				os.Exit(1)
			}
			a.OnDemandProbeListener = l
			a.OnDemandProbeListener.Start()

			break
		}
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
	if a.WSClient != nil {
		a.WSClient.Disconnect()
	}
	if a.OnDemandProbeListener != nil {
		a.OnDemandProbeListener.Stop()
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

	g, err := graph.NewGraphFromConfig(backend)
	if err != nil {
		panic(err)
	}
	tm := topology.NewTIDMapper(g)
	tm.Start()

	hostID := config.GetConfig().GetString("host_id")

	hserver, err := shttp.NewServerFromConfig("agent")
	if err != nil {
		panic(err)
	}

	_, err = api.NewApi(hserver, nil)
	if err != nil {
		panic(err)
	}

	wsServer := shttp.NewWSServerFromConfig(hserver, "/ws")

	m := graph.Metadata{"Name": hostID, "Type": "host"}
	if config.GetConfig().IsSet("agent.metadata") {
		subtree := config.GetConfig().Sub("agent.metadata")
		for key, value := range subtree.AllSettings() {
			m[key] = value
		}
	}
	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(hostID))
	root := g.NewNode(graph.Identifier(u.String()), m)

	api.RegisterTopologyApi("agent", g, hserver, nil, nil)

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

func waitAnalyzer(addr string, port int, authOptions *shttp.AuthenticationOpts) *shttp.AuthenticationClient {
	authClient := shttp.NewAuthenticationClient(addr, port, authOptions)

	for {
		if !authClient.Authenticated() {
			if err := authClient.Authenticate(); err != nil {
				logging.GetLogger().Warning("Waiting for agent to authenticate")
				time.Sleep(time.Second)
				continue
			}
		}

		restClient := shttp.NewRestClient(addr, port, authOptions)
		if _, err := restClient.Request("GET", "/api", nil); err == nil {
			logging.GetLogger().Info("Analyzer is ready:")
			return authClient
		}

		logging.GetLogger().Warning("Waiting for analyzer to start")
		time.Sleep(time.Second)
	}
}
