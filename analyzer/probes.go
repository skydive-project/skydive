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

package analyzer

import (
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/probes/fabric"
	"github.com/skydive-project/skydive/topology/probes/istio"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	"github.com/skydive-project/skydive/topology/probes/peering"
)

// NewTopologyProbeBundleFromConfig creates a new topology server probes from configuration
func NewTopologyProbeBundleFromConfig(g *graph.Graph) (*probe.Bundle, error) {
	list := config.GetStringSlice("analyzer.topology.probes")

	fabricProbe, err := fabric.NewProbe(g)
	if err != nil {
		return nil, err
	}

	probes := map[string]probe.Probe{
		"fabric":  fabricProbe,
		"peering": peering.NewProbe(g),
	}

	for _, t := range list {
		if _, ok := probes[t]; ok {
			continue
		}

		var err error

		switch t {
		case "k8s":
			probes[t], err = k8s.NewK8sProbe(g)
		case "istio":
			probes[t], err = istio.NewIstioProbe(g)
		default:
			logging.GetLogger().Errorf("unknown probe type: %s", t)
			continue
		}

		if err != nil {
			logging.GetLogger().Errorf("failed to initialize probe %s: %s", t, err)
			return nil, err
		}
	}

	return probe.NewBundle(probes), nil
}
