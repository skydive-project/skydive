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

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

type DockerProbe struct {
	NetNSProbe
	url     string
	client  *dockerclient.DockerClient
	nsProbe *NetNSProbe
	running atomic.Value
	quit    chan bool
	wg      sync.WaitGroup
}

type DockerContainerAttributes struct {
	ContainerID string
}

func (probe *DockerProbe) containerNamespace(info dockerclient.ContainerInfo) string {
	return fmt.Sprintf("/proc/%d/ns/net", info.State.Pid)
}

func (probe *DockerProbe) registerContainer(info dockerclient.ContainerInfo) {
	namespace := probe.containerNamespace(info)
	logging.GetLogger().Debugf("Register docker container %s and PID %d", info.Id, info.State.Pid)
	metadata := &graph.Metadata{
		"Docker.ContainerID":   info.Id,
		"Docker.ContainerName": info.Name,
	}
	probe.nsProbe.Register(namespace, metadata)
}

func (probe *DockerProbe) unregisterContainer(info dockerclient.ContainerInfo) {
	namespace := probe.containerNamespace(info)
	logging.GetLogger().Debugf("Stop listening for namespace %s with PID %d", namespace, info.State.Pid)
	probe.nsProbe.Unregister(namespace)
}

func (probe *DockerProbe) handleDockerEvent(event *dockerclient.Event) {
	info, err := probe.client.InspectContainer(event.ID)
	if err != nil {
		return
	}

	if event.Status == "start" {
		probe.registerContainer(*info)
	} else if event.Status == "die" {
		probe.unregisterContainer(*info)
	}
}

func (probe *DockerProbe) connect() {
	var err error

	logging.GetLogger().Debugf("Connecting to Docker daemon: %s", probe.url)
	probe.client, err = dockerclient.NewDockerClient(probe.url, nil)
	if err != nil {
		logging.GetLogger().Errorf("Failed to connect to Docker daemon: %s", err.Error())
		return
	}

	eventsOptions := &dockerclient.MonitorEventsOptions{
		Filters: &dockerclient.MonitorEventsFilters{
			Events: []string{"start", "die"},
		},
	}

	probe.quit = make(chan bool)
	eventErrChan, err := probe.client.MonitorEvents(eventsOptions, nil)
	if err != nil {
		logging.GetLogger().Errorf("Unable to monitor Docker events: %s", err.Error())
		return
	}

	containers, err := probe.client.ListContainers(false, false, "")
	if err != nil {
		logging.GetLogger().Errorf("Failed to list containers: %s", err.Error())
		return
	}

	probe.wg.Add(2)

	go func() {
		defer probe.wg.Done()

		for _, c := range containers {
			if probe.running.Load() == false {
				break
			}

			info, err := probe.client.InspectContainer(c.Id)
			if err != nil {
				logging.GetLogger().Errorf("Failed to inspect container %s: %s", c.Id, err.Error())
				continue
			}

			probe.registerContainer(*info)
		}
	}()

	defer probe.wg.Done()
	for {
		select {
		case <-probe.quit:
			return
		case e := <-eventErrChan:
			if e.Error != nil {
				logging.GetLogger().Errorf("Got error while waiting for Docker event: %s", e.Error.Error())
				return
			}
			probe.handleDockerEvent(&e.Event)
		}
	}
}

func (probe *DockerProbe) Start() {
	probe.running.Store(true)

	go func() {
		for probe.running.Load() == true {
			probe.connect()

			time.Sleep(1 * time.Second)
		}
	}()
}

func (probe *DockerProbe) Stop() {
	probe.running.Store(false)
	close(probe.quit)
	probe.wg.Wait()
}

func NewDockerProbe(g *graph.Graph, n *graph.Node, dockerURL string) *DockerProbe {
	return &DockerProbe{
		NetNSProbe: *NewNetNSProbe(g, n),
		url:        dockerURL,
	}
}

func NewDockerProbeFromConfig(g *graph.Graph, n *graph.Node) *DockerProbe {
	dockerURL := config.GetConfig().GetString("docker.url")
	return NewDockerProbe(g, n, dockerURL)
}
