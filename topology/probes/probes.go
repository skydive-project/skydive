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
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

type Probe interface {
	Start()
	Stop()
}

type ProbeBundle struct {
	probes map[string]Probe
}

func (p *ProbeBundle) Start() {
	for _, p := range p.probes {
		p.Start()
	}
}

func (p *ProbeBundle) Stop() {
	for _, p := range p.probes {
		p.Stop()
	}
}

func (p *ProbeBundle) GetProbe(k string) Probe {
	if probe, ok := p.probes[k]; ok {
		return probe
	}
	return nil
}

func NewProbeBundle(p map[string]Probe) *ProbeBundle {
	return &ProbeBundle{
		probes: p,
	}
}

func NewProbeBundleFromConfig(g *graph.Graph, n *graph.Node) *ProbeBundle {
	list := config.GetConfig().GetStringSlice("agent.topology.probes")

	// FIX(safchain) once viper setdefault on nested key will be fixed move this
	// to config init
	if len(list) == 0 {
		list = []string{"netlink", "netns"}
	}

	logging.GetLogger().Infof("Topology probes: %v", list)

	probes := make(map[string]Probe)
	for _, t := range list {
		if _, ok := probes[t]; ok {
			continue
		}

		switch t {
		case "netlink":
			probes[t] = NewNetLinkProbe(g, n)
		case "netns":
			probes[t] = NewNetNSProbe(g, n)
		case "ovsdb":
			probes[t] = NewOvsdbProbeFromConfig(g, n)
		default:
			logging.GetLogger().Error("unknown probe type %s", t)
		}
	}

	return NewProbeBundle(probes)
}
