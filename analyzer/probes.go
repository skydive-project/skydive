/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
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

package analyzer

import (
	"fmt"

	"github.com/skydive-project/skydive/config"
	fp "github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/packetinjector"
	"github.com/skydive-project/skydive/plugin"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/sflow"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes/blockdev"
	"github.com/skydive-project/skydive/topology/probes/fabric"
	"github.com/skydive-project/skydive/topology/probes/istio"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	"github.com/skydive-project/skydive/topology/probes/libvirt"
	"github.com/skydive-project/skydive/topology/probes/lldp"
	"github.com/skydive-project/skydive/topology/probes/lxd"
	"github.com/skydive-project/skydive/topology/probes/netlink"
	"github.com/skydive-project/skydive/topology/probes/neutron"
	"github.com/skydive-project/skydive/topology/probes/opencontrail"
	"github.com/skydive-project/skydive/topology/probes/ovn"
	"github.com/skydive-project/skydive/topology/probes/ovnk8s"
	"github.com/skydive-project/skydive/topology/probes/ovsdb"
	"github.com/skydive-project/skydive/topology/probes/peering"
)

func registerStaticProbes() {
	netlink.Register()
	blockdev.Register()
	topology.RegisterContainer()
	lldp.Register()
	lxd.Register()
	neutron.Register()
	opencontrail.Register()
	ovsdb.Register()
	libvirt.Register()
	ovn.Register()
}

func registerPluginProbes() error {
	plugins, err := plugin.LoadTopologyPlugins()
	if err != nil {
		return err
	}

	for _, p := range plugins {
		p.Register()
	}

	return nil
}

// RegisterProbes register graph metadata decoders
func registerProbes() error {
	registerStaticProbes()

	if err := registerPluginProbes(); err != nil {
		return err
	}

	graph.NodeMetadataDecoders["Captures"] = fp.CapturesMetadataDecoder
	graph.NodeMetadataDecoders["PacketInjections"] = packetinjector.InjectionsMetadataDecoder

	// TODO move it when flow probe plugin will be introduced
	graph.NodeMetadataDecoders["SFlow"] = sflow.SFMetadataDecoder

	return nil
}

// NewTopologyProbeBundleFromConfig creates a new topology server probes from configuration
func NewTopologyProbeBundleFromConfig(g *graph.Graph) (*probe.Bundle, error) {
	if err := registerProbes(); err != nil {
		return nil, err
	}

	list := config.GetStringSlice("analyzer.topology.probes")

	var handler probe.Handler
	var err error

	bundle := probe.NewBundle()

	fabricProbe, err := fabric.NewProbe(g)
	if err != nil {
		return nil, err
	}
	bundle.AddHandler("fabric", fabricProbe)
	bundle.AddHandler("peering", peering.NewProbe(g))

	for _, t := range list {
		if bundle.GetHandler(t) != nil {
			continue
		}

		switch t {
		case "ovn":
			addr := config.GetString("analyzer.topology.ovn.address")
			certFile := config.GetString("analyzer.topology.ovn.cert")
			keyFile := config.GetString("analyzer.topology.ovn.key")
			cacertFile := config.GetString("analyzer.topology.ovn.cacert")
			handler, err = ovn.NewProbe(g, addr, certFile, keyFile, cacertFile)
		case "ovnk8s":
			handler, err = ovnk8s.NewProbe(g)
		case "k8s":
			handler, err = k8s.NewK8sProbe(g)
		case "istio":
			handler, err = istio.NewIstioProbe(g)
		default:
			logging.GetLogger().Errorf("unknown probe type: %s", t)
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create probe %s: %w", t, err)
		}
		if handler != nil {
			bundle.AddHandler(t, handler)
		}
	}

	return bundle, nil
}
