/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"fmt"

	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/mappings"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

type FlowProbeBundle struct {
	probe.ProbeBundle
	Graph              *graph.Graph
	FlowTableAllocator *flow.TableAllocator
}

type FlowProbeInterface interface {
	probe.Probe
	RegisterProbe(n *graph.Node, capture *api.Capture, ft *flow.Table) error
	UnregisterProbe(n *graph.Node) error
}

type FlowProbe struct {
	fpi            FlowProbeInterface
	pipeline       *mappings.FlowMappingPipeline
	flowClientPool *analyzer.FlowClientPool
}

func (fp *FlowProbe) Start() {
	fp.fpi.Start()
}

func (fp *FlowProbe) Stop() {
	fp.fpi.Stop()
}

func (fp *FlowProbe) RegisterProbe(n *graph.Node, capture *api.Capture, ft *flow.Table) error {
	return fp.fpi.RegisterProbe(n, capture, ft)
}

func (fp *FlowProbe) UnregisterProbe(n *graph.Node) error {
	return fp.fpi.UnregisterProbe(n)
}

func (fp *FlowProbe) AsyncFlowPipeline(flows []*flow.Flow) {
	fp.pipeline.Enhance(flows)
	fp.flowClientPool.SendFlows(flows)
}

func (fpb *FlowProbeBundle) UnregisterAllProbes() {
	fpb.Graph.Lock()
	defer fpb.Graph.Unlock()

	for _, n := range fpb.Graph.GetNodes(graph.Metadata{}) {
		for _, p := range fpb.ProbeBundle.Probes {
			fprobe := p.(*FlowProbe)
			fprobe.UnregisterProbe(n)
		}
	}
}

func NewFlowProbeBundleFromConfig(tb *probe.ProbeBundle, g *graph.Graph, fta *flow.TableAllocator, fcpool *analyzer.FlowClientPool) *FlowProbeBundle {
	list := config.GetConfig().GetStringSlice("agent.flow.probes")
	logging.GetLogger().Infof("Flow probes: %v", list)

	pipeline := mappings.NewFlowMappingPipeline(mappings.NewGraphFlowEnhancer(g))

	// check that the neutron probe if loaded if so add the neutron flow enhancer
	if tb.GetProbe("neutron") != nil {
		pipeline.AddEnhancer(mappings.NewNeutronFlowEnhancer(g))
	}

	var captureTypes []string
	var fpi FlowProbeInterface
	var err error

	probes := make(map[string]probe.Probe)
	for _, t := range list {
		if _, ok := probes[t]; ok {
			continue
		}

		switch t {
		case "pcapsocket":
			fpi, err = NewPcapSocketProbeHandler(g)
			captureTypes = []string{"pcapsocket"}
		case "ovssflow":
			fpi, err = NewOvsSFlowProbesHandler(tb, g)
			captureTypes = []string{"ovssflow"}
		case "gopacket":
			fpi, err = NewGoPacketProbesHandler(g)
			captureTypes = []string{"afpacket", "pcap"}
		default:
			err = fmt.Errorf("unknown probe type %s", t)
		}

		if err != nil {
			logging.GetLogger().Errorf("failed to create %s probe: %s", t, err.Error())
			continue
		}

		flowProbe := &FlowProbe{fpi: fpi, pipeline: pipeline, flowClientPool: fcpool}
		for _, captureType := range captureTypes {
			probes[captureType] = flowProbe
		}
	}

	p := probe.NewProbeBundle(probes)

	return &FlowProbeBundle{
		ProbeBundle:        *p,
		Graph:              g,
		FlowTableAllocator: fta,
	}
}
