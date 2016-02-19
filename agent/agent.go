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
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
	tprobes "github.com/redhat-cip/skydive/topology/probes"
)

type Agent struct {
	Graph       *graph.Graph
	Gclient     *graph.AsyncClient
	GraphServer *graph.Server
	Root        *graph.Node
	TopoServer  *topology.Server
	NsProbe     *tprobes.NetNSProbe
	NlProbe     *tprobes.NetLinkProbe
	DockerProbe *tprobes.DockerProbe
	OvsMon      *ovsdb.OvsMonitor
	OvsProbe    *tprobes.OvsdbProbe
	SFlowProbe  *fprobes.SFlowProbe
}

func (a *Agent) Start() {
	var err error
	// send a first reset event to the analyzers
	a.Graph.DelSubGraph(a.Root)

	a.SFlowProbe, err = fprobes.NewSFlowProbeFromConfig(a.Graph)
	if err != nil {
		panic(err)
	}

	ovsSFlowProbe := ovsdb.SFlowProbe{
		ID:         "SkydiveSFlowProbe",
		Interface:  "eth0",
		Target:     a.SFlowProbe.GetTarget(),
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
	}
	sflowHandler := ovsdb.NewOvsSFlowProbesHandler([]ovsdb.SFlowProbe{ovsSFlowProbe})

	a.OvsMon.AddMonitorHandler(sflowHandler)

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

		a.SFlowProbe.SetAnalyzerClient(analyzer)

		a.Gclient = graph.NewAsyncClient(analyzerAddr, analyzerPort, "/ws/graph")
		graph.NewForwarder(a.Gclient, a.Graph)
		a.Gclient.Connect()
	}

	pipeline := mappings.NewFlowMappingPipeline()

	gfe, err := mappings.NewGraphFlowEnhancer(a.Graph)
	if err != nil {
		panic(err)
	}
	pipeline.AddFlowEnhancer(gfe)

	// start probes that will update the graph
	a.NsProbe.Start()
	a.NlProbe.Start()
	a.OvsProbe.Start()
	a.DockerProbe.Start()

	a.SFlowProbe.SetMappingPipeline(pipeline)
	go a.SFlowProbe.Start()

	if err := a.OvsMon.StartMonitoring(); err != nil {
		panic(err)
	}

	go a.TopoServer.ListenAndServe()

	go a.GraphServer.ListenAndServe()
}

func (a *Agent) Stop() {
	a.SFlowProbe.Stop()
	a.NlProbe.Stop()
	a.NsProbe.Stop()
	a.DockerProbe.Stop()
	a.OvsMon.StopMonitoring()
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

	ovsmon := ovsdb.NewOvsMonitorFromConfig()

	root := g.NewNode(graph.Identifier(hostname), graph.Metadata{"Name": hostname, "Type": "host"})

	ns := tprobes.NewNetNSProbe(g, root)
	nl := tprobes.NewNetLinkProbe(g, root)
	ovs := tprobes.NewOvsdbProbe(g, root, ovsmon)
	docker := tprobes.NewDockerProbeFromConfig(ns)

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
		Graph:       g,
		NsProbe:     ns,
		NlProbe:     nl,
		DockerProbe: docker,
		OvsMon:      ovsmon,
		OvsProbe:    ovs,
		TopoServer:  server,
		GraphServer: gserver,
		Root:        root,
	}
}
