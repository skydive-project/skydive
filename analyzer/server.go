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
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/skydive-project/skydive/alert"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/flow"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/client"
	"github.com/skydive-project/skydive/flow/storage"
	ftraversal "github.com/skydive-project/skydive/flow/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/packet_injector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

type Server struct {
	shttp.DefaultWSServerEventHandler
	HTTPServer        *shttp.Server
	WSServer          *shttp.WSServer
	TopologyForwarder *TopologyForwarder
	TopologyServer    *TopologyServer
	AlertServer       *alert.AlertServer
	OnDemandClient    *ondemand.OnDemandProbeClient
	FlowServer        *FlowServer
	ProbeBundle       *probe.ProbeBundle
	Storage           storage.Storage
	EmbeddedEtcd      *etcd.EmbeddedEtcd
	EtcdClient        *etcd.EtcdClient
	running           atomic.Value
	wgServers         sync.WaitGroup
	wgFlowsHandlers   sync.WaitGroup
}

func (s *Server) Start() {
	s.running.Store(true)

	if s.Storage != nil {
		s.Storage.Start()
	}

	s.TopologyForwarder.ConnectAll()

	s.ProbeBundle.Start()
	s.OnDemandClient.Start()
	s.AlertServer.Start()

	s.wgServers.Add(2)
	go func() {
		defer s.wgServers.Done()
		s.HTTPServer.ListenAndServe()
	}()

	go func() {
		defer s.wgServers.Done()
		s.WSServer.ListenAndServe()
	}()

	s.FlowServer.Start()
}

func (s *Server) Stop() {
	s.FlowServer.Stop()
	s.WSServer.Stop()
	s.HTTPServer.Stop()
	if s.EmbeddedEtcd != nil {
		s.EmbeddedEtcd.Stop()
	}
	if s.Storage != nil {
		s.Storage.Stop()
	}
	s.ProbeBundle.Stop()
	s.OnDemandClient.Stop()
	s.AlertServer.Stop()
	s.EtcdClient.Stop()
	s.wgServers.Wait()
	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
}

func NewServerFromConfig() (*Server, error) {
	embedEtcd := config.GetConfig().GetBool("etcd.embedded")

	httpServer, err := shttp.NewServerFromConfig(common.AnalyzerService)
	if err != nil {
		return nil, err
	}

	wsServer := shttp.NewWSServerFromConfig(common.AnalyzerService, httpServer, "/ws")

	tserver, err := NewTopologyServerFromConfig(wsServer)
	if err != nil {
		return nil, err
	}

	probeBundle, err := NewTopologyProbeBundleFromConfig(tserver.Graph)
	if err != nil {
		return nil, err
	}

	var etcdServer *etcd.EmbeddedEtcd
	if embedEtcd {
		if etcdServer, err = etcd.NewEmbeddedEtcdFromConfig(); err != nil {
			return nil, err
		}
	}

	etcdClient, err := etcd.NewEtcdClientFromConfig()
	if err != nil {
		return nil, err
	}

	analyzerUpdate := config.GetConfig().GetInt("analyzer.flowtable_update")
	analyzerExpire := config.GetConfig().GetInt("analyzer.flowtable_expire")

	if err = etcdClient.SetInt64("/agent/config/flowtable_update", int64(analyzerUpdate)); err != nil {
		return nil, err
	}

	if err = etcdClient.SetInt64("/agent/config/flowtable_expire", int64(analyzerExpire)); err != nil {
		return nil, err
	}

	apiServer, err := api.NewAPI(httpServer, etcdClient.KeysAPI, common.AnalyzerService)
	if err != nil {
		return nil, err
	}

	var captureAPIHandler *api.CaptureAPIHandler
	if captureAPIHandler, err = api.RegisterCaptureAPI(apiServer, tserver.Graph); err != nil {
		return nil, err
	}

	var alertAPIHandler *api.AlertAPIHandler
	if alertAPIHandler, err = api.RegisterAlertAPI(apiServer); err != nil {
		return nil, err
	}

	onDemandClient := ondemand.NewOnDemandProbeClient(tserver.Graph, captureAPIHandler, wsServer, etcdClient)

	tableClient := flow.NewTableClient(wsServer)

	store, err := storage.NewStorageFromConfig()
	if err != nil {
		return nil, err
	}

	fserver, err := NewFlowServer(httpServer.Addr, httpServer.Port, tserver.Graph, store, probeBundle)
	if err != nil {
		return nil, err
	}

	tr := traversal.NewGremlinTraversalParser(tserver.Graph)
	tr.AddTraversalExtension(topology.NewTopologyTraversalExtension())
	tr.AddTraversalExtension(ftraversal.NewFlowTraversalExtension(tableClient, store))

	aserver := alert.NewAlertServer(alertAPIHandler, wsServer, tr, etcdClient)

	piClient := packet_injector.NewPacketInjectorClient(wsServer)

	forwarder := NewTopologyForwarderFromConfig(tserver.Graph, wsServer)

	server := &Server{
		HTTPServer:        httpServer,
		WSServer:          wsServer,
		TopologyForwarder: forwarder,
		TopologyServer:    tserver,
		AlertServer:       aserver,
		OnDemandClient:    onDemandClient,
		EmbeddedEtcd:      etcdServer,
		EtcdClient:        etcdClient,
		FlowServer:        fserver,
		ProbeBundle:       probeBundle,
		Storage:           store,
	}

	wsServer.AddEventHandler(server)

	api.RegisterTopologyAPI(httpServer, tr)

	api.RegisterPacketInjectorAPI(piClient, tserver.Graph, httpServer)

	api.RegisterPcapAPI(httpServer, store)

	api.RegisterConfigAPI(httpServer)

	return server, nil
}
