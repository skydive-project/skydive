//go:generate go run ../../scripts/gendecoder.go -package github.com/skydive-project/skydive/flow/probes

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
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/ondemand"
	"github.com/skydive-project/skydive/probe"
)

// ErrProbeNotCompiled is thrown when a flow probe was not compiled within the binary
var ErrProbeNotCompiled = fmt.Errorf("probe is not compiled within skydive")

// Probe defines an active flow probe
type Probe = ondemand.Task

// FlowProbeHandler defines flow probe mechanism
type FlowProbeHandler interface {
	probe.Handler // inheritance of the probe.Handler interface Start/Stop functions
	RegisterProbe(n *graph.Node, capture *types.Capture, e ProbeEventHandler) (Probe, error)
	UnregisterProbe(n *graph.Node, e ProbeEventHandler, p Probe) error
	CaptureTypes() []string
	Init(ctx Context, bundle *probe.Bundle) (FlowProbeHandler, error)
}

// Context defines a context to be used by constructor of probes
type Context struct {
	Logger logging.Logger
	Config config.Config
	Graph  *graph.Graph
	FTA    *flow.TableAllocator
	TB     *probe.Bundle
}

// ProbeEventHandler used by probes to notify state
type ProbeEventHandler interface {
	OnStarted(*CaptureMetadata)
	OnStopped()
	OnError(err error)
}

// NewFlowProbeBundle returns a new bundle of flow probes
func NewFlowProbeBundle(tb *probe.Bundle, g *graph.Graph, fta *flow.TableAllocator) *probe.Bundle {
	list := []string{"pcapsocket", "ovssflow", "sflow", "gopacket", "dpdk", "ebpf", "ovsmirror", "ovsnetflow"}
	logging.GetLogger().Infof("Flow probes: %v", list)

	var handler FlowProbeHandler
	var err error

	bundle := probe.NewBundle()
	ctx := Context{
		Logger: logging.GetLogger(),
		Config: config.GetConfig(),
		Graph:  g,
		FTA:    fta,
		TB:     tb,
	}

	for _, t := range list {
		if bundle.GetHandler(t) != nil {
			continue
		}

		switch t {
		case "pcapsocket":
			handler, err = new(PcapSocketProbeHandler).Init(ctx, bundle)
		case "ovssflow":
			handler, err = NewOvsSFlowProbesHandler(g, fta, tb)
		case "ovsmirror":
			handler, err = NewOvsMirrorProbesHandler(g, tb, bundle)
		case "gopacket":
			handler, err = NewGoPacketProbesHandler(g, fta)
		case "sflow":
			handler, err = NewSFlowProbesHandler(g, fta)
		case "ovsnetflow":
			handler, err = NewOvsNetFlowProbesHandler(g, fta, tb)
		case "dpdk":
			handler, err = NewDPDKProbesHandler(g, fta)
		case "ebpf":
			handler, err = NewEBPFProbesHandler(g, fta)
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

		for _, captureType := range handler.CaptureTypes() {
			bundle.AddHandler(captureType, handler)
		}
	}

	return bundle
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

// Captures holds the captures metadata
// easyjson:json
// gendecoder
type Captures []*CaptureMetadata

// CaptureMetadata holds attributes and statistics about a capture
// easyjson:json
// gendecoder
type CaptureMetadata struct {
	CaptureStats
	ID          string `json:",omitempty"`
	State       string `json:",omitempty"`
	Name        string `json:",omitempty"`
	Description string `json:",omitempty"`
	BPFFilter   string `json:",omitempty"`
	Type        string `json:",omitempty"`
	PCAPSocket  string `json:",omitempty"`
	MirrorOf    string `json:",omitempty"`
	SFlowSocket string `json:",omitempty"`
	Error       string `json:",omitempty"`
}

// CaptureStats describes the statistics of a running capture
// easyjson:json
// gendecoder
type CaptureStats struct {
	PacketsReceived  int64
	PacketsDropped   int64
	PacketsIfDropped int64
}

// CapturesMetadataDecoder implements a json message raw decoder
func CapturesMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var captures Captures
	if err := json.Unmarshal(raw, &captures); err != nil {
		return nil, fmt.Errorf("unable to unmarshal captures metadata %s: %s", string(raw), err)
	}

	return &captures, nil
}
