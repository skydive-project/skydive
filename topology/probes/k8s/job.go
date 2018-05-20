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

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/tools/cache"
)

type jobProbe struct {
	defaultKubeCacheEventHandler
	*kubeCache
	graph *graph.Graph
}

func dumpJob(job *batchv1.Job) string {
	return fmt.Sprintf("job{Name: %s}", job.GetName())
}

func (p *jobProbe) newMetadata(job *batchv1.Job) graph.Metadata {
	return newMetadata("job", job.Namespace, job.GetName(), job)
}

func jobUID(job *batchv1.Job) graph.Identifier {
	return graph.Identifier(job.GetUID())
}

func (p *jobProbe) OnAdd(obj interface{}) {
	if job, ok := obj.(*batchv1.Job); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		newNode(p.graph, jobUID(job), p.newMetadata(job))
		logging.GetLogger().Debugf("Added %s", dumpJob(job))
	}
}

func (p *jobProbe) OnUpdate(oldObj, newObj interface{}) {
	if job, ok := newObj.(*batchv1.Job); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(jobUID(job)); node != nil {
			addMetadata(p.graph, node, job)
			logging.GetLogger().Debugf("Updated %s", dumpJob(job))
		}
	}
}

func (p *jobProbe) OnDelete(obj interface{}) {
	if job, ok := obj.(*batchv1.Job); ok {
		p.graph.Lock()
		defer p.graph.Unlock()

		if node := p.graph.GetNode(jobUID(job)); node != nil {
			p.graph.DelNode(node)
			logging.GetLogger().Debugf("Deleted %s", dumpJob(job))
		}
	}
}

func (p *jobProbe) Start() {
	p.kubeCache.Start()
}

func (p *jobProbe) Stop() {
	p.kubeCache.Stop()
}

func newJobKubeCache(handler cache.ResourceEventHandler) *kubeCache {
	return newKubeCache(getClientset().BatchV1().RESTClient(), &batchv1.Job{}, "jobs", handler)
}

func newJobProbe(g *graph.Graph) probe.Probe {
	p := &jobProbe{
		graph: g,
	}
	p.kubeCache = newJobKubeCache(p)
	return p
}
