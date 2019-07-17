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
	tp "github.com/skydive-project/skydive/topology/probes"
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

// NewTopologyProbeBundle creates a new topology probe.Bundle based on the configuration
func NewTopologyProbeBundle(g *graph.Graph, hostNode *graph.Node) (*probe.Bundle, error) {
	list := config.GetStringSlice("agent.topology.probes")
	logging.GetLogger().Infof("Topology probes: %v", list)

	var handler probe.Handler
	var err error

	bundle := probe.NewBundle()
	ctx := tp.Context{
		Logger:   logging.GetLogger(),
		Config:   config.GetConfig(),
		Graph:    g,
		RootNode: hostNode,
	}

	if runtime.GOOS == "linux" {
		nlHandler, err := new(netlink.ProbeHandler).Init(ctx, bundle)
		if err != nil {
			return nil, err
		}
		bundle.AddHandler("netlink", nlHandler)

		nsHandler, err := new(netns.ProbeHandler).Init(ctx, bundle)
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
			handler, err = new(ovsdb.Probe).Init(ctx, bundle)
		case "lxd":
			handler, err = new(lxd.ProbeHandler).Init(ctx, bundle)
		case "docker":
			handler, err = new(docker.ProbeHandler).Init(ctx, bundle)
		case "lldp":
			handler, err = new(lldp.Probe).Init(ctx, bundle)
		case "neutron":
			handler, err = new(neutron.Probe).Init(ctx, bundle)
		case "opencontrail":
			handler, err = new(opencontrail.Probe).Init(ctx, bundle)
		case "socketinfo":
			handler, err = new(socketinfo.ProbeHandler).Init(ctx, bundle)
		case "libvirt":
			handler, err = new(libvirt.Probe).Init(ctx, bundle)
		case "runc":
			handler, err = new(runc.ProbeHandler).Init(ctx, bundle)
		case "vpp":
			handler, err = new(vpp.Probe).Init(ctx, bundle)
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
