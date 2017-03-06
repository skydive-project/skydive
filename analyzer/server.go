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
	"github.com/skydive-project/skydive/common"
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
)

type Server struct {
	shttp.DefaultWSServerEventHandler
	HTTPServer          *shttp.Server
	WSServer            *shttp.WSServer
	TopologyForwarder   *TopologyForwarder
	TopologyServer      *TopologyServer
	AlertServer         *alert.AlertServer
	OnDemandClient      *ondemand.OnDemandProbeClient
	FlowMappingPipeline *mappings.FlowMappingPipeline
	ProbeBundle         *probe.ProbeBundle
	Storage             storage.Storage
	FlowTable           *flow.Table
	TableClient         *flow.TableClient
	conn                *FlowServerConn
	EmbeddedEtcd        *etcd.EmbeddedEtcd
	EtcdClient          *etcd.EtcdClient
	running             atomic.Value
	wgServers           sync.WaitGroup
	wgFlowsHandlers     sync.WaitGroup
}

func (s *Server) flowExpireUpdate(flows []*flow.Flow) {
	if s.Storage != nil && len(flows) > 0 {
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
func (s *Server) handleFlowPacket(conn *FlowServerConn) {
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

	s.TopologyForwarder.ConnectAll()

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
	s.conn, err = NewFlowServerConn(addr)
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

	httpServer, err := shttp.NewServerFromConfig(common.AnalyzerService)
	if err != nil {
		return nil, err
	}

	wsServer := shttp.NewWSServerFromConfig(common.AnalyzerService, httpServer, "/ws")

	topology := NewTopologyServerFromConfig(wsServer)

	probeBundle, err := NewTopologyProbeBundleFromConfig(topology.Graph)
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

	if err = etcdClient.SetInt64("/agent/config/flowtable_update", agentUpdate); err != nil {
		return nil, err
	}

	if err = etcdClient.SetInt64("/agent/config/flowtable_expire", agentExpire); err != nil {
		return nil, err
	}

	apiServer, err := api.NewAPI(httpServer, etcdClient.KeysAPI, "Analyzer")
	if err != nil {
		return nil, err
	}

	var captureAPIHandler *api.CaptureAPIHandler
	if captureAPIHandler, err = api.RegisterCaptureAPI(apiServer, topology.Graph); err != nil {
		return nil, err
	}

	var alertAPIHandler *api.AlertAPIHandler
	if alertAPIHandler, err = api.RegisterAlertAPI(apiServer); err != nil {
		return nil, err
	}

	onDemandClient := ondemand.NewOnDemandProbeClient(topology.Graph, captureAPIHandler, wsServer, etcdClient)

	pipeline := mappings.NewFlowMappingPipeline(mappings.NewGraphFlowEnhancer(topology.Graph))

	// check that the neutron probe is loaded if so add the neutron flow enhancer
	if probeBundle.GetProbe("neutron") != nil {
		pipeline.AddEnhancer(mappings.NewNeutronFlowEnhancer(topology.Graph))
	}

	tableClient := flow.NewTableClient(wsServer)
	store, err := storage.NewStorageFromConfig()
	if err != nil {
		return nil, err
	}

	aserver := alert.NewAlertServer(topology.Graph, alertAPIHandler, wsServer, tableClient, store, etcdClient)

	piClient := packet_injector.NewPacketInjectorClient(wsServer)

	forwarder := NewTopologyForwarderFromConfig(topology.Graph, wsServer)

	server := &Server{
		HTTPServer:          httpServer,
		WSServer:            wsServer,
		TopologyForwarder:   forwarder,
		TopologyServer:      topology,
		AlertServer:         aserver,
		OnDemandClient:      onDemandClient,
		FlowMappingPipeline: pipeline,
		TableClient:         tableClient,
		EmbeddedEtcd:        etcdServer,
		EtcdClient:          etcdClient,
		ProbeBundle:         probeBundle,
		Storage:             store,
	}

	wsServer.AddEventHandler(server)

	updateHandler := flow.NewFlowHandler(server.flowExpireUpdate, time.Second*time.Duration(analyzerUpdate))
	expireHandler := flow.NewFlowHandler(server.flowExpireUpdate, time.Second*time.Duration(analyzerExpire))
	flowtable := flow.NewTable(updateHandler, expireHandler)
	server.FlowTable = flowtable

	api.RegisterTopologyAPI(topology.Graph, httpServer, tableClient, server.Storage)

	api.RegisterFlowAPI(flowtable, server.Storage, httpServer)

	api.RegisterPacketInjectorAPI(piClient, topology.Graph, httpServer)

	api.RegisterPcapAPI(httpServer, flowtable.PacketsChan)

	api.RegisterConfigAPI(httpServer)

	return server, nil
}
