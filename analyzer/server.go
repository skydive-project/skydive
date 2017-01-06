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
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skydive-project/skydive/alert"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/etcd"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/mappings"
	ondemand "github.com/skydive-project/skydive/flow/ondemand/client"
	"github.com/skydive-project/skydive/flow/storage"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/packet_injector"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

type Server struct {
	HTTPServer          *shttp.Server
	WSServer            *shttp.WSServer
	GraphServer         *graph.GraphServer
	AlertServer         *alert.AlertServer
	OnDemandClient      *ondemand.OnDemandProbeClient
	FlowMappingPipeline *mappings.FlowMappingPipeline
	ProbeBundle         *probe.ProbeBundle
	Storage             storage.Storage
	FlowTable           *flow.Table
	TableClient         *flow.TableClient
	conn                *AgentAnalyzerServerConn
	EmbeddedEtcd        *etcd.EmbeddedEtcd
	EtcdClient          *etcd.EtcdClient
	running             atomic.Value
	wgServers           sync.WaitGroup
	wgFlowsHandlers     sync.WaitGroup
}

func (s *Server) flowExpireUpdate(flows []*flow.Flow) {
	if s.Storage != nil {
		s.Storage.StoreFlows(flows)
		logging.GetLogger().Debugf("%d flows stored", len(flows))
	}
}

func (s *Server) AnalyzeFlows(flows []*flow.Flow) {
	s.FlowTable.Update(flows)
	s.FlowMappingPipeline.Enhance(flows)

	logging.GetLogger().Debugf("%d flows received", len(flows))
}

/* handleFlowPacket can handle connection based on TCP or UDP */
func (s *Server) handleFlowPacket(conn *AgentAnalyzerServerConn) {
	defer s.wgFlowsHandlers.Done()
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
	data := make([]byte, 4096)

	for s.running.Load() == true {
		n, err := conn.Read(data)
		if err != nil {
			if conn.Timeout(err) {
				conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
				continue
			}
			if s.running.Load() == false {
				return
			}
			logging.GetLogger().Errorf("Error while reading: %s", err.Error())
			return
		}

		f, err := flow.FromData(data[0:n])
		if err != nil {
			logging.GetLogger().Errorf("Error while parsing flow: %s", err.Error())
			continue
		}

		s.AnalyzeFlows([]*flow.Flow{f})
	}
}

func (s *Server) ListenAndServe() {
	s.running.Store(true)

	if s.Storage != nil {
		s.Storage.Start()
	}

	s.ProbeBundle.Start()
	s.OnDemandClient.Start()
	s.AlertServer.Start()

	s.wgServers.Add(3)
	go func() {
		defer s.wgServers.Done()
		s.HTTPServer.ListenAndServe()
	}()

	go func() {
		defer s.wgServers.Done()
		s.WSServer.ListenAndServe()
	}()

	host := s.HTTPServer.Addr + ":" + strconv.FormatInt(int64(s.HTTPServer.Port), 10)
	addr, err := net.ResolveUDPAddr("udp", host)
	s.conn, err = NewAgentAnalyzerServerConn(addr)
	if err != nil {
		panic(err)
	}
	go func() {
		defer s.wgServers.Done()

		for s.running.Load() == true {
			switch s.conn.Mode() {
			case TLS:
				conn, err := s.conn.Accept()
				if s.running.Load() == false {
					break
				}
				if err != nil {
					logging.GetLogger().Errorf("Accept error : %s", err.Error())
					time.Sleep(200 * time.Millisecond)
					continue
				}

				s.wgFlowsHandlers.Add(1)
				go s.handleFlowPacket(conn)
			case UDP:
				s.wgFlowsHandlers.Add(1)
				s.handleFlowPacket(s.conn)
			}
		}
		s.wgFlowsHandlers.Wait()
		logging.GetLogger().Debug("server flows : wait for opened connection done")
	}()

	s.FlowTable.Start()
}

func (s *Server) Stop() {
	s.running.Store(false)
	s.FlowTable.Stop()
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
	s.conn.Cleanup()
	s.wgServers.Wait()
	if tr, ok := http.DefaultTransport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
}

func (s *Server) SetStorage(storage storage.Storage) {
	s.Storage = storage
}

func NewServerFromConfig() (*Server, error) {
	embedEtcd := config.GetConfig().GetBool("etcd.embedded")

	backend, err := graph.BackendFromConfig()
	if err != nil {
		return nil, err
	}

	g := graph.NewGraphFromConfig(backend)

	httpServer, err := shttp.NewServerFromConfig("analyzer")
	if err != nil {
		return nil, err
	}

	wsServer := shttp.NewWSServerFromConfig(httpServer, "/ws")

	probeBundle, err := NewTopologyProbeBundleFromConfig(g)
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
	agentRatio := config.GetConfig().GetFloat64("analyzer.flowtable_agent_ratio")
	if agentRatio == 0.0 {
		agentRatio = 0.5
	}

	agentUpdate := int64(float64(analyzerUpdate) * agentRatio)
	agentExpire := int64(float64(analyzerExpire) * agentRatio)

	if err := etcdClient.SetInt64("/agent/config/flowtable_update", agentUpdate); err != nil {
		return nil, err
	}

	if err := etcdClient.SetInt64("/agent/config/flowtable_expire", agentExpire); err != nil {
		return nil, err
	}

	apiServer, err := api.NewApi(httpServer, etcdClient.KeysApi)
	if err != nil {
		return nil, err
	}

	captureApiHandler := &api.CaptureApiHandler{
		BasicApiHandler: api.BasicApiHandler{
			ResourceHandler: &api.CaptureResourceHandler{},
			EtcdKeyAPI:      etcdClient.KeysApi,
		},
		Graph: g,
	}
	err = apiServer.RegisterApiHandler(captureApiHandler)
	if err != nil {
		return nil, err
	}

	alertApiHandler := &api.AlertApiHandler{
		BasicApiHandler: api.BasicApiHandler{
			ResourceHandler: &api.AlertResourceHandler{},
			EtcdKeyAPI:      etcdClient.KeysApi,
		},
	}
	err = apiServer.RegisterApiHandler(alertApiHandler)
	if err != nil {
		return nil, err
	}

	onDemandClient := ondemand.NewOnDemandProbeClient(g, captureApiHandler, wsServer)

	pipeline := mappings.NewFlowMappingPipeline(mappings.NewGraphFlowEnhancer(g))

	// check that the neutron probe is loaded if so add the neutron flow enhancer
	if probeBundle.GetProbe("neutron") != nil {
		pipeline.AddEnhancer(mappings.NewNeutronFlowEnhancer(g))
	}

	tableClient := flow.NewTableClient(wsServer)
	store, err := storage.NewStorageFromConfig()
	if err != nil {
		return nil, err
	}

	aserver := alert.NewAlertServer(g, alertApiHandler, wsServer, tableClient, store)
	gserver := graph.NewServer(g, wsServer)

	piClient := packet_injector.NewPacketInjectorClient(wsServer)

	server := &Server{
		HTTPServer:          httpServer,
		WSServer:            wsServer,
		GraphServer:         gserver,
		AlertServer:         aserver,
		OnDemandClient:      onDemandClient,
		FlowMappingPipeline: pipeline,
		TableClient:         tableClient,
		EmbeddedEtcd:        etcdServer,
		EtcdClient:          etcdClient,
		ProbeBundle:         probeBundle,
		Storage:             store,
	}

	updateHandler := flow.NewFlowHandler(server.flowExpireUpdate, time.Second*time.Duration(analyzerUpdate))
	expireHandler := flow.NewFlowHandler(server.flowExpireUpdate, time.Second*time.Duration(analyzerExpire))
	flowtable := flow.NewTable(updateHandler, expireHandler)
	server.FlowTable = flowtable

	api.RegisterTopologyApi("analyzer", g, httpServer, tableClient, server.Storage)

	api.RegisterFlowApi("analyzer", flowtable, server.Storage, httpServer)

	api.RegisterPacketInjectorApi("analyzer", piClient, g, httpServer)

	return server, nil
}
