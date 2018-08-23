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

package k8s

import (
	"fmt"

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

type deployProbe struct {
	DefaultKubeCacheEventHandler
	*KubeCache
	graph *graph.Graph
}

func dumpDeployment(deploy *v1beta1.Deployment) string {
	return fmt.Sprintf("deployment{Namespace: %s, Name: %s}", deploy.Namespace, deploy.Name)
}

func (p *deployProbe) newMetadata(deploy *v1beta1.Deployment) graph.Metadata {
	m := NewMetadata(Manager, "deployment", deploy.Namespace, deploy.Name, deploy)
	m.SetFieldAndNormalize("Selector", deploy.Spec.Selector)
	m.SetField("DesiredReplicas", int32ValueOrDefault(deploy.Spec.Replicas, 1))
	m.SetField("Replicas", deploy.Status.Replicas)
	m.SetField("ReadyReplicas", deploy.Status.ReadyReplicas)
	m.SetField("AvailableReplicas", deploy.Status.AvailableReplicas)
	m.SetField("UnavailableReplicas", deploy.Status.UnavailableReplicas)
	return m
}

func deployUID(deploy *v1beta1.Deployment) graph.Identifier {
	return graph.Identifier(deploy.GetUID())
}

func (p *deployProbe) OnAdd(obj interface{}) {
	if deploy, ok := obj.(*v1beta1.Deployment); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		NewNode(p.graph, deployUID(deploy), p.newMetadata(deploy))
		logging.GetLogger().Debugf("Added %s", dumpDeployment(deploy))
	}
}

func (p *deployProbe) OnUpdate(oldObj, newObj interface{}) {
	if deploy, ok := newObj.(*v1beta1.Deployment); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if deployNode := p.graph.GetNode(deployUID(deploy)); deployNode != nil {
			AddMetadata(p.graph, deployNode, deploy)
			logging.GetLogger().Debugf("Updated %s", dumpDeployment(deploy))
		}
	}
}

func (p *deployProbe) OnDelete(obj interface{}) {
	if deploy, ok := obj.(*v1beta1.Deployment); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if deployNode := p.graph.GetNode(deployUID(deploy)); deployNode != nil {
			p.graph.DelNode(deployNode)
			logging.GetLogger().Debugf("Deleted %s", dumpDeployment(deploy))
		}
	}
}

func (p *deployProbe) Start() {
	p.KubeCache.Start()
}

func (p *deployProbe) Stop() {
	p.KubeCache.Stop()
}

func newDeploymentKubeCache(handler cache.ResourceEventHandler) *KubeCache {
	return NewKubeCache(getClientset().ExtensionsV1beta1().RESTClient(), &v1beta1.Deployment{}, "deployments", handler)
}

func newDeploymentProbe(g *graph.Graph) probe.Probe {
	p := &deployProbe{
		graph: g,
	}
	p.KubeCache = newDeploymentKubeCache(p)
	return p
}
