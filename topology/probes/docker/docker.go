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
	"fmt"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/safchain/insanelock"
	"github.com/vishvananda/netns"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes"
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
	insanelock.RWMutex
	Ctx          tp.Context
	nsProbe      *ns.ProbeHandler
	url          string
	client       *client.Client
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
		if n, err = p.nsProbe.Register(namespace, info.Name[1:]); err != nil {
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

	dockerMetadata := topology.ContainerMetadata{
		ID:             info.ID,
		Image:          info.Config.Image,
		ImageID:        info.Image,
		Runtime:        "docker",
		Status:         info.State.Status,
		InitProcessPID: pid,
	}

	if len(info.Config.Labels) != 0 {
		dockerMetadata.Labels = graph.Metadata(graph.NormalizeValue(info.Config.Labels).(map[string]interface{}))
	}

	p.Ctx.Graph.Lock()
	defer p.Ctx.Graph.Unlock()

	containerNode := p.Ctx.Graph.LookupFirstNode(graph.Metadata{"Container.InitProcessPID": pid})
	if containerNode != nil {
		if err := p.Ctx.Graph.AddMetadata(containerNode, "Container", dockerMetadata); err != nil {
			p.Ctx.Logger.Error(err)
		}
	} else {
		metadata := graph.Metadata{
			"Type":      "container",
			"Name":      info.Name[1:],
			"Manager":   "docker",
			"Container": dockerMetadata,
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
	p.nsProbe.Unregister(namespace)

	delete(p.containerMap, id)
}

func (p *ProbeHandler) handleDockerEvent(event *events.Message) {
	if event.Status == "start" {
		p.registerContainer(event.ID)
	} else if event.Status == "die" {
		p.unregisterContainer(event.ID)
	}
}

// Do connects to the Docker daemon, registers the existing containers and
// start listening for Docker events
func (p *ProbeHandler) Do(ctx context.Context, wg *sync.WaitGroup) error {
	var err error

	p.Ctx.Logger.Debugf("Connecting to Docker daemon: %s", p.url)
	defaultHeaders := map[string]string{"User-Agent": fmt.Sprintf("skydive-agent-%s", sversion.Version)}
	p.client, err = client.NewClient(p.url, ClientAPIVersion, nil, defaultHeaders)
	if err != nil {
		return errors.Wrapf(err, "failed to create client to Docker daemon")
	}
	defer p.client.Close()

	version, err := p.client.ServerVersion(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to Docker daemon")
	}

	p.Ctx.Logger.Infof("Connected to Docker %s", version.Version)

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

	eventChan, errChan := p.client.Events(ctx, types.EventsOptions{Filters: eventsFilter})

	wg.Add(2)

	go func() {
		defer wg.Done()

		containers, err := p.client.ContainerList(ctx, types.ContainerListOptions{})
		if err != nil {
			p.Ctx.Logger.Errorf("Failed to list containers: %s", err)
			return
		}

		for _, c := range containers {
			select {
			case <-ctx.Done():
			default:
				p.registerContainer(c.ID)
			}
		}
	}()

	go func() {
		defer wg.Done()

		for {
			select {
			case err := <-errChan:
				switch {
				case err == nil || err == context.Canceled:
					return
				case err.Error() == "unexpected EOF":
					p.Ctx.Logger.Error("lost connection to Docker")
				default:
					p.Ctx.Logger.Errorf("got error while waiting for Docker event: %s", err)
				}
			case event := <-eventChan:
				p.handleDockerEvent(&event)
			}
		}
	}()

	return nil
}

// NewProbe returns a new topology Docker probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	nsHandler := bundle.GetHandler("netns")
	if nsHandler == nil {
		return nil, errors.New("unable to find the netns handler")
	}

	dockerURL := ctx.Config.GetString("agent.topology.docker.url")
	netnsRunPath := ctx.Config.GetString("agent.topology.docker.netns.run_path")

	p := &ProbeHandler{
		nsProbe:      nsHandler.(*ns.ProbeHandler),
		url:          dockerURL,
		containerMap: make(map[string]containerInfo),
		Ctx:          ctx,
	}

	if netnsRunPath != "" {
		p.nsProbe.Exclude(netnsRunPath + "/default")
		p.nsProbe.Watch(netnsRunPath)
	}

	return probes.NewProbeWrapper(p), nil
}
