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

	"github.com/redhat-cip/skydive/api"
	"github.com/redhat-cip/skydive/config"
	fprobes "github.com/redhat-cip/skydive/flow/probes"
	shttp "github.com/redhat-cip/skydive/http"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/storage/etcd"
	"github.com/redhat-cip/skydive/topology/graph"
	tprobes "github.com/redhat-cip/skydive/topology/probes"
)

type Agent struct {
	Graph                 *graph.Graph
	WSClient              *shttp.WSAsyncClient
	WSServer              *shttp.WSServer
	GraphServer           *graph.GraphServer
	Root                  *graph.Node
	TopologyProbeBundle   *tprobes.TopologyProbeBundle
	FlowProbeBundle       *fprobes.FlowProbeBundle
	OnDemandProbeListener *fprobes.OnDemandProbeListener
	HTTPServer            *shttp.Server
	EtcdClient            *etcd.EtcdClient
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
		authClient := shttp.NewAuthenticationClient(addr, port, authOptions)
		a.WSClient, err = shttp.NewWSAsyncClient(addr, port, "/ws", authClient)
		if err != nil {
			logging.GetLogger().Errorf("Unable to instantiate analyzer client %s", err.Error())
			os.Exit(1)
		}

		graph.NewForwarder(a.WSClient, a.Graph)
		a.WSClient.Connect()

		// send a first reset event to the analyzers
		a.Graph.DelSubGraph(a.Root)
	}

	a.TopologyProbeBundle = tprobes.NewTopologyProbeBundleFromConfig(a.Graph, a.Root)
	a.TopologyProbeBundle.Start()

	a.FlowProbeBundle = fprobes.NewFlowProbeBundleFromConfig(a.TopologyProbeBundle, a.Graph)
	a.FlowProbeBundle.Start()

	if addr != "" {
		a.EtcdClient, err = etcd.NewEtcdClientFromConfig()
		if err != nil {
			logging.GetLogger().Errorf("Unable to start etcd client %s", err.Error())
			os.Exit(1)
		}

		captureHandler := &api.BasicApiHandler{
			ResourceHandler: &api.CaptureHandler{},
			EtcdKeyAPI:      a.EtcdClient.KeysApi,
		}

		l, err := fprobes.NewOnDemandProbeListener(a.FlowProbeBundle, a.Graph, captureHandler)
		if err != nil {
			logging.GetLogger().Errorf("Unable to start on-demand flow probe %s", err.Error())
			os.Exit(1)
		}
		a.OnDemandProbeListener = l
		a.OnDemandProbeListener.Start()
	}

	go a.HTTPServer.ListenAndServe()
}

func (a *Agent) Stop() {
	a.FlowProbeBundle.UnregisterAllProbes()
	a.FlowProbeBundle.Stop()
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
}

func NewAgent() *Agent {
	backend, err := graph.NewMemoryBackend()
	if err != nil {
		panic(err)
	}

	g, err := graph.NewGraph(backend)
	if err != nil {
		panic(err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	hserver, err := shttp.NewServerFromConfig("agent")
	if err != nil {
		panic(err)
	}

	_, err = api.NewApi(hserver, nil)
	if err != nil {
		panic(err)
	}

	wsServer := shttp.NewWSServerFromConfig(hserver, "/ws")

	root := g.NewNode(graph.Identifier(hostname), graph.Metadata{"Name": hostname, "Type": "host"})

	api.RegisterTopologyApi("agent", g, hserver)

	gserver := graph.NewServer(g, wsServer)

	return &Agent{
		Graph:       g,
		WSServer:    wsServer,
		GraphServer: gserver,
		Root:        root,
		HTTPServer:  hserver,
	}
}
