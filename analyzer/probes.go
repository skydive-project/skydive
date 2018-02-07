/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package analyzer

import (
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/fabric"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	"github.com/skydive-project/skydive/topology/probes/peering"
)

// NewTopologyProbeBundleFromConfig creates a new topology server probes from configuration
func NewTopologyProbeBundleFromConfig(g *graph.Graph) (*probe.ProbeBundle, error) {
	list := config.GetStringSlice("analyzer.topology.probes")
	probes := map[string]probe.Probe{
		"fabric":  fabric.NewFabricProbe(g),
		"peering": peering.NewPeeringProbe(g),
	}

	for _, t := range list {
		if _, ok := probes[t]; ok {
			continue
		}

		switch t {
		case "k8s":
			var err error
			probes[t], err = k8s.NewProbe(g)
			if err != nil {
				logging.GetLogger().Errorf("Failed to initialize K8S probe: %s", err.Error())
				return nil, err
			}

		default:
			logging.GetLogger().Errorf("unknown probe type: %s", t)
		}
	}

	return probe.NewProbeBundle(probes), nil
}
