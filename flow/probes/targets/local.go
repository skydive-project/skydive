/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package targets

import (
	"github.com/google/gopacket"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// LocalTarget send packet to an agent flow table
type LocalTarget struct {
	table     *flow.Table
	fta       *flow.TableAllocator
	statsChan chan flow.Stats
}

// SendPacket implements the Target interface
func (l *LocalTarget) SendPacket(packet gopacket.Packet, bpf *flow.BPF) {
	l.table.FeedWithGoPacket(packet, bpf)
}

// SendStats implements the flow Sender interface
func (l *LocalTarget) SendStats(stats flow.Stats) {
	l.statsChan <- stats
}

// Start target
func (l *LocalTarget) Start() {
	_, _, l.statsChan = l.table.Start(nil)
}

// Stop target
func (l *LocalTarget) Stop() {
	l.table.Stop()
	l.fta.Release(l.table)
}

// NewLocalTarget returns a new local target
func NewLocalTarget(g *graph.Graph, n *graph.Node, capture *types.Capture, uuids flow.UUIDs, fta *flow.TableAllocator) (*LocalTarget, error) {
	table := fta.Alloc(uuids, tableOptsFromCapture(capture))

	return &LocalTarget{
		table: table,
		fta:   fta,
	}, nil
}
