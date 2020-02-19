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
	"errors"

	"github.com/google/gopacket"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
)

var (
	// ErrTargetTypeUnknown target type unknown
	ErrTargetTypeUnknown = errors.New("target type unknown")
)

// Target interface
type Target interface {
	SendPacket(packet gopacket.Packet, bpf *flow.BPF)
	SendStats(stats flow.Stats)
	Start()
	Stop()
}

func tableOptsFromCapture(capture *types.Capture) flow.TableOpts {
	layerKeyMode, _ := flow.LayerKeyModeByName(capture.LayerKeyMode)

	return flow.TableOpts{
		RawPacketLimit: int64(capture.RawPacketLimit),
		ExtraTCPMetric: capture.ExtraTCPMetric,
		IPDefrag:       capture.IPDefrag,
		ReassembleTCP:  capture.ReassembleTCP,
		LayerKeyMode:   layerKeyMode,
		ExtraLayers:    flow.ExtraLayers(capture.ExtraLayers),
	}
}

// NewTarget returns target according to the given type
func NewTarget(typ string, g *graph.Graph, n *graph.Node, capture *types.Capture, uuids flow.UUIDs, bpf *flow.BPF, fta *flow.TableAllocator) (Target, error) {
	switch typ {
	case "netflowv5":
		return NewNetFlowV5Target(g, n, capture, uuids)
	case "erspanv1":
		return NewERSpanTarget(g, n, capture)
	case "", "local":
		return NewLocalTarget(g, n, capture, uuids, fta)
	}

	return nil, ErrTargetTypeUnknown
}
