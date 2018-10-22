/*
 * Copyright 2018 IBM Corp.
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

package istio

import (
	kiali "github.com/kiali/kiali/kubernetes"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/k8s"
)

// Probe describes the Istio probe in charge of importing
// Istio resources into the graph
type Probe struct {
	*k8s.Probe
}

// NewIstioProbe creates the probe for tracking istio events
func NewIstioProbe(g *graph.Graph) (*k8s.Probe, error) {
	configFile := config.GetString("analyzer.topology.istio.config_file")
	config, err := k8s.NewConfig(configFile)
	if err != nil {
		return nil, err
	}

	client, err := kiali.NewClientFromConfig(config)
	if err != nil {
		return nil, err
	}

	subprobes := map[string]k8s.Subprobe{
		"destinationrule":  newDestinationRuleProbe(client, g),
		"gateway":          newGatewayProbe(client, g),
		"quotaspec":        newQuotaSpecProbe(client, g),
		"quotaspecbinding": newQuotaSpecBindingProbe(client, g),
		"serviceentry":     newServiceEntryProbe(client, g),
		"virtualservice":   newVirtualServiceProbe(client, g),
	}

	probe := k8s.NewProbe(g, Manager, subprobes, nil)

	probe.AppendNamespaceLinkers(
		"destinationrule",
		"gateway",
		"quotaspec",
		"quotaspecbinding",
		"serviceentry",
		"virtualservice",
	)

	return probe, nil
}
