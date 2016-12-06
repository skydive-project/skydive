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

package agent

import (
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
	tprobes "github.com/skydive-project/skydive/topology/probes"
)

func NewTopologyProbeBundleFromConfig(g *graph.Graph, n *graph.Node, wsClient *shttp.WSAsyncClient) (*probe.ProbeBundle, error) {
	list := config.GetConfig().GetStringSlice("agent.topology.probes")
	logging.GetLogger().Infof("Topology probes: %v", list)

	probes := make(map[string]probe.Probe)
	bundle := probe.NewProbeBundle(probes)

	for _, t := range list {
		if _, ok := probes[t]; ok {
			continue
		}

		switch t {
		case "netlink":
			probes[t] = tprobes.NewNetLinkProbe(g, n)
		case "netns":
			nsProbe, err := tprobes.NewNetNSProbeFromConfig(g, n)
			if err != nil {
				return nil, err
			}
			probes[t] = nsProbe
		case "ovsdb":
			probes[t] = tprobes.NewOvsdbProbeFromConfig(g, n)
		case "docker":
			dockerProbe, err := tprobes.NewDockerProbeFromConfig(g, n)
			if err != nil {
				return nil, err
			}
			probes[t] = dockerProbe
		case "neutron":
			neutron, err := tprobes.NewNeutronMapperFromConfig(g, wsClient)
			if err != nil {
				logging.GetLogger().Errorf("Failed to initialize Neutron probe: %s", err.Error())
				return nil, err
			}
			probes["neutron"] = neutron
		case "opencontrail":
			probes[t] = tprobes.NewOpenContrailMapper(g, n)
		default:
			logging.GetLogger().Errorf("unknown probe type %s", t)
		}
	}

	return bundle, nil
}
