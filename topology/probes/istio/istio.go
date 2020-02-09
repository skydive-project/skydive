/*
 * Copyright 2018 IBM Corp.
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

package istio

import (
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	client "istio.io/client-go/pkg/clientset/versioned"
)

// Probe describes the Istio probe in charge of importing
// Istio resources into the graph
type Probe struct {
	*k8s.Probe
}

// NewIstioProbe creates the probe for tracking istio events
func NewIstioProbe(g *graph.Graph) (*k8s.Probe, error) {
	kubeconfigPath := config.GetString("analyzer.topology.istio.config_file")
	enabledSubprobes := config.GetStringSlice("analyzer.topology.istio.probes")
	config, _, err := k8s.NewConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	client, err := client.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	subprobeHandlers := map[string]k8s.SubprobeHandler{
		"destinationrule":  newDestinationRuleProbe,
		"gateway":          newGatewayProbe,
		"quotaspec":        newQuotaSpecProbe,
		"quotaspecbinding": newQuotaSpecBindingProbe,
		"serviceentry":     newServiceEntryProbe,
		"virtualservice":   newVirtualServiceProbe,
	}

	k8s.InitSubprobes(enabledSubprobes, subprobeHandlers, client, g, Manager, "")

	verifierHandlers := []verifierHandler{
		newVirtualServiceGatewayVerifier,
	}

	verifiers := initResourceVerifiers(verifierHandlers, g)

	linkerHandlers := []k8s.LinkHandler{
		newVirtualServicePodLinker,
		newDestinationRuleServiceLinker,
		newDestinationRuleServiceEntryLinker,
		newGatewayVirtualServiceLinker,
	}

	linkers := k8s.InitLinkers(linkerHandlers, g)

	probe := k8s.NewProbe(g, Manager, k8s.GetSubprobesMap(Manager), linkers, verifiers)

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
