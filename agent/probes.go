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
	"fmt"
	"runtime"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/probes/docker"
	"github.com/skydive-project/skydive/topology/probes/docker/subprobes"
	docker_vpp "github.com/skydive-project/skydive/topology/probes/docker/subprobes/vpp"
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
			addr := config.GetString("ovs.ovsdb")
			enableStats := config.GetBool("ovs.enable_stats")
			ovsProbe, err := ovsdb.NewProbeFromConfig(g, hostNode, addr, enableStats)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize OVS probe: %s", err)
			}
			probes[t] = ovsProbe
		case "lxd":
			lxdURL := config.GetConfig().GetString("lxd.url")
			lxdProbe, err := lxd.NewProbe(nsProbe, lxdURL)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize LXD probe: %s", err)
			}
			probes[t] = lxdProbe
		case "docker":
			dockerURL := config.GetString("agent.topology.docker.url")
			netnsRunPath := config.GetString("agent.topology.docker.netns.run_path")
			subprobes := dockerSubprobes(nsProbe)
			dockerProbe, err := docker.NewProbe(nsProbe, dockerURL, netnsRunPath, subprobes)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize Docker probe: %s", err)
			}
			probes[t] = dockerProbe
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
		case "runc":
			runc, err := runc.NewProbe(nsProbe)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize runc probe: %s", err)
			}
			probes[t] = runc
		case "vpp":
			vpp, err := vpp.NewProbeFromConfig(g, hostNode)
			if err != nil {
				return nil, fmt.Errorf("Failed to initialize vpp probe: %s", err)
			}
			probes[t] = vpp
		default:
			logging.GetLogger().Errorf("unknown probe type %s", t)
		}
	}

	return bundle, nil
}

// dockerSubprobes create all docker related subprobes
func dockerSubprobes(nsProbe *netns.Probe) []subprobes.Subprobe {
	subprobes := make([]subprobes.Subprobe, 0)
	if vpp, err := docker_vpp.NewSubprobe(nsProbe); err != nil {
		logging.GetLogger().Warningf("VPP subprobe in docker probe will be disabled because its creation failed: %v", err)
	} else {
		subprobes = append(subprobes, vpp)
	}
	return subprobes
}
