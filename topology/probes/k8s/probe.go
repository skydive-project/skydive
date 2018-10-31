/*
 * Copyright 2017 IBM Corp.
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
	"sync"
	"time"

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"

	"k8s.io/apimachinery/pkg/util/runtime"
)

func int32ValueOrDefault(value *int32, defaultValue int32) int32 {
	if value == nil {
		return defaultValue
	}
	return *value
}

// Probe for tracking k8s events
type Probe struct {
	graph     *graph.Graph
	manager   string
	subprobes map[string]Subprobe
	linkers   []probe.Probe
}

// Subprobe describes a probe for a specific Kubernetes resource
// It must implement the ListenerHandler interface so that you
// listen for creation/update/removal of a resource
type Subprobe interface {
	probe.Probe
	graph.ListenerHandler
}

// Start k8s probe
func (p *Probe) Start() {
	for _, linker := range p.linkers {
		linker.Start()
	}

	for _, subprobe := range p.subprobes {
		subprobe.Start()
	}
}

// Stop k8s probe
func (p *Probe) Stop() {
	for _, linker := range p.linkers {
		linker.Stop()
	}

	for _, subprobe := range p.subprobes {
		subprobe.Stop()
	}
}

// AppendClusterLinkers appends newly created cluster linker per type
func (p *Probe) AppendClusterLinkers(types ...string) {
	if clusterLinker := newClusterLinker(p.graph, p.manager, types...); clusterLinker != nil {
		p.linkers = append(p.linkers, clusterLinker)
	}
}

// AppendNamespaceLinkers appends newly created namespace linker per type
func (p *Probe) AppendNamespaceLinkers(types ...string) {
	if namespaceLinker := newNamespaceLinker(p.graph, p.manager, types...); namespaceLinker != nil {
		p.linkers = append(p.linkers, namespaceLinker)
	}
}

// NewProbe creates the probe for tracking k8s events
func NewProbe(g *graph.Graph, manager string, subprobes map[string]Subprobe, linkers []probe.Probe) *Probe {
	return &Probe{
		graph:     g,
		manager:   manager,
		subprobes: subprobes,
		linkers:   linkers,
	}
}

func logOnError(err error) {
	logging.GetLogger().Warning(err)
}

type errorThrottle struct {
	period   time.Duration
	lastLock sync.RWMutex
	last     time.Time
}

func (r *errorThrottle) onError(error) {
	r.lastLock.RLock()
	d := time.Since(r.last)
	r.lastLock.RUnlock()

	if d < r.period {
		time.Sleep(r.period - d)
	}

	r.lastLock.Lock()
	r.last = time.Now()
	r.lastLock.Unlock()
}

func muteInternalErrors() {
	throttle := errorThrottle{
		period: time.Second,
		last:   time.Now(),
	}
	runtime.ErrorHandlers = []func(error){
		logOnError,
		throttle.onError,
	}
}

func init() {
	muteInternalErrors()
}
