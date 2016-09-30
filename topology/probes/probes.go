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
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

type TopologyProbeBundle struct {
	probe.ProbeBundle
}

func NewTopologyProbeBundleFromConfig(g *graph.Graph, n *graph.Node) *TopologyProbeBundle {
	list := config.GetConfig().GetStringSlice("agent.topology.probes")

	// FIX(safchain) once viper setdefault on nested key will be fixed move this
	// to config init
	if len(list) == 0 {
		list = []string{"fabric", "netlink", "netns"}
	}

	logging.GetLogger().Infof("Topology probes: %v", list)

	probes := make(map[string]probe.Probe)
	for _, t := range list {
		if _, ok := probes[t]; ok {
			continue
		}

		switch t {
		case "netlink":
			probes[t] = NewNetLinkProbe(g, n)
		case "netns":
			probes[t] = NewNetNSProbeFromConfig(g, n)
		case "ovsdb":
			probes[t] = NewOvsdbProbeFromConfig(g, n)
		case "docker":
			probes[t] = NewDockerProbeFromConfig(g, n)
		case "neutron":
			neutron, err := NewNeutronMapperFromConfig(g)
			if err != nil {
				logging.GetLogger().Errorf("Failed to initialize Neutron probe: %s", err.Error())
				continue
			}
			probes[t] = neutron
		case "fabric":
			probes[t] = NewFabricProbe(g)
		case "opencontrail":
			probes[t] = NewOpenContrailMapper(g, n)
		default:
			logging.GetLogger().Errorf("unknown probe type %s", t)
		}
	}

	p := probe.NewProbeBundle(probes)

	return &TopologyProbeBundle{*p}
}
