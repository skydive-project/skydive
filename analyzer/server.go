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

type Server struct {
	Port                int
	TopoServer          *topology.Server
	GraphServer         *graph.Server
	FlowMappingPipeline *mappings.FlowMappingPipeline
	//Storage     storage.Storage
}

func (s *Server) AnalyzeFlows(flows []*flow.Flow) {
	s.FlowMappingPipeline.Enhance(flows)
	//Server.Storage.StoreFlows(flows)

	logging.GetLogger().Debug("%d flows stored", len(flows))
}

func (s *Server) handleUDPFlowPacket(conn *net.UDPConn) {
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

		s.AnalyzeFlows([]*flow.Flow{f})
	}
}

func (s *Server) ListenAndServe() {
	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		s.TopoServer.ListenAndServe()
	}()

	go func() {
		defer wg.Done()
		s.GraphServer.ListenAndServe()
	}()

	go func() {
		defer wg.Done()

		addr, err := net.ResolveUDPAddr("udp", ":"+strconv.FormatInt(int64(s.Port), 10))
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		s.handleUDPFlowPacket(conn)
	}()

	wg.Wait()
}

func NewServer(port int) (*Server, error) {
	g, err := graph.NewGraph("Global")
	if err != nil {
		return nil, err
	}

	server := topology.NewServer(g, port)
	server.RegisterStaticEndpoints()
	server.RegisterRpcEndpoints()

	gserver := graph.NewServer(g, server.Router)

	gfe, err := mappings.NewGraphFlowEnhancer(g)
	if err != nil {
		return nil, err
	}

	pipeline := mappings.NewFlowMappingPipeline([]mappings.FlowEnhancer{gfe})

	return &Server{
		TopoServer:          server,
		GraphServer:         gserver,
		FlowMappingPipeline: pipeline,
	}, nil
}
