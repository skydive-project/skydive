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
}

func (probe *SFlowProbe) GetTarget() string {
	target := []string{probe.Addr, strconv.FormatInt(int64(probe.Port), 10)}
	return strings.Join(target, ":")
}

func (probe *SFlowProbe) cacheUpdater() {
	logging.GetLogger().Debug("Start SFlowProbe cache updater")

	var index uint32
	for {
		index = <-probe.cacheUpdaterChan

		logging.GetLogger().Debug("SFlowProbe request received: %d", index)

		probe.Graph.Lock()

		intfs := probe.Graph.LookupNodes(graph.Metadatas{"IfIndex": index})

		// lookup for the interface that is a part of an ovs bridge
		for _, intf := range intfs {
			ancestors, ok := intf.GetAncestorsTo(graph.Metadatas{"Type": "ovsbridge"})
			if ok {
				bridge := ancestors[2]
				ancestors, ok = bridge.GetAncestorsTo(graph.Metadatas{"Type": "host"})

				var path string
				for i := len(ancestors) - 1; i >= 0; i-- {
					if len(path) > 0 {
						path += "/"
					}
					path += ancestors[i].Metadatas["Name"].(string)
				}
				probe.cache.Set(strconv.FormatUint(uint64(index), 10), path, cache.DefaultExpiration)
				break
			}
		}
		probe.Graph.Unlock()
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

func (probe *SFlowProbe) Start() error {
	var buf [maxDgramSize]byte

	addr := net.UDPAddr{
		Port: probe.Port,
		IP:   net.ParseIP(probe.Addr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	defer conn.Close()
	if err != nil {
		logging.GetLogger().Error("Unable to listen on port %d: %s", probe.Port, err.Error())
		return err
	}

	// start index/mac cache updater
	go probe.cacheUpdater()

	for {
		_, _, err := conn.ReadFromUDP(buf[:])
		if err != nil {
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
				flows := flow.FLowsFromSFlowSample(&sample, probe.getProbePath(sample.InputInterface))

				logging.GetLogger().Debug("%d flows captured", len(flows))

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

func (probe *SFlowProbe) SetAnalyzerClient(a *analyzer.Client) {
	probe.AnalyzerClient = a
}

func (probe *SFlowProbe) SetMappingPipeline(p *mappings.FlowMappingPipeline) {
	probe.FlowMappingPipeline = p
}

func NewSFlowProbe(a string, p int, g *graph.Graph) (*SFlowProbe, error) {
	probe := &SFlowProbe{
		Addr:  a,
		Port:  p,
		Graph: g,
	}

	expire, err := config.GetConfig().Section("cache").Key("expire").Int()
	if err != nil {
		return nil, err
	}
	cleanup, err := config.GetConfig().Section("cache").Key("cleanup").Int()
	if err != nil {
		return nil, err
	}

	probe.cache = cache.New(time.Duration(expire)*time.Second, time.Duration(cleanup)*time.Second)
	probe.cacheUpdaterChan = make(chan uint32, 200)

	return probe, nil
}
