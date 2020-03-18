//go:generate go run github.com/skydive-project/skydive/scripts/gendecoder -package github.com/skydive-project/skydive/flow/probes
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

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

// TableOptsFromCapture is a helper that returns the flow options for a capture
func TableOptsFromCapture(capture *types.Capture) flow.TableOpts {
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
