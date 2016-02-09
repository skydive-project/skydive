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

type SFlowProbe struct {
	Addr string
	Port int

	Graph               *graph.Graph
	AnalyzerClient      *analyzer.Client
	FlowMappingPipeline *mappings.FlowMappingPipeline

	cache            *cache.Cache
	cacheUpdaterChan chan uint32

	done    chan bool
	running atomic.Value
	wg      sync.WaitGroup
}

func (probe *SFlowProbe) GetTarget() string {
	target := []string{probe.Addr, strconv.FormatInt(int64(probe.Port), 10)}
	return strings.Join(target, ":")
}

func (probe *SFlowProbe) cacheUpdater() {
	probe.wg.Add(1)
	defer probe.wg.Done()

	logging.GetLogger().Debug("Start SFlowProbe cache updater")

	var index uint32
	for probe.running.Load() == true {
		select {
		case index = <-probe.cacheUpdaterChan:
			logging.GetLogger().Debug("SFlowProbe request received: %d", index)

			probe.Graph.Lock()

			intfs := probe.Graph.LookupNodes(graph.Metadatas{"IfIndex": index})

			// lookup for the interface that is a part of an ovs bridge
			for _, intf := range intfs {
				ancestors, ok := probe.Graph.GetAncestorsTo(intf, graph.Metadatas{"Type": "ovsbridge"})
				if ok {
					bridge := ancestors[2]
					ancestors, ok = probe.Graph.GetAncestorsTo(bridge, graph.Metadatas{"Type": "host"})

					var path string
					for i := len(ancestors) - 1; i >= 0; i-- {
						if len(path) > 0 {
							path += "/"
						}
						path += ancestors[i].Metadatas()["Name"].(string)
					}
					probe.cache.Set(strconv.FormatUint(uint64(index), 10), path, cache.DefaultExpiration)
					break
				}
			}
			probe.Graph.Unlock()

		case <-probe.done:
			return
		}
	}
}

func (probe *SFlowProbe) getProbePath(index uint32) *string {
	p, f := probe.cache.Get(strconv.FormatUint(uint64(index), 10))
	if f {
		path := p.(string)
		return &path
	}

	probe.cacheUpdaterChan <- index

	return nil
}

func (probe *SFlowProbe) flowExpire(f *flow.Flow) {
	/* send a special event to the analyzer */
}

func (probe *SFlowProbe) Start() error {
	probe.wg.Add(1)
	defer probe.wg.Done()

	var buf [maxDgramSize]byte

	addr := net.UDPAddr{
		Port: probe.Port,
		IP:   net.ParseIP(probe.Addr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		logging.GetLogger().Error("Unable to listen on port %d: %s", probe.Port, err.Error())
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(1 * time.Second))

	// start index/mac cache updater
	go probe.cacheUpdater()

	flowtable := flow.NewFlowTable()
	cfgFlowtable_expire, err := config.GetConfig().Section("agent").Key("flowtable_expire").Int()
	if err != nil || cfgFlowtable_expire < 1 {
		logging.GetLogger().Error("Config flowTable_expire invalid value ", cfgFlowtable_expire, err.Error())
		return err
	}
	go flowtable.AsyncExpire(probe.flowExpire, time.Duration(cfgFlowtable_expire)*time.Minute)

	for probe.running.Load() == true {
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
				flows := flow.FLowsFromSFlowSample(flowtable, &sample, probe.getProbePath(sample.InputInterface))

				logging.GetLogger().Debug("%d flows captured", len(flows))

				flowtable.Update(flows)

				if probe.FlowMappingPipeline != nil {
					probe.FlowMappingPipeline.Enhance(flows)
				}

				if probe.AnalyzerClient != nil {
					// FIX(safchain) add flow state cache in order to send only flow changes
					// to not flood the analyzer
					probe.AnalyzerClient.SendFlows(flows)
				}
			}
		}
	}

	return nil
}

func (probe *SFlowProbe) Stop() {
	probe.running.Store(false)
	probe.done <- true
	probe.wg.Wait()
}

func (probe *SFlowProbe) SetAnalyzerClient(a *analyzer.Client) {
	probe.AnalyzerClient = a
}

func (probe *SFlowProbe) SetMappingPipeline(p *mappings.FlowMappingPipeline) {
	probe.FlowMappingPipeline = p
}

func NewSFlowProbe(a string, p int, g *graph.Graph, expire int, cleanup int) (*SFlowProbe, error) {
	probe := &SFlowProbe{
		Addr:  a,
		Port:  p,
		Graph: g,
	}

	probe.cache = cache.New(time.Duration(expire)*time.Second, time.Duration(cleanup)*time.Second)
	probe.cacheUpdaterChan = make(chan uint32, 200)
	probe.done = make(chan bool)
	probe.running.Store(true)

	return probe, nil
}

func NewSFlowProbeFromConfig(g *graph.Graph) (*SFlowProbe, error) {
	addr, port, err := config.GetHostPortAttributes("sflow", "listen")
	if err != nil {
		return nil, err
	}

	expire, err := config.GetConfig().Section("cache").Key("expire").Int()
	if err != nil {
		return nil, err
	}
	cleanup, err := config.GetConfig().Section("cache").Key("cleanup").Int()
	if err != nil {
		return nil, err
	}

	return NewSFlowProbe(addr, port, g, expire, cleanup)
}
