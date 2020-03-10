/*
 * Copyright 2017 IBM Corp.
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

package k8s

import (
	"sync"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/probe"

	"k8s.io/apimachinery/pkg/util/runtime"
)

// LinkHandler creates a linker
type LinkHandler func(g *graph.Graph) probe.Handler

// InitLinkers initializes the listed linkers
func InitLinkers(linkerHandlers []LinkHandler, g *graph.Graph) (linkers []probe.Handler) {
	for _, handler := range linkerHandlers {
		if linker := handler(g); linker != nil {
			linkers = append(linkers, linker)
		}
	}
	return
}

var subprobes = make(map[string]map[string]Subprobe)

// PutSubprobe puts a new subprobe in the subprobes map
func PutSubprobe(manager, name string, subprobe Subprobe) {
	subprobes[manager][name] = subprobe
}

// GetSubprobe returns a specific subprobe
func GetSubprobe(manager, name string) Subprobe {
	return subprobes[manager][name]
}

// GetSubprobesMap returns a map of all the subprobes that belong to manager probe
func GetSubprobesMap(manager string) map[string]Subprobe {
	return subprobes[manager]
}

// ListSubprobes returns the list of Subprobe as ListernerHandler
func ListSubprobes(manager string, types ...string) (handlers []graph.ListenerHandler) {
	for _, t := range types {
		if subprobe := GetSubprobe(Manager, t); subprobe != nil {
			handlers = append(handlers, subprobe)
		}
	}
	return
}

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
	linkers   []probe.Handler
	verifiers []probe.Handler
}

// Subprobe describes a probe for a specific Kubernetes resource
// It must implement the ListenerHandler interface so that you
// listen for creation/update/removal of a resource
type Subprobe interface {
	Start() error
	Stop()
	graph.ListenerHandler
}

// Linker defines a k8s linker
type Linker struct {
	*graph.ResourceLinker
}

// OnError implements the LinkerEventListener interface
func (l *Linker) OnError(err error) {
	logging.GetLogger().Error(err)
}

// Start k8s probe
func (p *Probe) Start() error {
	for _, linker := range p.linkers {
		if err := linker.Start(); err != nil {
			return err
		}
	}

	for _, subprobe := range p.subprobes {
		if err := subprobe.Start(); err != nil {
			return err
		}
	}

	for _, verifier := range p.verifiers {
		if err := verifier.Start(); err != nil {
			return err
		}
	}

	return nil
}

// Stop k8s probe
func (p *Probe) Stop() {
	for _, linker := range p.linkers {
		linker.Stop()
	}

	for _, subprobe := range p.subprobes {
		subprobe.Stop()
	}

	for _, verifier := range p.verifiers {
		verifier.Stop()
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
func NewProbe(g *graph.Graph, manager string, subprobes map[string]Subprobe, linkers []probe.Handler, verifiers []probe.Handler) *Probe {
	names := []string{}
	for k := range subprobes {
		names = append(names, k)
	}
	logging.GetLogger().Infof("Probe %s subprobes %v", manager, names)
	return &Probe{
		graph:     g,
		manager:   manager,
		subprobes: subprobes,
		linkers:   linkers,
		verifiers: verifiers,
	}
}

// SubprobeHandler the signature of ctor of a subprobe
type SubprobeHandler func(client interface{}, g *graph.Graph) Subprobe

// InitSubprobes initializes only the subprobes which are enabled
func InitSubprobes(enabled []string, subprobeHandlers map[string]SubprobeHandler, client interface{}, g *graph.Graph, manager, clusterName string) {
	if subprobes[manager] == nil {
		subprobes[manager] = make(map[string]Subprobe)
	}

	if len(enabled) == 0 {
		for name := range subprobeHandlers {
			enabled = append(enabled, name)
		}
	}

	for _, name := range enabled {
		if handler := subprobeHandlers[name]; handler != nil {
			subprobe := handler(client, g)
			if resourceCache, ok := subprobe.(*ResourceCache); ok {
				resourceCache.clusterName = clusterName
			}

			PutSubprobe(manager, name, subprobe)
		}
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
