// +build linux

/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package docker

import (
	"context"
	"errors"
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
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	tp "github.com/skydive-project/skydive/topology/probes"
	ns "github.com/skydive-project/skydive/topology/probes/netns"
	sversion "github.com/skydive-project/skydive/version"
)

// ClientAPIVersion Client API version used
const ClientAPIVersion = "1.18"

type containerInfo struct {
	Pid  int
	Node *graph.Node
}

// ProbeHandler describes a Docker topology graph that enhance the graph
type ProbeHandler struct {
	common.RWMutex
	*ns.ProbeHandler
	url          string
	client       *client.Client
	cancel       context.CancelFunc
	state        int64
	connected    atomic.Value
	wg           sync.WaitGroup
	hostNs       netns.NsHandle
	containerMap map[string]containerInfo
}

func (p *ProbeHandler) containerNamespace(pid int) string {
	return fmt.Sprintf("/proc/%d/ns/net", pid)
}

func (p *ProbeHandler) registerContainer(id string) {
	p.Lock()
	defer p.Unlock()

	if _, ok := p.containerMap[id]; ok {
		return
	}
	info, err := p.client.ContainerInspect(context.Background(), id)
	if err != nil {
		p.Ctx.Logger.Errorf("Failed to inspect Docker container %s: %s", id, err)
		return
	}

	nsHandle, err := netns.GetFromPid(info.State.Pid)
	if err != nil {
		return
	}
	defer nsHandle.Close()

	namespace := p.containerNamespace(info.State.Pid)
	p.Ctx.Logger.Debugf("Register docker container %s and PID %d", info.ID, info.State.Pid)

	var n *graph.Node
	if p.hostNs.Equal(nsHandle) {
		// The container is in net=host mode
		n = p.Ctx.RootNode
	} else {
		if n, err = p.Register(namespace, info.Name[1:]); err != nil {
			p.Ctx.Logger.Debugf("Failed to register probe for namespace %s: %s", namespace, err)
			return
		}

		p.Ctx.Graph.Lock()
		if err := p.Ctx.Graph.AddMetadata(n, "Manager", "docker"); err != nil {
			p.Ctx.Logger.Error(err)
		}
		p.Ctx.Graph.Unlock()
	}

	pid := int64(info.State.Pid)

	dockerMetadata := Metadata{
		ContainerID:   info.ID,
		ContainerName: info.Name[1:],
	}

	if len(info.Config.Labels) != 0 {
		dockerMetadata.Labels = graph.Metadata(common.NormalizeValue(info.Config.Labels).(map[string]interface{}))
	}

	p.Ctx.Graph.Lock()
	defer p.Ctx.Graph.Unlock()

	containerNode := p.Ctx.Graph.LookupFirstNode(graph.Metadata{"InitProcessPID": pid})
	if containerNode != nil {
		if err := p.Ctx.Graph.AddMetadata(containerNode, "Docker", dockerMetadata); err != nil {
			p.Ctx.Logger.Error(err)
		}
	} else {
		metadata := graph.Metadata{
			"Type":           "container",
			"Name":           info.Name[1:],
			"Manager":        "docker",
			"InitProcessPID": pid,
			"Docker":         dockerMetadata,
		}

		if containerNode, err = p.Ctx.Graph.NewNode(graph.GenID(), metadata); err != nil {
			p.Ctx.Logger.Error(err)
			return
		}
	}
	topology.AddOwnershipLink(p.Ctx.Graph, n, containerNode, nil)

	p.containerMap[info.ID] = containerInfo{
		Pid:  info.State.Pid,
		Node: containerNode,
	}
}

func (p *ProbeHandler) unregisterContainer(id string) {
	p.Lock()
	defer p.Unlock()

	infos, ok := p.containerMap[id]
	if !ok {
		return
	}

	p.Ctx.Graph.Lock()
	if err := p.Ctx.Graph.DelNode(infos.Node); err != nil {
		p.Ctx.Graph.Unlock()
		p.Ctx.Logger.Error(err)
		return
	}
	p.Ctx.Graph.Unlock()

	namespace := p.containerNamespace(infos.Pid)
	p.Ctx.Logger.Debugf("Stop listening for namespace %s with PID %d", namespace, infos.Pid)
	p.Unregister(namespace)

	delete(p.containerMap, id)
}

func (p *ProbeHandler) handleDockerEvent(event *events.Message) {
	if event.Status == "start" {
		p.registerContainer(event.ID)
	} else if event.Status == "die" {
		p.unregisterContainer(event.ID)
	}
}

func (p *ProbeHandler) connect() error {
	var err error

	p.Ctx.Logger.Debugf("Connecting to Docker daemon: %s", p.url)
	defaultHeaders := map[string]string{"User-Agent": fmt.Sprintf("skydive-agent-%s", sversion.Version)}
	p.client, err = client.NewClient(p.url, ClientAPIVersion, nil, defaultHeaders)
	if err != nil {
		p.Ctx.Logger.Errorf("Failed to create client to Docker daemon: %s", err)
		return err
	}
	defer p.client.Close()

	if _, err := p.client.ServerVersion(context.Background()); err != nil {
		p.Ctx.Logger.Errorf("Failed to connect to Docker daemon: %s", err)
		return err
	}

	if p.hostNs, err = netns.Get(); err != nil {
		return err
	}
	defer p.hostNs.Close()

	for id := range p.containerMap {
		p.unregisterContainer(id)
	}

	eventsFilter := filters.NewArgs()
	eventsFilter.Add("event", "start")
	eventsFilter.Add("event", "die")

	ctx, cancel := context.WithCancel(context.Background())
	eventChan, errChan := p.client.Events(ctx, types.EventsOptions{Filters: eventsFilter})

	p.cancel = cancel
	p.wg.Add(2)

	p.connected.Store(true)
	defer p.connected.Store(false)

	go func() {
		defer p.wg.Done()

		containers, err := p.client.ContainerList(ctx, types.ContainerListOptions{})
		if err != nil {
			p.Ctx.Logger.Errorf("Failed to list containers: %s", err)
			return
		}

		for _, c := range containers {
			if atomic.LoadInt64(&p.state) != common.RunningState {
				break
			}
			p.registerContainer(c.ID)
		}
	}()

	defer p.wg.Done()

	for {
		select {
		case err := <-errChan:
			if atomic.LoadInt64(&p.state) != common.StoppingState {
				err = fmt.Errorf("Got error while waiting for Docker event: %s", err)
			}
			return err
		case event := <-eventChan:
			p.handleDockerEvent(&event)
		}
	}
}

// Start the probe
func (p *ProbeHandler) Start() {
	if !atomic.CompareAndSwapInt64(&p.state, common.StoppedState, common.RunningState) {
		return
	}

	go func() {
		for {
			state := atomic.LoadInt64(&p.state)
			if state == common.StoppingState || state == common.StoppedState {
				break
			}

			if p.connect() != nil {
				time.Sleep(1 * time.Second)
			}

			p.wg.Wait()
		}
	}()
}

// Stop the probe
func (p *ProbeHandler) Stop() {
	if !atomic.CompareAndSwapInt64(&p.state, common.RunningState, common.StoppingState) {
		return
	}

	if p.connected.Load() == true {
		p.cancel()
		p.wg.Wait()
	}

	atomic.StoreInt64(&p.state, common.StoppedState)
}

// Init initializes a new topology Docker probe
func (p *ProbeHandler) Init(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	nsHandler := bundle.GetHandler("netns")
	if nsHandler == nil {
		return nil, errors.New("unable to find the netns handler")
	}

	dockerURL := ctx.Config.GetString("agent.topology.docker.url")
	netnsRunPath := ctx.Config.GetString("agent.topology.docker.netns.run_path")

	p.ProbeHandler = nsHandler.(*ns.ProbeHandler)
	p.url = dockerURL
	p.containerMap = make(map[string]containerInfo)
	p.state = common.StoppedState

	if netnsRunPath != "" {
		p.Exclude(netnsRunPath + "/default")
		p.Watch(netnsRunPath)
	}

	return p, nil
}
