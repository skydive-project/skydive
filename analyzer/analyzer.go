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

package analyzer

import (
	"net"
	"strconv"
	"sync"

	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
	//"github.com/redhat-cip/skydive/storage"
	"github.com/redhat-cip/skydive/topology"
	"github.com/redhat-cip/skydive/topology/graph"
)

type Analyzer struct {
	Port            int
	TopoServer      *topology.Server
	GraphServer     *graph.Server
	MappingPipeline *mappings.MappingPipeline
	//Storage     storage.Storage
}

func (a *Analyzer) AnalyzeFlows(flows []*flow.Flow) {
	a.MappingPipeline.Enhance(flows)
	//analyzer.Storage.StoreFlows(flows)

	logging.GetLogger().Debug("%d flows stored", len(flows))
}

func (a *Analyzer) handleUDPFlowPacket(conn *net.UDPConn) {
	data := make([]byte, 4096)

	for {
		n, _, err := conn.ReadFromUDP(data)
		if err != nil {
			logging.GetLogger().Error("Error while reading: %s", err.Error())
			return
		}

		f, err := flow.FromData(data[0:n])
		if err != nil {
			logging.GetLogger().Error("Error while parsing flow: %s", err.Error())
		}

		a.AnalyzeFlows([]*flow.Flow{f})
	}
}

func (a *Analyzer) ListenAndServe() {
	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		a.TopoServer.ListenAndServe()
	}()

	go func() {
		defer wg.Done()
		a.GraphServer.ListenAndServe()
	}()

	go func() {
		defer wg.Done()

		addr, err := net.ResolveUDPAddr("udp", ":"+strconv.FormatInt(int64(a.Port), 10))
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		a.handleUDPFlowPacket(conn)
	}()

	wg.Wait()
}

func NewAnalyzer(port int) (*Analyzer, error) {
	g, err := graph.NewGraph("Global")
	if err != nil {
		return nil, err
	}

	server := topology.NewServer(g, port)
	server.RegisterStaticEndpoints()
	server.RegisterRpcEndpoints()

	gserver := graph.NewServer(g, server.Router)

	gm, err := mappings.NewGraphMappingDriver(g)
	if err != nil {
		return nil, err
	}

	ip := mappings.NewInterfacePipeline([]mappings.InterfaceDriver{gm})
	pipeline := mappings.NewMappingPipeline(ip)

	return &Analyzer{
		TopoServer:      server,
		GraphServer:     gserver,
		MappingPipeline: pipeline,
	}, nil
}
