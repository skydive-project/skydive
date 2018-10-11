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
	"github.com/skydive-project/skydive/topology/probes/istio"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	"github.com/skydive-project/skydive/topology/probes/peering"
)

// NewTopologyProbeBundleFromConfig creates a new topology server probes from configuration
func NewTopologyProbeBundleFromConfig(g *graph.Graph) (*probe.Bundle, error) {
	list := config.GetStringSlice("analyzer.topology.probes")
	probes := map[string]probe.Probe{
		"fabric":  fabric.NewProbe(g),
		"peering": peering.NewProbe(g),
	}

	for _, t := range list {
		if _, ok := probes[t]; ok {
			continue
		}

		type newProbeFuncType func(*graph.Graph) (*k8s.Probe, error)
		newProbeFuncMap := map[string]newProbeFuncType{
			"k8s":   k8s.NewK8sProbe,
			"istio": istio.NewIstioProbe,
		}

		if newProbeFunc, ok := newProbeFuncMap[t]; !ok {
			logging.GetLogger().Errorf("unknown probe type: %s", t)
		} else {
			var err error
			probes[t], err = newProbeFunc(g)
			if err != nil {
				logging.GetLogger().Errorf("failed to initialize probe %s: %s", t, err)
				return nil, err
			}
		}
	}

	return probe.NewBundle(probes), nil
}
