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
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vishvananda/netns"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	ns "github.com/skydive-project/skydive/topology/probes/netns"
)

// ProbeHandler describes a Docker topology graph that enhance the graph
type ProbeHandler struct {
	common.RWMutex
	*ns.ProbeHandler
	state          int64
	hostNs         netns.NsHandle
	wg             sync.WaitGroup
	watcher        *fsnotify.Watcher
	paths          []string
	containers     map[string]*container
	containersLock common.RWMutex
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

	i, err := common.ToInt64(v)
	if err != nil {
		return err
	}

	*ips = initProcessStart(i)

	return nil
}

func getLabels(raw []string) graph.Metadata {
	labels := graph.Metadata{}

	for _, label := range raw {
		kv := strings.SplitN(label, "=", 2)
		switch len(kv) {
		case 1:
			logging.GetLogger().Warningf("Label format should be key=value: %s", label)
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

	info, err := common.GetProcessInfo(state.InitProcessPid)
	if err != nil {
		return "stopped"
	}
	if info.Start != int64(state.InitProcessStart) || info.State == common.Zombie || info.State == common.Dead {
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

func parseHosts(state *containerState) *Hosts {
	path, err := getHostsFromState(state)
	if err != nil {
		return nil
	}

	hosts, err := readHosts(path)
	if err != nil {
		if os.IsNotExist(err) {
			logging.GetLogger().Debug(err)
		} else {
			logging.GetLogger().Error(err)
		}

		return nil
	}

	return hosts
}

func getMetadata(state *containerState) Metadata {
	m := Metadata{
		ContainerID: state.ID,
		Status:      getStatus(state),
	}

	if labels := getLabels(state.Config.Labels); len(labels) > 0 {
		m.Labels = labels

		if b, ok := labels["bundle"]; ok {
			cc, err := getCreateConfig(b.(string) + "/artifacts/create-config")
			if err != nil {
				logging.GetLogger().Error(err)
			} else {
				m.CreateConfig = cc
			}
		}
	}

	if hosts := parseHosts(state); hosts != nil {
		m.Hosts = hosts
	}

	return m
}

func (p *ProbeHandler) updateContainer(path string, cnt *container) error {
	state, err := parseState(path)
	if err != nil {
		return err
	}

	p.Graph.Lock()
	p.Graph.AddMetadata(cnt.node, "Runc", getMetadata(state))
	p.Graph.Unlock()

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
		return err
	}
	defer nsHandle.Close()

	namespace := p.containerNamespace(state.InitProcessPid)
	logging.GetLogger().Debugf("Register runc container %s and PID %d", state.ID, state.InitProcessPid)

	var n *graph.Node
	if p.hostNs.Equal(nsHandle) {
		// The container is in net=host mode
		n = p.Root
	} else {
		if n, err = p.Register(namespace, state.ID); err != nil {
			return err
		}

		p.Graph.Lock()
		tr := p.Graph.StartMetadataTransaction(n)
		tr.AddMetadata("Runtime", "runc")

		if _, err := n.GetFieldString("Manager"); err != nil {
			tr.AddMetadata("Manager", "runc")
		}
		if err := tr.Commit(); err != nil {
			logging.GetLogger().Error(err)
		}
		p.Graph.Unlock()
	}

	pid := int64(state.InitProcessPid)
	runcMetadata := getMetadata(state)

	p.Graph.Lock()
	defer p.Graph.Unlock()

	containerNode := p.Graph.LookupFirstNode(graph.Metadata{"InitProcessPID": pid})
	if containerNode != nil {
		tr := p.Graph.StartMetadataTransaction(containerNode)
		tr.AddMetadata("Runc", runcMetadata)
		tr.AddMetadata("Runtime", "runc")
		if err := tr.Commit(); err != nil {
			logging.GetLogger().Error(err)
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

		if containerNode, err = p.Graph.NewNode(graph.GenID(), metadata); err != nil {
			return err
		}
	}
	topology.AddOwnershipLink(p.Graph, n, containerNode, nil)

	p.containers[path] = &container{
		namespace: namespace,
		node:      containerNode,
	}

	return nil
}

func (p *ProbeHandler) initializeFolder(path string) {
	p.watcher.Add(path)

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
				logging.GetLogger().Error(err)
			}
		}
	}
}

func (p *ProbeHandler) initialize(path string) {
	defer p.wg.Done()

	common.Retry(func() error {
		if atomic.LoadInt64(&p.state) != common.RunningState {
			return nil
		}

		if _, err := os.Stat(path); err != nil {
			return err
		}

		p.initializeFolder(path)

		logging.GetLogger().Debugf("Probe initialized for %s", path)

		return nil
	}, math.MaxInt32, time.Second)
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

	var err error
	if p.hostNs, err = netns.Get(); err != nil {
		logging.GetLogger().Errorf("Unable to get host namespace: %s", err)
		return
	}

	if !atomic.CompareAndSwapInt64(&p.state, common.StoppedState, common.RunningState) {
		return
	}

	for _, path := range p.paths {
		p.wg.Add(1)
		go p.initialize(path)
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for atomic.LoadInt64(&p.state) == common.RunningState {
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
						logging.GetLogger().Error(err)
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
			logging.GetLogger().Errorf("Error while watching runc state folder: %s", err)
		case <-ticker.C:
		}
	}
}

// Start the probe
func (p *ProbeHandler) Start() {
	p.wg.Add(1)
	go p.start()
}

// Stop the probe
func (p *ProbeHandler) Stop() {
	if !atomic.CompareAndSwapInt64(&p.state, common.RunningState, common.StoppingState) {
		return
	}
	p.wg.Wait()

	p.hostNs.Close()

	atomic.StoreInt64(&p.state, common.StoppedState)
}

// NewProbeHandler creates a new topology runc probe
func NewProbeHandler(nsHandler *ns.ProbeHandler) (*ProbeHandler, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("Unable to create a new Watcher: %s", err)
	}

	paths := config.GetStringSlice("agent.topology.runc.run_path")

	handler := &ProbeHandler{
		ProbeHandler: nsHandler,
		state:        common.StoppedState,
		watcher:      watcher,
		paths:        paths,
		containers:   make(map[string]*container),
	}

	return handler, nil
}
