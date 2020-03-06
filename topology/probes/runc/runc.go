// +build linux

/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package runc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/safchain/insanelock"
	"github.com/spf13/cast"
	"github.com/vishvananda/netns"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/process"
	"github.com/skydive-project/skydive/topology"
	tp "github.com/skydive-project/skydive/topology/probes"
	ns "github.com/skydive-project/skydive/topology/probes/netns"
)

// ProbeHandler describes a Docker topology graph that enhance the graph
type ProbeHandler struct {
	insanelock.RWMutex
	*ns.ProbeHandler
	state          service.State
	hostNs         netns.NsHandle
	wg             sync.WaitGroup
	watcher        *fsnotify.Watcher
	paths          []string
	containers     map[string]*container
	containersLock insanelock.RWMutex
}

type container struct {
	namespace string
	node      *graph.Node
}

func (p *ProbeHandler) containerNamespace(pid int) string {
	return fmt.Sprintf("/proc/%d/ns/net", pid)
}

type initProcessStart int64

// mount contains source (host machine) to destination (guest machine) mappings
type mount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type containerState struct {
	path             string
	ID               string           `json:"id"`
	InitProcessPid   int              `json:"init_process_pid"`
	InitProcessStart initProcessStart `json:"init_process_start"`
	Config           struct {
		Labels []string `json:"labels"`
		Mounts []mount  `json:"mounts"`
	} `json:"config"`
}

// UnmarshalJSON custom marshall to handle both string and int64
func (ips *initProcessStart) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	i, err := cast.ToInt64E(v)
	if err != nil {
		return err
	}

	*ips = initProcessStart(i)

	return nil
}

func (p *ProbeHandler) getLabels(raw []string) graph.Metadata {
	labels := graph.Metadata{}

	for _, label := range raw {
		kv := strings.SplitN(label, "=", 2)
		switch len(kv) {
		case 1:
			p.Ctx.Logger.Warningf("Label format should be key=value: %s", label)
		case 2:
			var value interface{}
			if err := json.Unmarshal([]byte(kv[1]), &value); err != nil {
				labels.SetFieldAndNormalize(kv[0], kv[1])
			} else {
				labels.SetFieldAndNormalize(kv[0], value)
			}
		}
	}

	return labels
}

func getCreateConfig(path string) (*CreateConfig, error) {
	body, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("Unable to read create config %s: %s", path, err)
	}

	var cc CreateConfig
	if err := json.Unmarshal(body, &cc); err != nil {
		return nil, fmt.Errorf("Unable to parse create config %s: %s", path, err)
	}

	return &cc, nil
}

func getStatus(state *containerState) string {
	if state.InitProcessPid == 0 {
		return "stopped"
	}

	info, err := process.GetInfo(state.InitProcessPid)
	if err != nil {
		return "stopped"
	}
	if info.Start != int64(state.InitProcessStart) || info.State == process.Zombie || info.State == process.Dead {
		return "stopped"
	}
	base := path.Base(state.path)
	if _, err := os.Stat(path.Join(base, "exec.fifo")); err == nil {
		return "created"
	}
	return "running"
}

func parseState(path string) (*containerState, error) {
	body, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Unable to read container state %s: %s", path, err)
	}

	var state containerState
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, fmt.Errorf("Unable to parse container state %s: %s", path, err)
	}

	state.path = path

	return &state, nil
}

func getHostsFromState(state *containerState) (string, error) {
	const path = "/etc/hosts"
	for _, mount := range state.Config.Mounts {
		if mount.Destination == path {
			return mount.Source, nil
		}
	}
	return "", fmt.Errorf("Unable to find binding of %s", path)
}

func (p *ProbeHandler) parseHosts(state *containerState) *Hosts {
	path, err := getHostsFromState(state)
	if err != nil {
		return nil
	}

	hosts, err := readHosts(path)
	if err != nil {
		if os.IsNotExist(err) {
			p.Ctx.Logger.Debug(err)
		} else {
			p.Ctx.Logger.Error(err)
		}

		return nil
	}

	return hosts
}

func (p *ProbeHandler) getMetadata(state *containerState) Metadata {
	m := Metadata{
		ContainerID: state.ID,
		Status:      getStatus(state),
	}

	if labels := p.getLabels(state.Config.Labels); len(labels) > 0 {
		m.Labels = labels

		if b, ok := labels["bundle"]; ok {
			cc, err := getCreateConfig(b.(string) + "/artifacts/create-config")
			if err != nil {
				p.Ctx.Logger.Error(err)
			} else {
				m.CreateConfig = cc
			}
		}
	}

	if hosts := p.parseHosts(state); hosts != nil {
		m.Hosts = hosts
	}

	return m
}

func (p *ProbeHandler) updateContainer(path string, cnt *container) error {
	state, err := parseState(path)
	if err != nil {
		return err
	}

	p.Ctx.Graph.Lock()
	p.Ctx.Graph.AddMetadata(cnt.node, "Runc", p.getMetadata(state))
	p.Ctx.Graph.Unlock()

	return nil
}

func (p *ProbeHandler) registerContainer(path string) error {
	state, err := parseState(path)
	if err != nil {
		return err
	}

	p.Lock()
	defer p.Unlock()

	nsHandle, err := netns.GetFromPid(state.InitProcessPid)
	if err != nil {
		return fmt.Errorf("Unable to open netns, pid %d, state file %s : %s", state.InitProcessPid, path, err)
	}
	defer nsHandle.Close()

	namespace := p.containerNamespace(state.InitProcessPid)
	p.Ctx.Logger.Debugf("Register runc container %s and PID %d", state.ID, state.InitProcessPid)

	var n *graph.Node
	if p.hostNs.Equal(nsHandle) {
		// The container is in net=host mode
		n = p.Ctx.RootNode
	} else {
		if n, err = p.Register(namespace, state.ID); err != nil {
			return err
		}

		p.Ctx.Graph.Lock()
		tr := p.Ctx.Graph.StartMetadataTransaction(n)
		tr.AddMetadata("Runtime", "runc")

		if _, err := n.GetFieldString("Manager"); err != nil {
			tr.AddMetadata("Manager", "runc")
		}
		if err := tr.Commit(); err != nil {
			p.Ctx.Logger.Error(err)
		}
		p.Ctx.Graph.Unlock()
	}

	pid := int64(state.InitProcessPid)
	runcMetadata := p.getMetadata(state)

	p.Ctx.Graph.Lock()
	defer p.Ctx.Graph.Unlock()

	containerNode := p.Ctx.Graph.LookupFirstNode(graph.Metadata{"InitProcessPID": pid})
	if containerNode != nil {
		tr := p.Ctx.Graph.StartMetadataTransaction(containerNode)
		tr.AddMetadata("Runc", runcMetadata)
		tr.AddMetadata("Runtime", "runc")
		if err := tr.Commit(); err != nil {
			p.Ctx.Logger.Error(err)
		}
	} else {
		metadata := graph.Metadata{
			"Type":           "container",
			"Name":           state.ID,
			"Manager":        "runc",
			"Runtime":        "runc",
			"InitProcessPID": pid,
			"Runc":           runcMetadata,
		}

		if containerNode, err = p.Ctx.Graph.NewNode(graph.GenID(), metadata); err != nil {
			return err
		}
	}
	topology.AddOwnershipLink(p.Ctx.Graph, n, containerNode, nil)

	p.containersLock.Lock()
	p.containers[path] = &container{
		namespace: namespace,
		node:      containerNode,
	}
	p.containersLock.Unlock()

	return nil
}

func (p *ProbeHandler) initializeFolder(path string) {
	if err := p.watcher.Add(path); err != nil {
		p.Ctx.Logger.Error(err)
	}

	files, _ := ioutil.ReadDir(path)
	for _, f := range files {
		subpath := path + "/" + f.Name()

		if p.isFolder(subpath) {
			p.initializeFolder(subpath)
		} else if p.isStatePath(subpath) {
			p.containersLock.RLock()
			c, ok := p.containers[subpath]
			p.containersLock.RUnlock()

			var err error
			if ok {
				err = p.updateContainer(subpath, c)
			} else {
				err = p.registerContainer(subpath)
			}
			if err != nil {
				p.Ctx.Logger.Error(err)
			}
		}
	}
}

func (p *ProbeHandler) initialize(path string) {
	defer p.wg.Done()

	for p.state.Load() == service.RunningState {
		if _, err := os.Stat(path); err == nil {
			p.initializeFolder(path)
			p.Ctx.Logger.Debugf("Probe initialized for %s", path)
			return
		}

		time.Sleep(time.Second)
	}
}

func (p *ProbeHandler) isStatePath(path string) bool {
	return strings.HasSuffix(path, "/state.json")
}

func (p *ProbeHandler) isFolder(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

// Start the probe
func (p *ProbeHandler) start() {
	defer p.wg.Done()

	for _, path := range p.paths {
		p.wg.Add(1)
		go p.initialize(path)
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for p.state.Load() == service.RunningState {
		select {
		case ev := <-p.watcher.Events:
			if ev.Op&fsnotify.Create == fsnotify.Create {
				if p.isFolder(ev.Name) {
					p.watcher.Add(ev.Name)
				}
			}
			if ev.Op&fsnotify.Write == fsnotify.Write {
				if p.isStatePath(ev.Name) {
					p.containersLock.RLock()
					cnt, ok := p.containers[ev.Name]
					p.containersLock.RUnlock()

					var err error
					if ok {
						err = p.updateContainer(ev.Name, cnt)
					} else {
						err = p.registerContainer(ev.Name)
					}
					if err != nil {
						p.Ctx.Logger.Error(err)
					}
				}
			}
			if ev.Op&fsnotify.Remove == fsnotify.Remove {
				if p.isStatePath(ev.Name) {
					p.containersLock.RLock()
					cnt, ok := p.containers[ev.Name]
					p.containersLock.RUnlock()

					if ok {
						p.Unregister(cnt.namespace)

						p.containersLock.Lock()
						delete(p.containers, ev.Name)
						p.containersLock.Unlock()
					}
				} else {
					p.containersLock.Lock()
					for path, cnt := range p.containers {
						if strings.HasPrefix(path, ev.Name) {
							p.Unregister(cnt.namespace)

							delete(p.containers, path)
						}
					}
					p.containersLock.Unlock()
				}
			}
		case err := <-p.watcher.Errors:
			p.Ctx.Logger.Errorf("Error while watching runc state folder: %s", err)
		case <-ticker.C:
		}
	}
}

// Start the probe
func (p *ProbeHandler) Start() error {
	if !p.state.CompareAndSwap(service.StoppedState, service.RunningState) {
		return probe.ErrNotStopped
	}

	var err error
	if p.hostNs, err = netns.Get(); err != nil {
		p.Ctx.Logger.Errorf("Unable to get host namespace: %s", err)
		return err
	}

	p.wg.Add(1)
	go p.start()
	return nil
}

// Stop the probe
func (p *ProbeHandler) Stop() {
	if !p.state.CompareAndSwap(service.RunningState, service.StoppingState) {
		return
	}
	p.wg.Wait()

	p.hostNs.Close()

	p.state.Store(service.StoppedState)
}

// NewProbe returns a new runc topology probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	nsHandler := bundle.GetHandler("netns")
	if nsHandler == nil {
		return nil, errors.New("unable to find the netns handler")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("Unable to create a new Watcher: %s", err)
	}

	return &ProbeHandler{
		ProbeHandler: nsHandler.(*ns.ProbeHandler),
		state:        service.StoppedState,
		watcher:      watcher,
		paths:        ctx.Config.GetStringSlice("agent.topology.runc.run_path"),
		containers:   make(map[string]*container),
	}, nil
}

// Register registers graph metadata decoders
func Register() {
	graph.NodeMetadataDecoders["Runc"] = MetadataDecoder
}
