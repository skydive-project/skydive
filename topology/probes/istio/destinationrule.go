/*
 * Copyright (C) 2018 IBM, Inc.
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
	"fmt"

	kiali "github.com/hunchback/kiali/kubernetes"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/k8s"

	"k8s.io/client-go/tools/cache"
)

type destinationRulesProbe struct {
	k8s.DefaultKubeCacheEventHandler
	*k8s.KubeCache
	graph *graph.Graph
}

func dumpDestinationRule(dr *kiali.DestinationRule) string {
	return fmt.Sprintf("destinationrule{Namespace: %s, Name: %s}", dr.Namespace, dr.Name)
}

func (p *destinationRulesProbe) newMetadata(dr *kiali.DestinationRule) graph.Metadata {
	return newMetadata("destinationrule", dr.Namespace, dr.Name, dr)
}

func destinationRulesUID(dr *kiali.DestinationRule) graph.Identifier {
	return graph.Identifier(dr.GetUID())
}

func (p *destinationRulesProbe) OnAdd(obj interface{}) {
	if dr, ok := obj.(*kiali.DestinationRule); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		k8s.NewNode(p.graph, destinationRulesUID(dr), p.newMetadata(dr))
		logging.GetLogger().Debugf("Added %s", dumpDestinationRule(dr))
	}
}

func (p *destinationRulesProbe) OnUpdate(oldObj, newObj interface{}) {
	if dr, ok := newObj.(*kiali.DestinationRule); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if drNode := p.graph.GetNode(destinationRulesUID(dr)); drNode != nil {
			k8s.AddMetadata(p.graph, drNode, dr)
			logging.GetLogger().Debugf("Updated %s", dumpDestinationRule(dr))
		}
	}
}

func (p *destinationRulesProbe) OnDelete(obj interface{}) {
	if dr, ok := obj.(*kiali.DestinationRule); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if drNode := p.graph.GetNode(destinationRulesUID(dr)); drNode != nil {
			p.graph.DelNode(drNode)
			logging.GetLogger().Debugf("Deleted %s", dumpDestinationRule(dr))
		}
	}
}

func (p *destinationRulesProbe) Start() {
	p.KubeCache.Start()
}

func (p *destinationRulesProbe) Stop() {
	p.KubeCache.Stop()
}

func newDestinationRuleKubeCache(handler cache.ResourceEventHandler) *k8s.KubeCache {
	return k8s.NewKubeCache(getClient().GetIstioNetworkingApi(), &kiali.DestinationRule{}, "destinationrules", handler)
}

func newDestinationRuleProbe(g *graph.Graph) probe.Probe {
	p := &destinationRulesProbe{
		graph: g,
	}
	p.KubeCache = newDestinationRuleKubeCache(p)
	return p
}
