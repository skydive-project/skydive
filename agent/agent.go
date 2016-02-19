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
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow/mappings"
	fprobes "github.com/redhat-cip/skydive/flow/probes"
	"github.com/redhat-cip/skydive/ovs"
	probe "github.com/redhat-cip/skydive/probe"
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
	tprobes "github.com/redhat-cip/skydive/topology/probes"
)

type Agent struct {
	Graph           *graph.Graph
	Gclient         *graph.AsyncClient
	GraphServer     *graph.Server
	Root            *graph.Node
	TopoServer      *topology.Server
	TopoProbeBundle *probe.ProbeBundle
	SFlowAgent      *fprobes.SFlowAgent
}

func (a *Agent) Start() {
	var err error
	// send a first reset event to the analyzers
	a.Graph.DelSubGraph(a.Root)

	gfe, err := mappings.NewGraphFlowEnhancer(a.Graph)
	if err != nil {
		panic(err)
	}

	pipeline := mappings.NewFlowMappingPipeline([]mappings.FlowEnhancer{gfe})

	if p := a.TopoProbeBundle.GetProbe("ovsdb"); p != nil {
		a.SFlowAgent, err = fprobes.NewSFlowAgentFromConfig(a.Graph)
		if err != nil {
			panic(err)
		}

		ovsSFlowProbe := ovsdb.SFlowProbe{
			ID:         "SkydiveSFlowProbe",
			Interface:  "eth0",
			Target:     a.SFlowAgent.GetTarget(),
			HeaderSize: 256,
			Sampling:   1,
			Polling:    0,
		}
		sflowHandler := ovsdb.NewOvsSFlowProbesHandler([]ovsdb.SFlowProbe{ovsSFlowProbe})

		o := p.(*tprobes.OvsdbProbe)
		o.OvsMon.AddMonitorHandler(sflowHandler)

		a.SFlowAgent.SetMappingPipeline(pipeline)

		go a.SFlowAgent.Start()
	}

	analyzers := config.GetConfig().GetStringSlice("agent.analyzers")
	// TODO(safchain) HA Connection ???
	if len(analyzers) > 0 {
		analyzerAddr := strings.Split(analyzers[0], ":")[0]
		analyzerPort, err := strconv.Atoi(strings.Split(analyzers[0], ":")[1])
		if err != nil {
			panic(err)
		}

		analyzer, err := analyzer.NewClient(analyzerAddr, analyzerPort)
		if err != nil {
			panic(err)
		}

		if a.SFlowAgent != nil {
			a.SFlowAgent.SetAnalyzerClient(analyzer)
		}

		a.Gclient = graph.NewAsyncClient(analyzerAddr, analyzerPort, "/ws/graph")
		graph.NewForwarder(a.Gclient, a.Graph)
		a.Gclient.Connect()
	}

	a.TopoProbeBundle.Start()

	go a.TopoServer.ListenAndServe()

	go a.GraphServer.ListenAndServe()
}

func (a *Agent) Stop() {
	if a.SFlowAgent != nil {
		a.SFlowAgent.Stop()
	}
	a.TopoProbeBundle.Stop()
	a.TopoServer.Stop()
	a.GraphServer.Stop()
	if a.Gclient != nil {
		a.Gclient.Disconnect()
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

	root := g.NewNode(graph.Identifier(hostname), graph.Metadata{"Name": hostname, "Type": "host"})

	bundle := tprobes.NewProbeBundleFromConfig(g, root)

	router := mux.NewRouter().StrictSlash(true)

	server, err := topology.NewServerFromConfig("agent", g, router)
	if err != nil {
		panic(err)
	}

	server.RegisterStaticEndpoints()
	server.RegisterRPCEndpoints()

	gserver, err := graph.NewServerFromConfig(g, nil, router)
	if err != nil {
		panic(err)
	}

	return &Agent{
		Graph:           g,
		TopoProbeBundle: bundle,
		TopoServer:      server,
		GraphServer:     gserver,
		Root:            root,
	}
}
