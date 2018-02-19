// +build linux

/*
 * Copyright (C) 2018 Iain Grant
 * Copyright (C) 2018 Red Hat, Inc.
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

package lxd

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	lxd "github.com/lxc/lxd/client"
	"github.com/mitchellh/mapstructure"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	ns "github.com/skydive-project/skydive/topology/probes/netns"
	"github.com/vishvananda/netns"
)

type containerMetadata struct {
	Architecture string
	Config       map[string]interface{}
	CreatedAt    string
	Description  string
	Devices      map[string]map[string]string
	Ephemeral    string
	Profiles     []string
	Restore      string
	Status       string
	Stateful     string
}

type containerInfo struct {
	Pid  int
	Node *graph.Node
}

type loggingEvent struct {
	Type     string `mapstructure:"type"`
	Metadata struct {
		Context struct {
			Action string `mapstructure:"action"`
			Name   string `mapstructure:"name"`
		} `mapstructure:"context"`
		Level   string `mapstructure:"level"`
		Message string `mapstructure:"message"`
	} `mapstructure:"metadata"`
}

// LxdProbe describes a LXD topology graph that enhance the graph
type LxdProbe struct {
	sync.RWMutex
	*ns.NetNSProbe
	state        int64
	wg           sync.WaitGroup
	connected    atomic.Value
	cancel       context.CancelFunc
	quit         chan struct{}
	containerMap map[string]containerInfo
	hostNs       netns.NsHandle
	client       lxd.ContainerServer
}

func (probe *LxdProbe) containerNamespace(pid int) string {
	return fmt.Sprintf("/proc/%d/ns/net", pid)
}

func (probe *LxdProbe) registerContainer(id string) {
	logging.GetLogger().Debugf("Registering container %s", id)

	probe.Lock()
	defer probe.Unlock()

	if _, ok := probe.containerMap[id]; ok {
		return
	}

	container, _, err := probe.client.GetContainer(id)
	if err != nil {
		logging.GetLogger().Errorf("Failed to retrieve container %s", id)
		return
	}

	// state.Network[].HostName == host side interface
	// state.Pid for lookup of network namespace
	state, _, _ := probe.client.GetContainerState(id)

	if state.Status != "Running" {
		logging.GetLogger().Errorf("Container %s is not running", id)
		return
	}

	nsHandle, err := netns.GetFromPid(int(state.Pid))
	if err != nil {
		return
	}
	defer nsHandle.Close()

	namespace := probe.containerNamespace(int(state.Pid))

	var n *graph.Node
	if probe.hostNs.Equal(nsHandle) {
		n = probe.Root
	} else {
		if n, err = probe.Register(namespace, id); err == nil {
			probe.Graph.Lock()
			probe.Graph.AddMetadata(n, "Manager", "lxd")
			probe.Graph.Unlock()
		} else {
			logging.GetLogger().Errorf("Error registering probe: %s", err)
		}
	}

	metadata := containerMetadata{
		Architecture: container.Architecture,
		CreatedAt:    container.CreatedAt.String(),
		Description:  container.Description,
		Devices:      container.Devices,
		Ephemeral:    strconv.FormatBool(container.Ephemeral),
		Profiles:     container.Profiles,
		Restore:      container.Restore,
		Stateful:     strconv.FormatBool(container.Stateful),
		Status:       container.Status,
	}

	if len(container.Config) != 0 {
		metadata.Config = common.NormalizeValue(container.Config).(map[string]interface{})
	}

	probe.Graph.Lock()

	containerNode := probe.Graph.NewNode(graph.GenID(), graph.Metadata{
		"Type": "container",
		"Name": id,
		"LXD":  metadata,
	})

	topology.AddOwnershipLink(probe.Graph, n, containerNode, nil)
	probe.Graph.Unlock()

	probe.containerMap[id] = containerInfo{
		Pid:  int(state.Pid),
		Node: containerNode,
	}
}

func (probe *LxdProbe) unregisterContainer(id string) {
	probe.Lock()
	defer probe.Unlock()

	infos, ok := probe.containerMap[id]
	if !ok {
		return
	}

	probe.Graph.Lock()
	probe.Graph.DelNode(infos.Node)
	probe.Graph.Unlock()

	namespace := probe.containerNamespace(infos.Pid)
	probe.Unregister(namespace)

	delete(probe.containerMap, id)
}

func (probe *LxdProbe) connect() (err error) {
	if probe.hostNs, err = netns.Get(); err != nil {
		return err
	}
	defer probe.hostNs.Close()

	probe.wg.Add(1)
	defer probe.wg.Done()

	logging.GetLogger().Debugf("Connecting to LXD")
	client, err := lxd.ConnectLXDUnix("", nil)
	if err != nil {
		return err
	}
	probe.client = client

	events, err := client.GetEvents()
	if err != nil {
		return err
	}

	target, err := events.AddHandler(nil, func(obj interface{}) {
		var event loggingEvent
		if err := mapstructure.Decode(obj, &event); err != nil {
			return
		}

		if event.Type == "logging" {
			if event.Metadata.Context.Action == "start" && event.Metadata.Message == "Started container" {
				probe.registerContainer(event.Metadata.Context.Name)
			} else if event.Metadata.Message == "Deleted container" {
				probe.unregisterContainer(event.Metadata.Context.Name)
			}
		}
	})

	if err != nil {
		return err
	}
	defer events.RemoveHandler(target)

	probe.connected.Store(true)
	defer probe.connected.Store(false)

	go func() {
		defer probe.wg.Done()

		logging.GetLogger().Debugf("Listing LXD containers")
		containers, err := probe.client.GetContainers()
		if err != nil {
			return
		}

		for _, n := range containers {
			probe.registerContainer(n.Name)
		}
	}()

	<-probe.quit

	return nil
}

// Start the probe
func (probe *LxdProbe) Start() {
	if !atomic.CompareAndSwapInt64(&probe.state, common.StoppedState, common.RunningState) {
		return
	}

	go func() {
		for {
			state := atomic.LoadInt64(&probe.state)
			if state == common.StoppingState || state == common.StoppedState {
				break
			}

			if probe.connect() != nil {
				time.Sleep(1 * time.Second)
			}

			probe.wg.Wait()
		}
	}()
}

// Stop the probe
func (probe *LxdProbe) Stop() {
	if !atomic.CompareAndSwapInt64(&probe.state, common.RunningState, common.StoppingState) {
		return
	}

	if probe.connected.Load() == true {
		probe.cancel()
		probe.quit <- struct{}{}
		probe.wg.Wait()
	}

	atomic.StoreInt64(&probe.state, common.StoppedState)
}

// NewLxdProbe creates a new topology Lxd probe
func NewLxdProbe(nsProbe *ns.NetNSProbe, lxdURL string) (*LxdProbe, error) {
	probe := &LxdProbe{
		NetNSProbe:   nsProbe,
		state:        common.StoppedState,
		containerMap: make(map[string]containerInfo),
		quit:         make(chan struct{}),
	}

	return probe, nil
}
