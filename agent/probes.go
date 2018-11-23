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
	"fmt"
	"runtime"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/docker"
	"github.com/skydive-project/skydive/topology/probes/libvirt"
	"github.com/skydive-project/skydive/topology/probes/lldp"
	"github.com/skydive-project/skydive/topology/probes/lxd"
	"github.com/skydive-project/skydive/topology/probes/netlink"
	"github.com/skydive-project/skydive/topology/probes/netns"
	"github.com/skydive-project/skydive/topology/probes/neutron"
	"github.com/skydive-project/skydive/topology/probes/opencontrail"
	"github.com/skydive-project/skydive/topology/probes/ovsdb"
	"github.com/skydive-project/skydive/topology/probes/socketinfo"
	"github.com/skydive-project/skydive/topology/probes/sriov"
)

// NewTopologyProbeBundleFromConfig creates a new topology probe.Bundle based on the configuration
func NewTopologyProbeBundleFromConfig(g *graph.Graph, hostNode *graph.Node) (*probe.Bundle, error) {
	list := config.GetStringSlice("agent.topology.probes")
	logging.GetLogger().Infof("Topology probes: %v", list)

	probes := make(map[string]probe.Probe)
	bundle := probe.NewBundle(probes)

	var nsProbe *netns.Probe
	if runtime.GOOS == "linux" {
		nlProbe, err := netlink.NewProbe(g, hostNode)
		if err != nil {
			return nil, err
		}
		probes["netlink"] = nlProbe

		nsProbe, err = netns.NewProbe(g, hostNode, nlProbe)
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
			probes[t] = ovsdb.NewProbeFromConfig(g, hostNode)
		case "lxd":
			lxdURL := config.GetConfig().GetString("lxd.url")
			Probe, err := lxd.NewProbe(nsProbe, lxdURL)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize LXD probe: %s", err)
			}
			probes[t] = Probe
		case "docker":
			dockerURL := config.GetString("docker.url")
			Probe, err := docker.NewProbe(nsProbe, dockerURL)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize Docker probe: %s", err)
			}
			probes[t] = Probe
		case "lldp":
			interfaces := config.GetStringSlice("agent.topology.lldp.interfaces")
			lldpProbe, err := lldp.NewProbe(g, hostNode, interfaces)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize LLDP probe: %s", err)
			}
			probes[t] = lldpProbe
		case "neutron":
			neutron, err := neutron.NewProbeFromConfig(g)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize Neutron probe: %s", err)
			}
			probes["neutron"] = neutron
		case "opencontrail":
			opencontrail, err := opencontrail.NewProbeFromConfig(g, hostNode)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize OpenContrail probe: %s", err)
			}
			probes[t] = opencontrail
		case "socketinfo":
			probes[t] = socketinfo.NewSocketInfoProbe(g, hostNode)
		case "libvirt":
			libvirt, err := libvirt.NewProbeFromConfig(g, hostNode)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize Libvirt probe: %s", err)
			}
			probes[t] = libvirt
		case "sriov":
			sriov, err := sriov.NewProbe(g, hostNode)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize Sriov probe: %s", err)
			}
			probes[t] = sriov
		default:
			logging.GetLogger().Errorf("unknown probe type %s", t)
		}
	}

	return bundle, nil
}
