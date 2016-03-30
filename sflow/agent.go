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

package sflow

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

const (
	maxDgramSize = 1500
)

type ProbePathGetter interface {
	GetProbePath(index int64) string
}

type SFlowAgent struct {
	Addr                string
	Port                int
	Graph               *graph.Graph
	AnalyzerClient      *analyzer.Client
	flowTable           *flow.FlowTable
	FlowMappingPipeline *mappings.FlowMappingPipeline
	ProbePathGetter     ProbePathGetter
	running             atomic.Value
	wg                  sync.WaitGroup
	flush               chan bool
	flushDone           chan bool
}

func (sfa *SFlowAgent) GetTarget() string {
	target := []string{sfa.Addr, strconv.FormatInt(int64(sfa.Port), 10)}
	return strings.Join(target, ":")
}

func (sfa *SFlowAgent) feedFlowTable(conn *net.UDPConn) {
	var buf [maxDgramSize]byte
	_, _, err := conn.ReadFromUDP(buf[:])
	if err != nil {
		conn.SetDeadline(time.Now().Add(1 * time.Second))
		return
	}

	p := gopacket.NewPacket(buf[:], layers.LayerTypeSFlow, gopacket.Default)
	sflowLayer := p.Layer(layers.LayerTypeSFlow)
	sflowPacket, ok := sflowLayer.(*layers.SFlowDatagram)
	if !ok {
		return
	}

	if sflowPacket.SampleCount > 0 {
		for _, sample := range sflowPacket.FlowSamples {
			var probePath string
			if sfa.ProbePathGetter != nil {
				probePath = sfa.ProbePathGetter.GetProbePath(int64(sample.InputInterface))
			}

			flows := flow.FlowsFromSFlowSample(sfa.flowTable, &sample, probePath)

			logging.GetLogger().Debugf("%d flows captured at %v", len(flows), probePath)
		}
	}
}

func (sfa *SFlowAgent) asyncFlowPipeline(flows []*flow.Flow) {
	if sfa.FlowMappingPipeline != nil {
		sfa.FlowMappingPipeline.Enhance(flows)
	}
	if sfa.AnalyzerClient != nil {
		sfa.AnalyzerClient.SendFlows(flows)
	}
}

func (sfa *SFlowAgent) start() error {
	addr := net.UDPAddr{
		Port: sfa.Port,
		IP:   net.ParseIP(sfa.Addr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		logging.GetLogger().Errorf("Unable to listen on port %d: %s", sfa.Port, err.Error())
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(1 * time.Second))

	sfa.wg.Add(1)
	defer sfa.wg.Done()

	sfa.running.Store(true)

	sfa.flowTable = flow.NewFlowTable()
	defer sfa.flowTable.UnregisterAll()

	cfgFlowtable_expire := config.GetConfig().GetInt("agent.flowTable_expire")
	sfa.flowTable.RegisterExpire(sfa.asyncFlowPipeline, time.Duration(cfgFlowtable_expire)*time.Minute)

	cfgFlowtable_update := config.GetConfig().GetInt("agent.flowTable_update")
	sfa.flowTable.RegisterUpdated(sfa.asyncFlowPipeline, time.Duration(cfgFlowtable_update)*time.Second)

	for sfa.running.Load() == true {
		select {
		case now := <-sfa.flowTable.GetExpireTicker():
			sfa.flowTable.Expire(now)
		case now := <-sfa.flowTable.GetUpdatedTicker():
			sfa.flowTable.Updated(now)
		case <-sfa.flush:
			sfa.flowTable.ExpireNow()
			sfa.flushDone <- true
		default:
			sfa.feedFlowTable(conn)
		}
	}

	return nil
}

func (sfa *SFlowAgent) Start() {
	go sfa.start()
}

func (sfa *SFlowAgent) Stop() {
	if sfa.running.Load() == true {
		sfa.running.Store(false)
		sfa.wg.Wait()
	}
}

func (sfa *SFlowAgent) Flush() {
	logging.GetLogger().Critical("Flush() MUST be called for testing purpose only, not in production")
	sfa.flush <- true
	<-sfa.flushDone
}

func (sfa *SFlowAgent) SetAnalyzerClient(a *analyzer.Client) {
	sfa.AnalyzerClient = a
}

func (sfa *SFlowAgent) SetMappingPipeline(p *mappings.FlowMappingPipeline) {
	sfa.FlowMappingPipeline = p
}

func (sfa *SFlowAgent) SetProbePathGetter(p ProbePathGetter) {
	sfa.ProbePathGetter = p
}

func NewSFlowAgent(a string, p int, g *graph.Graph) (*SFlowAgent, error) {
	sfa := &SFlowAgent{
		Addr:      a,
		Port:      p,
		Graph:     g,
		flush:     make(chan bool),
		flushDone: make(chan bool),
	}

	return sfa, nil
}

func NewSFlowAgentFromConfig(g *graph.Graph) (*SFlowAgent, error) {
	addr, port, err := config.GetHostPortAttributes("sflow", "listen")
	if err != nil {
		return nil, err
	}

	return NewSFlowAgent(addr, port, g)
}
