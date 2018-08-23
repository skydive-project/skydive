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

	"k8s.io/api/batch/v1beta1"
	"k8s.io/client-go/tools/cache"
)

type cronJobProbe struct {
	DefaultKubeCacheEventHandler
	*KubeCache
	graph *graph.Graph
}

func dumpCronJob(cj *v1beta1.CronJob) string {
	return fmt.Sprintf("cronjob{Namespace: %s, Name: %s}", cj.Namespace, cj.Name)
}

func (p *cronJobProbe) newMetadata(cj *v1beta1.CronJob) graph.Metadata {
	m := NewMetadata(Manager, "cronjob", cj.Namespace, cj.Name, cj)
	m.SetField("Schedule", cj.Spec.Schedule)
	m.SetField("Suspended", cj.Spec.Suspend != nil && *cj.Spec.Suspend)
	return m
}

func cronJobUID(cj *v1beta1.CronJob) graph.Identifier {
	return graph.Identifier(cj.GetUID())
}

func (p *cronJobProbe) OnAdd(obj interface{}) {
	if cj, ok := obj.(*v1beta1.CronJob); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		NewNode(p.graph, cronJobUID(cj), p.newMetadata(cj))
		logging.GetLogger().Debugf("Added %s", dumpCronJob(cj))
	}
}

func (p *cronJobProbe) OnUpdate(oldObj, newObj interface{}) {
	if cj, ok := newObj.(*v1beta1.CronJob); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(cronJobUID(cj)); node != nil {
			AddMetadata(p.graph, node, cj)
			logging.GetLogger().Debugf("Updated %s", dumpCronJob(cj))
		}
	}
}

func (p *cronJobProbe) OnDelete(obj interface{}) {
	if cj, ok := obj.(*v1beta1.CronJob); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(cronJobUID(cj)); node != nil {
			p.graph.DelNode(node)
			logging.GetLogger().Debugf("Deleted %s", dumpCronJob(cj))
		}
	}
}

func (p *cronJobProbe) Start() {
	p.KubeCache.Start()
}

func (p *cronJobProbe) Stop() {
	p.KubeCache.Stop()
}

func newCronJobKubeCache(handler cache.ResourceEventHandler) *KubeCache {
	return NewKubeCache(getClientset().BatchV1beta1().RESTClient(), &v1beta1.CronJob{}, "cronjobs", handler)
}

func newCronJobProbe(g *graph.Graph) probe.Probe {
	p := &cronJobProbe{
		graph: g,
	}
	p.KubeCache = newCronJobKubeCache(p)
	return p
}
