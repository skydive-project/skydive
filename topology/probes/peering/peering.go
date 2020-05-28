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

package peering

import (
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology"
)

// Probe describes graph peering based on MAC address and graph events
type Probe struct {
	graph.DefaultGraphListener
	graph              *graph.Graph
	peerIntfMACIndexer *graph.MetadataIndexer
	macIndexer         *graph.MetadataIndexer
	linker             *graph.MetadataIndexerLinker
}

// Start the MAC peering resolver probe
func (p *Probe) Start() error {
	p.peerIntfMACIndexer.Start()
	p.macIndexer.Start()
	p.linker.Start()
	return nil
}

// Stop the probe
func (p *Probe) Stop() {
	p.peerIntfMACIndexer.Stop()
	p.macIndexer.Stop()
	p.linker.Stop()
}

// OnError implements the LinkerEventListener interface
func (p *Probe) OnError(err error) {
	logging.GetLogger().Error(err)
}

// NewProbe creates a new graph node peering probe
func NewProbe(g *graph.Graph) *Probe {
	peerIntfMACIndexer := graph.NewMetadataIndexer(g, g, nil, "PeerIntfMAC")
	macIndexer := graph.NewMetadataIndexer(g, g, nil, "MAC")

	linker := graph.NewMetadataIndexerLinker(g, peerIntfMACIndexer, macIndexer, graph.Metadata{"RelationType": topology.Layer2Link})

	probe := &Probe{
		graph:              g,
		peerIntfMACIndexer: peerIntfMACIndexer,
		macIndexer:         macIndexer,
		linker:             linker,
	}
	linker.AddEventListener(probe)

	return probe
}
