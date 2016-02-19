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

package probes

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/pmylund/go-cache"

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

type SFlowAgent struct {
	Addr                string
	Port                int
	Graph               *graph.Graph
	AnalyzerClient      *analyzer.Client
	FlowMappingPipeline *mappings.FlowMappingPipeline
	cache               *cache.Cache
	cacheUpdaterChan    chan int64
	done                chan bool
	running             atomic.Value
	wg                  sync.WaitGroup
}

func (sfa *SFlowAgent) GetTarget() string {
	target := []string{sfa.Addr, strconv.FormatInt(int64(sfa.Port), 10)}
	return strings.Join(target, ":")
}

func (sfa *SFlowAgent) lookupForProbePath(index int64) string {
	sfa.Graph.Lock()
	defer sfa.Graph.Unlock()

	intfs := sfa.Graph.LookupNodes(graph.Metadata{"IfIndex": index})
	if len(intfs) == 0 {
		return ""
	}

	// lookup for the interface that is a part of an ovs bridge
	for _, intf := range intfs {
		ancestors, ok := sfa.Graph.GetAncestorsTo(intf, graph.Metadata{"Type": "ovsbridge"})
		if !ok {
			continue
		}

		bridge := ancestors[2]
		ancestors, ok = sfa.Graph.GetAncestorsTo(bridge, graph.Metadata{"Type": "host"})
		if !ok {
			continue
		}

		var path string
		for i := len(ancestors) - 1; i >= 0; i-- {
			if len(path) > 0 {
				path += "/"
			}
			path += ancestors[i].Metadata()["Name"].(string)
		}

		return path
	}

	return ""
}

func (sfa *SFlowAgent) cacheUpdater() {
	sfa.wg.Add(1)
	defer sfa.wg.Done()

	logging.GetLogger().Debug("Start SFlowAgent cache updater")

	var index int64
	for sfa.running.Load() == true {
		select {
		case index = <-sfa.cacheUpdaterChan:
			logging.GetLogger().Debugf("SFlowAgent request received: %d", index)

			path := sfa.lookupForProbePath(index)
			if path != "" {
				sfa.cache.Set(strconv.FormatInt(index, 10), path, cache.DefaultExpiration)
			}

		case <-sfa.done:
			return
		}
	}
}

func (sfa *SFlowAgent) getProbePath(index int64) string {
	p, f := sfa.cache.Get(strconv.FormatInt(index, 10))
	if f {
		path := p.(string)
		return path
	}
	sfa.cacheUpdaterChan <- index

	return ""
}

func (sfa *SFlowAgent) flowExpire(f *flow.Flow) {
	/* send a special event to the analyzer */
}

func (sfa *SFlowAgent) Start() error {
	sfa.wg.Add(1)
	defer sfa.wg.Done()

	var buf [maxDgramSize]byte

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

	// start index/mac cache updater
	go sfa.cacheUpdater()

	flowtable := flow.NewFlowTable()
	cfgFlowtable_expire := config.GetConfig().GetInt("agent.flowtable_expire")
	go flowtable.AsyncExpire(sfa.flowExpire, time.Duration(cfgFlowtable_expire)*time.Minute)

	for sfa.running.Load() == true {
		_, _, err := conn.ReadFromUDP(buf[:])
		if err != nil {
			conn.SetDeadline(time.Now().Add(1 * time.Second))
			continue
		}

		p := gopacket.NewPacket(buf[:], layers.LayerTypeSFlow, gopacket.Default)
		sflowLayer := p.Layer(layers.LayerTypeSFlow)
		sflowPacket, ok := sflowLayer.(*layers.SFlowDatagram)
		if !ok {
			continue
		}

		if sflowPacket.SampleCount > 0 {
			for _, sample := range sflowPacket.FlowSamples {
				probePath := sfa.getProbePath(int64(sample.InputInterface))

				flows := flow.FLowsFromSFlowSample(flowtable, &sample, probePath)

				logging.GetLogger().Debugf("%d flows captured at %v", len(flows), probePath)

				flowtable.Update(flows)

				if sfa.FlowMappingPipeline != nil {
					sfa.FlowMappingPipeline.Enhance(flows)
				}

				if sfa.AnalyzerClient != nil {
					// FIX(safchain) add flow state cache in order to send only flow changes
					// to not flood the analyzer
					sfa.AnalyzerClient.SendFlows(flows)
				}
			}
		}
	}

	return nil
}

func (sfa *SFlowAgent) Stop() {
	sfa.running.Store(false)
	sfa.done <- true
	sfa.wg.Wait()
}

func (sfa *SFlowAgent) SetAnalyzerClient(a *analyzer.Client) {
	sfa.AnalyzerClient = a
}

func (sfa *SFlowAgent) SetMappingPipeline(p *mappings.FlowMappingPipeline) {
	sfa.FlowMappingPipeline = p
}

func NewSFlowAgent(a string, p int, g *graph.Graph, expire int, cleanup int) (*SFlowAgent, error) {
	sfa := &SFlowAgent{
		Addr:  a,
		Port:  p,
		Graph: g,
	}

	sfa.cache = cache.New(time.Duration(expire)*time.Second, time.Duration(cleanup)*time.Second)
	sfa.cacheUpdaterChan = make(chan int64, 200)
	sfa.done = make(chan bool)
	sfa.running.Store(true)

	return sfa, nil
}

func NewSFlowAgentFromConfig(g *graph.Graph) (*SFlowAgent, error) {
	addr, port, err := config.GetHostPortAttributes("sflow", "listen")
	if err != nil {
		return nil, err
	}

	expire := config.GetConfig().GetInt("cache.expire")
	cleanup := config.GetConfig().GetInt("cache.cleanup")

	return NewSFlowAgent(addr, port, g, expire, cleanup)
}
