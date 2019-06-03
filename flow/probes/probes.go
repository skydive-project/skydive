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

package probes

import (
	"fmt"

	"github.com/skydive-project/skydive/analyzer"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
)

// ErrProbeNotCompiled is thrown when a flow probe was not compiled within the binary
var ErrProbeNotCompiled = fmt.Errorf("probe is not compiled within skydive")

// FlowProbe defines flow probe mechanism
type FlowProbe interface {
	probe.Probe // inheritance of the probe.Probe interface Start/Stop functions
	RegisterProbe(n *graph.Node, capture *types.Capture, e FlowProbeEventHandler) error
	UnregisterProbe(n *graph.Node, e FlowProbeEventHandler) error
}

// FlowProbeEventHandler used by probes to notify capture state
type FlowProbeEventHandler interface {
	OnStarted()
	OnStopped()
	OnError(err error)
}

// FlowProbeTableAllocator allocates table and set the table update callback
type FlowProbeTableAllocator struct {
	*flow.TableAllocator
	fcpool *analyzer.FlowClientPool
}

// Alloc override the default implementation provide a default update function
func (a *FlowProbeTableAllocator) Alloc(nodeTID string, opts flow.TableOpts) *flow.Table {
	return a.TableAllocator.Alloc(a.fcpool.SendFlows, nodeTID, opts)
}

// NewFlowProbeBundle returns a new bundle of flow probes
func NewFlowProbeBundle(tb *probe.Bundle, g *graph.Graph, fta *flow.TableAllocator, fcpool *analyzer.FlowClientPool) *probe.Bundle {
	list := []string{"pcapsocket", "ovssflow", "sflow", "gopacket", "dpdk", "ebpf", "ovsmirror", "ovsnetflow"}
	logging.GetLogger().Infof("Flow probes: %v", list)

	var captureTypes []string
	var fp FlowProbe
	var err error

	fpta := &FlowProbeTableAllocator{
		TableAllocator: fta,
		fcpool:         fcpool,
	}

	fb := probe.NewBundle(make(map[string]probe.Probe))

	for _, t := range list {
		if fb.GetProbe(t) != nil {
			continue
		}

		switch t {
		case "pcapsocket":
			fp, err = NewPcapSocketProbeHandler(g, fpta)
			captureTypes = []string{"pcapsocket"}
		case "ovssflow":
			fp, err = NewOvsSFlowProbesHandler(g, fpta, tb)
			captureTypes = []string{"ovssflow"}
		case "ovsmirror":
			fp, err = NewOvsMirrorProbesHandler(g, tb, fb)
			captureTypes = []string{"ovsmirror"}
		case "gopacket":
			fp, err = NewGoPacketProbesHandler(g, fpta)
			captureTypes = []string{"afpacket", "pcap"}
		case "sflow":
			fp, err = NewSFlowProbesHandler(g, fpta)
			captureTypes = []string{"sflow"}
		case "ovsnetflow":
			fp, err = NewOvsNetFlowProbesHandler(g, fpta, tb)
			captureTypes = []string{"ovsnetflow"}
		case "dpdk":
			if fp, err = NewDPDKProbesHandler(g, fpta); err == nil {
				captureTypes = []string{"dpdk"}
			}
		case "ebpf":
			if fp, err = NewEBPFProbesHandler(g, fpta); err == nil {
				captureTypes = []string{"ebpf"}
			}
		default:
			err = fmt.Errorf("unknown probe type %s", t)
		}

		if err != nil {
			if err != ErrProbeNotCompiled {
				logging.GetLogger().Errorf("Failed to create %s probe: %s", t, err)
			} else {
				logging.GetLogger().Infof("Not compiled with %s support, skipping it", t)
			}
			continue
		}

		for _, captureType := range captureTypes {
			fb.AddProbe(captureType, fp)
		}
	}

	return fb
}

func tableOptsFromCapture(capture *types.Capture) flow.TableOpts {
	layerKeyMode, _ := flow.LayerKeyModeByName(capture.LayerKeyMode)

	return flow.TableOpts{
		RawPacketLimit: int64(capture.RawPacketLimit),
		ExtraTCPMetric: capture.ExtraTCPMetric,
		IPDefrag:       capture.IPDefrag,
		ReassembleTCP:  capture.ReassembleTCP,
		LayerKeyMode:   layerKeyMode,
		ExtraLayers:    capture.ExtraLayers,
	}
}
