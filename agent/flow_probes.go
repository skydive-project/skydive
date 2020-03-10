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

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	fp "github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/flow/probes/dpdk"
	"github.com/skydive-project/skydive/flow/probes/ebpf"
	"github.com/skydive-project/skydive/flow/probes/gopacket"
	"github.com/skydive-project/skydive/flow/probes/ovsmirror"
	"github.com/skydive-project/skydive/flow/probes/ovsnetflow"
	"github.com/skydive-project/skydive/flow/probes/ovssflow"
	"github.com/skydive-project/skydive/flow/probes/pcapsocket"
	"github.com/skydive-project/skydive/flow/probes/sflow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/probe"
)

// NewFlowProbeBundle returns a new bundle of flow probes
func NewFlowProbeBundle(tb *probe.Bundle, g *graph.Graph, fta *flow.TableAllocator) *probe.Bundle {
	list := []string{"pcapsocket", "ovssflow", "sflow", "gopacket", "dpdk", "ebpf", "ovsmirror", "ovsnetflow"}
	logging.GetLogger().Infof("Flow probes: %v", list)

	var handler fp.FlowProbeHandler
	var err error

	bundle := probe.NewBundle()
	ctx := fp.Context{
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
			handler, err = pcapsocket.NewProbe(ctx, bundle)
		case "ovssflow":
			handler, err = ovssflow.NewProbe(ctx, bundle)
		case "ovsmirror":
			handler, err = ovsmirror.NewProbe(ctx, bundle)
		case "gopacket":
			handler, err = gopacket.NewProbe(ctx, bundle)
		case "sflow":
			handler, err = sflow.NewProbe(ctx, bundle)
		case "ovsnetflow":
			handler, err = ovsnetflow.NewProbe(ctx, bundle)
		case "dpdk":
			handler, err = dpdk.NewProbe(ctx, bundle)
		case "ebpf":
			handler, err = ebpf.NewProbe(ctx, bundle)
		default:
			err = fmt.Errorf("unknown probe type %s", t)
		}

		if err != nil {
			if err != probe.ErrNotCompiled {
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
