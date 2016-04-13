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
	Gclient               *graph.AsyncClient
	GraphServer           *graph.Server
	Root                  *graph.Node
	TopologyProbeBundle   *tprobes.TopologyProbeBundle
	FlowProbeBundle       *fprobes.FlowProbeBundle
	OnDemandProbeListener *fprobes.OnDemandProbeListener
	HTTPServer            *shttp.Server
}

func (a *Agent) Start() {
	var err error
	// send a first reset event to the analyzers
	a.Graph.DelSubGraph(a.Root)

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
		a.Gclient = graph.NewAsyncClient(addr, port, "/ws/graph", authClient)
		graph.NewForwarder(a.Gclient, a.Graph)
		a.Gclient.Connect()
	}

	a.TopologyProbeBundle = tprobes.NewTopologyProbeBundleFromConfig(a.Graph, a.Root)
	a.TopologyProbeBundle.Start()

	a.FlowProbeBundle = fprobes.NewFlowProbeBundleFromConfig(a.TopologyProbeBundle, a.Graph)
	a.FlowProbeBundle.Start()

	if addr != "" {
		etcdClient, err := etcd.NewEtcdClientFromConfig()
		if err != nil {
			logging.GetLogger().Errorf("Unable to start etcd client %s", err.Error())
			os.Exit(1)
		}

		captureHandler := &api.BasicApiHandler{
			ResourceHandler: &api.CaptureHandler{},
			EtcdKeyAPI:      etcdClient.KeysApi,
		}

		l, err := fprobes.NewOnDemandProbeListener(a.FlowProbeBundle, a.Graph, captureHandler)
		if err != nil {
			logging.GetLogger().Errorf("Unable to start on-demand flow probe %s", err.Error())
			os.Exit(1)
		}
		a.OnDemandProbeListener = l
		a.OnDemandProbeListener.Start()
	}

	go a.GraphServer.ListenAndServe()

	go a.HTTPServer.ListenAndServe()
}

func (a *Agent) Stop() {
	a.TopologyProbeBundle.Stop()
	a.FlowProbeBundle.Stop()
	a.GraphServer.Stop()
	a.HTTPServer.Stop()
	if a.Gclient != nil {
		a.Gclient.Disconnect()
	}
	if a.OnDemandProbeListener != nil {
		a.OnDemandProbeListener.Stop()
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

	root := g.NewNode(graph.Identifier(hostname), graph.Metadata{"Name": hostname, "Type": "host"})

	api.RegisterTopologyApi("agent", g, hserver)

	gserver, err := graph.NewServerFromConfig(g, hserver)
	if err != nil {
		panic(err)
	}

	return &Agent{
		Graph:       g,
		GraphServer: gserver,
		Root:        root,
		HTTPServer:  hserver,
	}
}
