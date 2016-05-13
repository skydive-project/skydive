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

package probes

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lebauce/dockerclient"
	"github.com/vishvananda/netns"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

const (
	StoppedState  = iota
	RunningState  = iota
	StoppingState = iota
)

type ContainerInfo struct {
	Pid  int
	Node *graph.Node
}

type DockerProbe struct {
	sync.RWMutex
	NetNSProbe
	url          string
	client       *dockerclient.DockerClient
	state        int64
	connected    atomic.Value
	quit         chan bool
	wg           sync.WaitGroup
	hostNs       netns.NsHandle
	containerMap map[string]ContainerInfo
}

func (probe *DockerProbe) containerNamespace(pid int) string {
	return fmt.Sprintf("/proc/%d/ns/net", pid)
}

func (probe *DockerProbe) registerContainer(id string) {
	probe.Lock()
	defer probe.Unlock()

	if _, ok := probe.containerMap[id]; ok {
		return
	}
	info, err := probe.client.InspectContainer(id)
	if err != nil {
		logging.GetLogger().Errorf("Failed to inspect Docker container %s: %s", id, err.Error())
		return
	}

	nsHandle, err := netns.GetFromPid(info.State.Pid)
	if err != nil {
		return
	}

	namespace := probe.containerNamespace(info.State.Pid)
	logging.GetLogger().Debugf("Register docker container %s and PID %d", info.Id, info.State.Pid)

	var n *graph.Node
	if probe.hostNs.Equal(nsHandle) {
		// The container is in net=host mode
		n = probe.Root
	} else {
		n = probe.Register(namespace, graph.Metadata{"Name": info.Name[1:], "Manager": "docker"})

	}

	probe.Graph.Lock()
	metadata := graph.Metadata{
		"Type":                 "container",
		"Name":                 info.Name[1:],
		"Docker.ContainerID":   info.Id,
		"Docker.ContainerName": info.Name,
		"Docker.ContainerPID":  info.State.Pid,
	}
	containerNode := probe.Graph.NewNode(graph.GenID(), metadata)
	probe.Graph.Link(n, containerNode, graph.Metadata{"RelationType": "membership"})
	probe.Graph.Unlock()

	probe.containerMap[info.Id] = ContainerInfo{
		Pid:  info.State.Pid,
		Node: containerNode,
	}
}

func (probe *DockerProbe) unregisterContainer(id string) {
	probe.Lock()
	defer probe.Unlock()

	infos, ok := probe.containerMap[id]
	if !ok {
		return
	}
	namespace := probe.containerNamespace(infos.Pid)
	logging.GetLogger().Debugf("Stop listening for namespace %s with PID %d", namespace, infos.Pid)
	probe.Unregister(namespace)

	probe.Graph.Lock()
	probe.Graph.DelNode(infos.Node)
	probe.Graph.Unlock()

	delete(probe.containerMap, id)
}

func (probe *DockerProbe) handleDockerEvent(event *dockerclient.Event) {
	if event.Status == "start" {
		probe.registerContainer(event.ID)
	} else if event.Status == "die" {
		probe.unregisterContainer(event.ID)
	}
}

func (probe *DockerProbe) connect() error {
	var err error

	if probe.hostNs, err = netns.Get(); err != nil {
		return err
	}

	logging.GetLogger().Debugf("Connecting to Docker daemon: %s", probe.url)
	probe.client, err = dockerclient.NewDockerClient(probe.url, nil)
	if err != nil {
		logging.GetLogger().Errorf("Failed to connect to Docker daemon: %s", err.Error())
		return err
	}

	eventsOptions := &dockerclient.MonitorEventsOptions{
		Filters: &dockerclient.MonitorEventsFilters{
			Events: []string{"start", "die"},
		},
	}

	eventErrChan, err := probe.client.MonitorEvents(eventsOptions, nil)
	if err != nil {
		logging.GetLogger().Errorf("Unable to monitor Docker events: %s", err.Error())
		return err
	}

	probe.wg.Add(2)
	probe.quit = make(chan bool)

	probe.connected.Store(true)
	defer probe.connected.Store(false)

	go func() {
		defer probe.wg.Done()

		containers, err := probe.client.ListContainers(false, false, "")
		if err != nil {
			logging.GetLogger().Errorf("Failed to list containers: %s", err.Error())
			return
		}

		for _, c := range containers {
			if atomic.LoadInt64(&probe.state) != RunningState {
				break
			}
			probe.registerContainer(c.Id)
		}
	}()

	defer probe.wg.Done()

	for {
		select {
		case <-probe.quit:
			return nil
		case e := <-eventErrChan:
			if e.Error != nil {
				logging.GetLogger().Errorf("Got error while waiting for Docker event: %s", e.Error.Error())
				return e.Error
			}
			probe.handleDockerEvent(&e.Event)
		}
	}
}

func (probe *DockerProbe) Start() {
	if !atomic.CompareAndSwapInt64(&probe.state, StoppedState, RunningState) {
		return
	}

	go func() {
		for {
			state := atomic.LoadInt64(&probe.state)
			if state == StoppingState || state == StoppedState {
				break
			}

			if probe.connect() != nil {
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func (probe *DockerProbe) Stop() {
	if !atomic.CompareAndSwapInt64(&probe.state, RunningState, StoppingState) {
		return
	}

	if probe.connected.Load() == true {
		close(probe.quit)
		probe.wg.Wait()
	}

	atomic.StoreInt64(&probe.state, StoppedState)
}

func NewDockerProbe(g *graph.Graph, n *graph.Node, dockerURL string) (probe *DockerProbe) {
	probe = &DockerProbe{
		NetNSProbe:   *NewNetNSProbe(g, n),
		url:          dockerURL,
		containerMap: make(map[string]ContainerInfo),
		state:        StoppedState,
	}
	return
}

func NewDockerProbeFromConfig(g *graph.Graph, n *graph.Node) *DockerProbe {
	dockerURL := config.GetConfig().GetString("docker.url")
	return NewDockerProbe(g, n, dockerURL)
}
