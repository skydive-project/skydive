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

// Probe describes a Docker topology graph that enhance the graph
type Probe struct {
	common.RWMutex
	*ns.Probe
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

func (probe *Probe) containerNamespace(pid int) string {
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
		logging.GetLogger().Error(err)
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

func (probe *Probe) updateContainer(path string, cnt *container) error {
	state, err := parseState(path)
	if err != nil {
		return err
	}

	probe.Graph.Lock()
	probe.Graph.AddMetadata(cnt.node, "Runc", getMetadata(state))
	probe.Graph.Unlock()

	return nil
}

func (probe *Probe) registerContainer(path string) error {
	state, err := parseState(path)
	if err != nil {
		return err
	}

	probe.Lock()
	defer probe.Unlock()

	nsHandle, err := netns.GetFromPid(state.InitProcessPid)
	if err != nil {
		return err
	}
	defer nsHandle.Close()

	namespace := probe.containerNamespace(state.InitProcessPid)
	logging.GetLogger().Debugf("Register runc container %s and PID %d", state.ID, state.InitProcessPid)

	var n *graph.Node
	if probe.hostNs.Equal(nsHandle) {
		// The container is in net=host mode
		n = probe.Root
	} else {
		if n, err = probe.Register(namespace, state.ID); err != nil {
			return err
		}

		probe.Graph.Lock()
		tr := probe.Graph.StartMetadataTransaction(n)
		tr.AddMetadata("Runtime", "runc")

		if _, err := n.GetFieldString("Manager"); err != nil {
			tr.AddMetadata("Manager", "runc")
		}
		if err := tr.Commit(); err != nil {
			logging.GetLogger().Error(err)
		}
		probe.Graph.Unlock()
	}

	pid := int64(state.InitProcessPid)
	runcMetadata := getMetadata(state)

	probe.Graph.Lock()
	defer probe.Graph.Unlock()

	containerNode := probe.Graph.LookupFirstNode(graph.Metadata{"InitProcessPID": pid})
	if containerNode != nil {
		tr := probe.Graph.StartMetadataTransaction(containerNode)
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

		if containerNode, err = probe.Graph.NewNode(graph.GenID(), metadata); err != nil {
			return err
		}
	}
	topology.AddOwnershipLink(probe.Graph, n, containerNode, nil)

	probe.containers[path] = &container{
		namespace: namespace,
		node:      containerNode,
	}

	return nil
}

func (probe *Probe) initializeFolder(path string) {
	probe.watcher.Add(path)

	files, _ := ioutil.ReadDir(path)
	for _, f := range files {
		subpath := path + "/" + f.Name()

		if probe.isFolder(subpath) {
			probe.initializeFolder(subpath)
		} else if probe.isStatePath(subpath) {
			probe.containersLock.RLock()
			c, ok := probe.containers[subpath]
			probe.containersLock.RUnlock()

			var err error
			if ok {
				err = probe.updateContainer(subpath, c)
			} else {
				err = probe.registerContainer(subpath)
			}
			if err != nil {
				logging.GetLogger().Error(err)
			}
		}
	}
}

func (probe *Probe) initialize(path string) {
	defer probe.wg.Done()

	common.Retry(func() error {
		if atomic.LoadInt64(&probe.state) != common.RunningState {
			return nil
		}

		if _, err := os.Stat(path); err != nil {
			return err
		}

		probe.initializeFolder(path)

		logging.GetLogger().Debugf("Probe initialized for %s", path)

		return nil
	}, math.MaxInt32, time.Second)
}

func (probe *Probe) isStatePath(path string) bool {
	return strings.HasSuffix(path, "/state.json")
}

func (probe *Probe) isFolder(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

// Start the probe
func (probe *Probe) start() {
	defer probe.wg.Done()

	var err error
	if probe.hostNs, err = netns.Get(); err != nil {
		logging.GetLogger().Errorf("Unable to get host namespace: %s", err)
		return
	}

	if !atomic.CompareAndSwapInt64(&probe.state, common.StoppedState, common.RunningState) {
		return
	}

	for _, path := range probe.paths {
		probe.wg.Add(1)
		go probe.initialize(path)
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for atomic.LoadInt64(&probe.state) == common.RunningState {
		select {
		case ev := <-probe.watcher.Events:
			if ev.Op&fsnotify.Create == fsnotify.Create {
				if probe.isFolder(ev.Name) {
					probe.watcher.Add(ev.Name)
				}
			}
			if ev.Op&fsnotify.Write == fsnotify.Write {
				if probe.isStatePath(ev.Name) {
					probe.containersLock.RLock()
					cnt, ok := probe.containers[ev.Name]
					probe.containersLock.RUnlock()

					var err error
					if ok {
						err = probe.updateContainer(ev.Name, cnt)
					} else {
						err = probe.registerContainer(ev.Name)
					}
					if err != nil {
						logging.GetLogger().Error(err)
					}
				}
			}
			if ev.Op&fsnotify.Remove == fsnotify.Remove {
				if probe.isStatePath(ev.Name) {
					probe.containersLock.RLock()
					cnt, ok := probe.containers[ev.Name]
					probe.containersLock.RUnlock()

					if ok {
						probe.Unregister(cnt.namespace)

						probe.containersLock.Lock()
						delete(probe.containers, ev.Name)
						probe.containersLock.Unlock()
					}
				} else {
					probe.containersLock.Lock()
					for path, cnt := range probe.containers {
						if strings.HasPrefix(path, ev.Name) {
							probe.Unregister(cnt.namespace)

							delete(probe.containers, path)
						}
					}
					probe.containersLock.Unlock()
				}
			}
		case err := <-probe.watcher.Errors:
			logging.GetLogger().Errorf("Error while watching runc state folder: %s", err)
		case <-ticker.C:
		}
	}
}

// Start the probe
func (probe *Probe) Start() {
	probe.wg.Add(1)
	go probe.start()
}

// Stop the probe
func (probe *Probe) Stop() {
	if !atomic.CompareAndSwapInt64(&probe.state, common.RunningState, common.StoppingState) {
		return
	}
	probe.wg.Wait()

	probe.hostNs.Close()

	atomic.StoreInt64(&probe.state, common.StoppedState)
}

// NewProbe creates a new topology runc probe
func NewProbe(nsProbe *ns.Probe) (*Probe, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("Unable to create a new Watcher: %s", err)
	}

	paths := config.GetStringSlice("agent.topology.runc.run_path")

	probe := &Probe{
		Probe:      nsProbe,
		state:      common.StoppedState,
		watcher:    watcher,
		paths:      paths,
		containers: make(map[string]*container),
	}

	return probe, nil
}
