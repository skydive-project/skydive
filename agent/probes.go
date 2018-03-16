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
	"runtime"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/docker"
	"github.com/skydive-project/skydive/topology/probes/lxd"
	"github.com/skydive-project/skydive/topology/probes/netlink"
	"github.com/skydive-project/skydive/topology/probes/netns"
	"github.com/skydive-project/skydive/topology/probes/neutron"
	"github.com/skydive-project/skydive/topology/probes/opencontrail"
	"github.com/skydive-project/skydive/topology/probes/ovsdb"
	"github.com/skydive-project/skydive/topology/probes/socketinfo"
)

// NewTopologyProbeBundleFromConfig creates a new topology probe.ProbeBundle based on the configuration
func NewTopologyProbeBundleFromConfig(g *graph.Graph, n *graph.Node) (*probe.ProbeBundle, error) {
	list := config.GetStringSlice("agent.topology.probes")
	logging.GetLogger().Infof("Topology probes: %v", list)

	probes := make(map[string]probe.Probe)
	bundle := probe.NewProbeBundle(probes)

	var nsProbe *netns.NetNSProbe
	if runtime.GOOS == "linux" {
		nlProbe, err := netlink.NewNetLinkProbe(g, n)
		if err != nil {
			return nil, err
		}
		probes["netlink"] = nlProbe

		nsProbe, err = netns.NewNetNSProbe(g, n, nlProbe)
		if err != nil {
			return nil, err
		}
		probes["netns"] = nsProbe
	}

	for _, t := range list {
		if _, ok := probes[t]; ok {
			continue
		}

		switch t {
		case "ovsdb":
			probes[t] = ovsdb.NewOvsdbProbeFromConfig(g, n)
		case "lxd":
			lxdURL := config.GetConfig().GetString("lxd.url")
			lxdProbe, err := lxd.NewLxdProbe(nsProbe, lxdURL)
			if err != nil {
				return nil, err
			}
			probes[t] = lxdProbe
		case "docker":
			dockerURL := config.GetString("docker.url")
			dockerProbe, err := docker.NewDockerProbe(nsProbe, dockerURL)
			if err != nil {
				return nil, err
			}
			probes[t] = dockerProbe
		case "neutron":
			neutron, err := neutron.NewNeutronProbeFromConfig(g)
			if err != nil {
				logging.GetLogger().Errorf("Failed to initialize Neutron probe: %s", err.Error())
				return nil, err
			}
			probes["neutron"] = neutron
		case "opencontrail":
			opencontrail, err := opencontrail.NewOpenContrailProbeFromConfig(g, n)
			if err != nil {
				logging.GetLogger().Errorf("Failed to initialize OpenContrail probe: %s", err.Error())
				return nil, err
			}
			probes[t] = opencontrail
		case "socketinfo":
			probes[t] = socketinfo.NewSocketInfoProbe(g, n)
		default:
			logging.GetLogger().Errorf("unknown probe type %s", t)
		}
	}

	return bundle, nil
}
