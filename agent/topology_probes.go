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
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/plugin"
	"github.com/skydive-project/skydive/probe"
	tp "github.com/skydive-project/skydive/topology/probes"
	"github.com/skydive-project/skydive/topology/probes/bess"
	"github.com/skydive-project/skydive/topology/probes/blockdev"
	"github.com/skydive-project/skydive/topology/probes/docker"
	"github.com/skydive-project/skydive/topology/probes/libvirt"
	"github.com/skydive-project/skydive/topology/probes/lldp"
	"github.com/skydive-project/skydive/topology/probes/lxd"
	"github.com/skydive-project/skydive/topology/probes/netlink"
	"github.com/skydive-project/skydive/topology/probes/netns"
	"github.com/skydive-project/skydive/topology/probes/neutron"
	"github.com/skydive-project/skydive/topology/probes/opencontrail"
	"github.com/skydive-project/skydive/topology/probes/ovn"
	"github.com/skydive-project/skydive/topology/probes/ovsdb"
	"github.com/skydive-project/skydive/topology/probes/runc"
	"github.com/skydive-project/skydive/topology/probes/socketinfo"
	"github.com/skydive-project/skydive/topology/probes/vpp"
)

func registerStaticProbes() {
	blockdev.Register()
	netlink.Register()
	docker.Register()
	lldp.Register()
	lxd.Register()
	neutron.Register()
	opencontrail.Register()
	ovsdb.Register()
	runc.Register()
	libvirt.Register()
	ovn.Register()
}

// NewTopologyProbe creates a new topology probe
func NewTopologyProbe(name string, ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	switch name {
	case "blockdev":
		return blockdev.NewProbe(ctx, bundle)
	case "netlink":
		return netlink.NewProbe(ctx, bundle)
	case "netns":
		return netns.NewProbe(ctx, bundle)
	case "ovsdb":
		return ovsdb.NewProbe(ctx, bundle)
	case "lxd":
		return lxd.NewProbe(ctx, bundle)
	case "docker":
		return docker.NewProbe(ctx, bundle)
	case "lldp":
		return lldp.NewProbe(ctx, bundle)
	case "neutron":
		return neutron.NewProbe(ctx, bundle)
	case "opencontrail":
		return opencontrail.NewProbe(ctx, bundle)
	case "socketinfo":
		return socketinfo.NewProbe(ctx, bundle)
	case "libvirt":
		return libvirt.NewProbe(ctx, bundle)
	case "runc":
		return runc.NewProbe(ctx, bundle)
	case "vpp":
		return vpp.NewProbe(ctx, bundle)
	case "bess":
		return bess.NewProbe(ctx, bundle)
	default:
		return nil, fmt.Errorf("unsupported probe %s", name)
	}
}

func loadPluginProbes(ctx tp.Context, bundle *probe.Bundle) error {
	plugins, err := plugin.LoadTopologyPlugins()
	if err != nil {
		return err
	}

	for _, p := range plugins {
		p.Register()
	}

	// then register agent plugins
	for _, p := range plugins {
		if p.AgentCtor != nil {
			handler, err := p.AgentCtor(ctx, bundle)
			if err != nil {
				return fmt.Errorf("Failed to instantiate plugin %s: %s", p.Name, err)
			}

			bundle.AddHandler(p.Name, handler)
		}
	}

	return nil
}

// NewTopologyProbeBundle creates a new topology probe.Bundle based on the configuration
func NewTopologyProbeBundle(g *graph.Graph, hostNode *graph.Node) (*probe.Bundle, error) {
	registerStaticProbes()

	ctx := tp.Context{
		Logger:   logging.GetLogger(),
		Config:   config.GetConfig(),
		Graph:    g,
		RootNode: hostNode,
	}

	bundle := probe.NewBundle()
	if err := loadPluginProbes(ctx, bundle); err != nil {
		return nil, err
	}

	var probeList []string
	if runtime.GOOS == "linux" {
		probeList = append(probeList, "netlink", "netns")
	}

	probeList = append(probeList, config.GetStringSlice("agent.topology.probes")...)
	logging.GetLogger().Infof("Topology probes: %v", probeList)

	if runtime.GOOS == "linux" {
		nlHandler, err := NewTopologyProbe("netlink", ctx, bundle)
		if err != nil {
			return nil, err
		}
		bundle.AddHandler("netlink", nlHandler)

		nsHandler, err := NewTopologyProbe("netns", ctx, bundle)
		if err != nil {
			return nil, err
		}
		bundle.AddHandler("netns", nsHandler)
	}

	for _, t := range probeList {
		if bundle.GetHandler(t) != nil {
			continue
		}

		handler, err := NewTopologyProbe(t, ctx, bundle)
		if err != nil {
			return nil, err
		} else if handler != nil {
			bundle.AddHandler(t, handler)
		}
	}

	return bundle, nil
}
