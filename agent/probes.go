/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package agent

import (
	"runtime"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/probes/docker"
	"github.com/skydive-project/skydive/topology/probes/libvirt"
	"github.com/skydive-project/skydive/topology/probes/lldp"
	"github.com/skydive-project/skydive/topology/probes/lxd"
	"github.com/skydive-project/skydive/topology/probes/netlink"
	"github.com/skydive-project/skydive/topology/probes/netns"
	"github.com/skydive-project/skydive/topology/probes/neutron"
	"github.com/skydive-project/skydive/topology/probes/opencontrail"
	"github.com/skydive-project/skydive/topology/probes/ovsdb"
	"github.com/skydive-project/skydive/topology/probes/runc"
	"github.com/skydive-project/skydive/topology/probes/socketinfo"
	"github.com/skydive-project/skydive/topology/probes/vpp"
)

// NewTopologyProbeBundleFromConfig creates a new topology probe.Bundle based on the configuration
func NewTopologyProbeBundleFromConfig(g *graph.Graph, hostNode *graph.Node) (*probe.Bundle, error) {
	list := config.GetStringSlice("agent.topology.probes")
	logging.GetLogger().Infof("Topology probes: %v", list)

	var handler probe.Handler
	var err error

	bundle := probe.NewBundle()

	var nsProbe *netns.Probe
	if runtime.GOOS == "linux" {
		nlHandler, err := netlink.NewProbeHandler(g, hostNode)
		if err != nil {
			return nil, err
		}
		bundle.AddHandler("netlink", handler)

		nsHandler, err := netns.NewProbe(g, hostNode, nlHandler)
		if err != nil {
			return nil, err
		}
		bundle.AddHandler("netns", nsHandler)
	}

	for _, t := range list {
		if bundle.GetHandler(t) != nil {
			continue
		}

		switch t {
		case "ovsdb":
			addr := config.GetString("ovs.ovsdb")
			enableStats := config.GetBool("ovs.enable_stats")
			handler, err = ovsdb.NewProbeFromConfig(g, hostNode, addr, enableStats)
		case "lxd":
			lxdURL := config.GetConfig().GetString("lxd.url")
			handler, err = lxd.NewProbe(nsProbe, lxdURL)
		case "docker":
			dockerURL := config.GetString("agent.topology.docker.url")
			netnsRunPath := config.GetString("agent.topology.docker.netns.run_path")
			handler, err = docker.NewProbe(nsProbe, dockerURL, netnsRunPath)
		case "lldp":
			interfaces := config.GetStringSlice("agent.topology.lldp.interfaces")
			handler, err = lldp.NewProbe(g, hostNode, interfaces)
		case "neutron":
			handler, err = neutron.NewProbeFromConfig(g)
		case "opencontrail":
			handler, err = opencontrail.NewProbeFromConfig(g, hostNode)
		case "socketinfo":
			handler = socketinfo.NewSocketInfoProbe(g, hostNode)
		case "libvirt":
			handler, err = libvirt.NewProbeFromConfig(g, hostNode)
		case "runc":
			handler, err = runc.NewProbe(nsProbe)
		case "vpp":
			handler, err = vpp.NewProbeFromConfig(g, hostNode)
		default:
			logging.GetLogger().Errorf("unknown probe type %s", t)
		}

		if err != nil {
			return nil, err
		}
		if handler != nil {
			bundle.AddHandler(t, handler)
		}
	}

	return bundle, nil
}
