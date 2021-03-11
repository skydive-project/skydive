/*
 * Copyright (C) 2019 Red Hat, Inc.
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
package ovnk8s

import (
	"fmt"
	"strings"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
)

// Probe describes an OVN-kubernetes probe
type Probe struct {
	graph.DefaultGraphListener
	graph         *graph.Graph
	bundle        *probe.Bundle
	podIndexer    *graph.MetadataIndexer
	lspPodIndexer *graph.MetadataIndexer
	ovnPodLinker  *graph.ResourceLinker
}

// Start starts the probe. Part of Probe interface
func (p *Probe) Start() error {
	return p.bundle.Start()
}

// Stop stops the probe. Part of Probe interface
func (p *Probe) Stop() {
	p.bundle.Stop()
}

// OnError handles linker errors. Part of LinkerEventListener interface
func (p *Probe) OnError(err error) {
	logging.GetLogger().Error("Linker error: %s", err.Error())
}

// ovnPodLinker implements ResourceLinker and creates links between pods and
// logical_switch_ports
type ovnPodLinker struct {
	probe *Probe
}

// GetABLinks returns the links from a Pod to it's logical switch port
func (l *ovnPodLinker) GetABLinks(podNode *graph.Node) (edges []*graph.Edge) {
	name, _ := podNode.GetFieldString("Name")
	namespace, _ := podNode.GetFieldString("K8s.Namespace")
	combined := fmt.Sprintf("%s_%s", namespace, name)
	lsPortNode, _ := l.probe.lspPodIndexer.GetNode(combined)
	if lsPortNode != nil {
		link, err := topology.NewLink(l.probe.graph, podNode, lsPortNode, topology.Layer2Link, nil)
		if err != nil {
			logging.GetLogger().Error(link)
		}
		logging.GetLogger().Debugf("Linking Pod (%s/%s) with Logical Port (%s)",
			namespace, name, combined)
		edges = append(edges, link)
	}
	return edges
}

// GetBALinks returns the links from a LogicalSwitchPort to its Pod
func (l *ovnPodLinker) GetBALinks(lsPortNode *graph.Node) (edges []*graph.Edge) {
	name, _ := lsPortNode.GetFieldString("Name")
	split := strings.Split(name, "_")
	if len(split) < 2 {
		return edges
	}
	podNode, _ := l.probe.podIndexer.GetNode(split[1], split[0])
	if podNode != nil {
		link, err := topology.NewLink(l.probe.graph, podNode, lsPortNode, topology.Layer2Link, nil)
		if err != nil {
			logging.GetLogger().Error(link)
		}
		logging.GetLogger().Debugf("Linking Pod (%s/%s) with Logical Port (%s)",
			split[0], split[1], name)
		edges = append(edges, link)
	}
	return edges
}

// NewProbe creates a new graph OVN-kubernetes probe
func NewProbe(g *graph.Graph) (probe.Handler, error) {
	p := &Probe{
		graph:  g,
		bundle: probe.NewBundle(),
	}

	p.podIndexer = graph.NewMetadataIndexer(g, g, graph.Metadata{"Type": "pod"}, "Name", "K8s.Namespace")
	p.bundle.AddHandler("podIndexer", p.podIndexer)

	p.lspPodIndexer = graph.NewMetadataIndexer(g, g, graph.Metadata{"Type": "logical_switch_port", "OVN.ExtID.pod": "true"}, "Name")
	p.bundle.AddHandler("lspIndexer", p.lspPodIndexer)

	p.ovnPodLinker = graph.NewResourceLinker(g,
		[]graph.ListenerHandler{p.podIndexer},
		[]graph.ListenerHandler{p.lspPodIndexer},
		&ovnPodLinker{probe: p}, graph.Metadata{"Manager": "ovnk8s"})
	p.bundle.AddHandler("ovnPodLinker", p.ovnPodLinker)
	// Add Probe as event listener to report errors
	p.ovnPodLinker.AddEventListener(p)

	logging.GetLogger().Info("OVN-kubenetes probe created")

	return p, nil
}
