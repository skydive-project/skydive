// +build linux

/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package docker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/vishvananda/netns"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	ns "github.com/skydive-project/skydive/topology/probes/netns"
	sversion "github.com/skydive-project/skydive/version"
)

// ClientAPIVersion Client API version used
const ClientAPIVersion = "1.18"

type containerInfo struct {
	Pid  int
	Node *graph.Node
}

// Probe describes a Docker topology graph that enhance the graph
type Probe struct {
	common.RWMutex
	*ns.Probe
	url          string
	client       *client.Client
	cancel       context.CancelFunc
	state        int64
	connected    atomic.Value
	wg           sync.WaitGroup
	hostNs       netns.NsHandle
	containerMap map[string]containerInfo
}

func (probe *Probe) containerNamespace(pid int) string {
	return fmt.Sprintf("/proc/%d/ns/net", pid)
}

func (probe *Probe) registerContainer(id string) {
	probe.Lock()
	defer probe.Unlock()

	if _, ok := probe.containerMap[id]; ok {
		return
	}
	info, err := probe.client.ContainerInspect(context.Background(), id)
	if err != nil {
		logging.GetLogger().Errorf("Failed to inspect Docker container %s: %s", id, err)
		return
	}

	nsHandle, err := netns.GetFromPid(info.State.Pid)
	if err != nil {
		return
	}
	defer nsHandle.Close()

	namespace := probe.containerNamespace(info.State.Pid)
	logging.GetLogger().Debugf("Register docker container %s and PID %d", info.ID, info.State.Pid)

	var n *graph.Node
	if probe.hostNs.Equal(nsHandle) {
		// The container is in net=host mode
		n = probe.Root
	} else {
		if n, err = probe.Register(namespace, info.Name[1:]); err != nil {
			logging.GetLogger().Debugf("Failed to register probe for namespace %s: %s", namespace, err)
			return
		}

		probe.Graph.Lock()
		probe.Graph.AddMetadata(n, "Manager", "docker")
		probe.Graph.Unlock()
	}

	probe.Graph.Lock()
	metadata := graph.Metadata{
		"Type":    "container",
		"Name":    info.Name[1:],
		"Manager": "docker",
		"Docker": map[string]interface{}{
			"ContainerID":   info.ID,
			"ContainerName": info.Name,
			"ContainerPID":  int64(info.State.Pid),
		},
	}

	if len(info.Config.Labels) != 0 {
		metadata["Docker"].(map[string]interface{})["Labels"] = common.NormalizeValue(info.Config.Labels)
	}

	containerNode := probe.Graph.NewNode(graph.GenID(), metadata)
	topology.AddOwnershipLink(probe.Graph, n, containerNode, nil)
	probe.Graph.Unlock()

	probe.containerMap[info.ID] = containerInfo{
		Pid:  info.State.Pid,
		Node: containerNode,
	}
}

func (probe *Probe) unregisterContainer(id string) {
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
	logging.GetLogger().Debugf("Stop listening for namespace %s with PID %d", namespace, infos.Pid)
	probe.Unregister(namespace)

	delete(probe.containerMap, id)
}

func (probe *Probe) handleDockerEvent(event *events.Message) {
	if event.Status == "start" {
		probe.registerContainer(event.ID)
	} else if event.Status == "die" {
		probe.unregisterContainer(event.ID)
	}
}

func (probe *Probe) connect() error {
	var err error

	logging.GetLogger().Debugf("Connecting to Docker daemon: %s", probe.url)
	defaultHeaders := map[string]string{"User-Agent": fmt.Sprintf("skydive-agent-%s", sversion.Version)}
	probe.client, err = client.NewClient(probe.url, ClientAPIVersion, nil, defaultHeaders)
	if err != nil {
		logging.GetLogger().Errorf("Failed to create client to Docker daemon: %s", err)
		return err
	}
	defer probe.client.Close()

	if _, err := probe.client.ServerVersion(context.Background()); err != nil {
		logging.GetLogger().Errorf("Failed to connect to Docker daemon: %s", err)
		return err
	}

	if probe.hostNs, err = netns.Get(); err != nil {
		return err
	}
	defer probe.hostNs.Close()

	for id := range probe.containerMap {
		probe.unregisterContainer(id)
	}

	eventsFilter := filters.NewArgs()
	eventsFilter.Add("event", "start")
	eventsFilter.Add("event", "die")

	ctx, cancel := context.WithCancel(context.Background())
	eventChan, errChan := probe.client.Events(ctx, types.EventsOptions{Filters: eventsFilter})

	probe.cancel = cancel
	probe.wg.Add(2)

	probe.connected.Store(true)
	defer probe.connected.Store(false)

	go func() {
		defer probe.wg.Done()

		containers, err := probe.client.ContainerList(ctx, types.ContainerListOptions{})
		if err != nil {
			logging.GetLogger().Errorf("Failed to list containers: %s", err)
			return
		}

		for _, c := range containers {
			if atomic.LoadInt64(&probe.state) != common.RunningState {
				break
			}
			probe.registerContainer(c.ID)
		}
	}()

	defer probe.wg.Done()

	for {
		select {
		case err := <-errChan:
			if atomic.LoadInt64(&probe.state) != common.StoppingState {
				err = fmt.Errorf("Got error while waiting for Docker event: %s", err)
			}
			return err
		case event := <-eventChan:
			probe.handleDockerEvent(&event)
		}
	}
}

// Start the probe
func (probe *Probe) Start() {
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
func (probe *Probe) Stop() {
	if !atomic.CompareAndSwapInt64(&probe.state, common.RunningState, common.StoppingState) {
		return
	}

	if probe.connected.Load() == true {
		probe.cancel()
		probe.wg.Wait()
	}

	atomic.StoreInt64(&probe.state, common.StoppedState)
}

// NewProbe creates a new topology Docker probe
func NewProbe(nsProbe *ns.Probe, dockerURL string) (*Probe, error) {
	Probe := &Probe{
		Probe:        nsProbe,
		url:          dockerURL,
		containerMap: make(map[string]containerInfo),
		state:        common.StoppedState,
	}

	if path := config.GetString("docker.netns.run_path"); path != "" {
		nsProbe.Exclude(path + "/default")
		nsProbe.Watch(path)
	}

	return Probe, nil
}
