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
	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow/mappings"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/probe"
	"github.com/redhat-cip/skydive/topology/graph"
	"github.com/redhat-cip/skydive/topology/probes"
)

type FlowProbeBundle struct {
	probe.ProbeBundle
}

func NewFlowProbeBundleFromConfig(tb *probes.TopologyProbeBundle, g *graph.Graph) *FlowProbeBundle {
	list := config.GetConfig().GetStringSlice("agent.flow.probes")

	logging.GetLogger().Infof("Flow probes: %v", list)

	gfe, err := mappings.NewGraphFlowEnhancer(g)
	if err != nil {
		panic(err)
	}

	pipeline := mappings.NewFlowMappingPipeline(gfe)

	var aclient *analyzer.Client

	addr, port, err := config.GetAnalyzerClientAddr()
	if err != nil {
		logging.GetLogger().Errorf("Unable to parse analyzer client: %s", err.Error())
		return nil
	}

	if addr != "" {
		aclient, err = analyzer.NewClient(addr, port)
		if err != nil {
			logging.GetLogger().Errorf("Analyzer client error %s:%d : %s", addr, port, err.Error())
			return nil
		}
	}

	probes := make(map[string]probe.Probe)
	for _, t := range list {
		if _, ok := probes[t]; ok {
			continue
		}

		switch t {
		case "ovssflow":
			o := NewOvsSFlowProbesHandlerFromConfig(tb, g, pipeline, aclient)
			if o != nil {
				probes[t] = o
			}
		case "pcap":
			o := NewPcapProbesHandler(tb, g, pipeline, aclient)
			if o != nil {
				probes[t] = o
			}
		default:
			logging.GetLogger().Error("unknown probe type %s", t)
		}
	}

	p := probe.NewProbeBundle(probes)

	return &FlowProbeBundle{
		ProbeBundle: *p,
	}
}
