// +build linux,lxd

/*
 * Copyright (C) 2018 Iain Grant
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

package lxd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/safchain/insanelock"
	"github.com/vishvananda/netns"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes"
	tp "github.com/skydive-project/skydive/topology/probes"
	ns "github.com/skydive-project/skydive/topology/probes/netns"
)

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

// ProbeHandler describes a LXD topology graph that enhance the graph
type ProbeHandler struct {
	insanelock.RWMutex
	Ctx          tp.Context
	nsProbe      *ns.ProbeHandler
	hostNs       netns.NsHandle
	containerMap map[string]containerInfo
	client       lxd.ContainerServer
}

func (p *ProbeHandler) containerNamespace(pid int) string {
	return fmt.Sprintf("/proc/%d/ns/net", pid)
}

func (p *ProbeHandler) registerContainer(id string) {
	p.Ctx.Logger.Debugf("Registering container %s", id)

	p.Lock()
	defer p.Unlock()

	if _, ok := p.containerMap[id]; ok {
		return
	}

	container, _, err := p.client.GetContainer(id)
	if err != nil {
		p.Ctx.Logger.Errorf("Failed to retrieve container %s", id)
		return
	}

	// state.Network[].HostName == host side interface
	// state.Pid for lookup of network namespace
	state, _, _ := p.client.GetContainerState(id)

	if state.Status != "Running" {
		p.Ctx.Logger.Errorf("Container %s is not running", id)
		return
	}

	nsHandle, err := netns.GetFromPid(int(state.Pid))
	if err != nil {
		return
	}
	defer nsHandle.Close()

	namespace := p.containerNamespace(int(state.Pid))

	var n *graph.Node
	if p.hostNs.Equal(nsHandle) {
		n = p.Ctx.RootNode
	} else {
		if n, err = p.nsProbe.Register(namespace, id); err == nil {
			p.Ctx.Graph.Lock()
			p.Ctx.Graph.AddMetadata(n, "Manager", "lxd")
			p.Ctx.Graph.Unlock()
		} else {
			p.Ctx.Logger.Errorf("Error registering probe: %s", err)
		}
	}

	devices := graph.Metadata{}
	for k, v := range container.Devices {
		devices[k] = v
	}

	metadata := Metadata{
		Architecture: container.Architecture,
		CreatedAt:    container.CreatedAt.String(),
		Description:  container.Description,
		Devices:      devices,
		Ephemeral:    strconv.FormatBool(container.Ephemeral),
		Profiles:     container.Profiles,
		Restore:      container.Restore,
		Stateful:     strconv.FormatBool(container.Stateful),
		Status:       container.Status,
	}

	if len(container.Config) != 0 {
		metadata.Config = graph.Metadata(graph.NormalizeValue(container.Config).(map[string]interface{}))
	}

	p.Ctx.Graph.Lock()

	containerNode, err := p.Ctx.Graph.NewNode(graph.GenID(), graph.Metadata{
		"Type": "container",
		"Name": id,
		"LXD":  metadata,
	})
	if err != nil {
		p.Ctx.Logger.Error(err)
		return
	}

	topology.AddOwnershipLink(p.Ctx.Graph, n, containerNode, nil)
	p.Ctx.Graph.Unlock()

	p.containerMap[id] = containerInfo{
		Pid:  int(state.Pid),
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
		p.Ctx.Logger.Error(err)
	}
	p.Ctx.Graph.Unlock()

	namespace := p.containerNamespace(infos.Pid)
	p.nsProbe.Unregister(namespace)

	delete(p.containerMap, id)
}

func (p *ProbeHandler) Do(ctx context.Context, wg *sync.WaitGroup) (err error) {
	if p.hostNs, err = netns.Get(); err != nil {
		return err
	}

	p.Ctx.Logger.Debugf("Connecting to LXD")
	client, err := lxd.ConnectLXDUnix("", nil)
	if err != nil {
		return err
	}
	p.client = client

	events, err := client.GetEvents()
	if err != nil {
		return err
	}

	target, err := events.AddHandler(nil, func(event api.Event) {
		if event.Type == "logging" {
			logEntry := api.EventLogging{}
			if err := json.Unmarshal(event.Metadata, &logEntry); err != nil || logEntry.Context == nil {
				return
			}

			if logEntry.Message == "Started container" && logEntry.Context["action"] == "start" {
				p.registerContainer(logEntry.Context["name"])
			} else if logEntry.Message == "Deleted container" {
				p.unregisterContainer(logEntry.Context["name"])
			}
		}
	})

	if err != nil {
		return err
	}

	wg.Add(3)
	go func() {
		defer wg.Done()

		p.Ctx.Logger.Debugf("Listing LXD containers")
		containers, err := p.client.GetContainers()
		if err != nil {
			return
		}

		for _, n := range containers {
			p.registerContainer(n.Name)
		}
	}()

	disconnected := make(chan error, 1)
	go func() {
		defer wg.Done()
		defer events.RemoveHandler(target)
		defer p.hostNs.Close()

		disconnected <- events.Wait()
	}()

	go func() {
		defer wg.Done()

		select {
		case <-ctx.Done():
			events.Disconnect()
		case <-disconnected:
		}
	}()

	return err
}

// NewProbe returns a new topology LXD probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	nsHandler := bundle.GetHandler("netns")
	if nsHandler == nil {
		return nil, errors.New("unable to find the netns handler")
	}

	p := &ProbeHandler{
		nsProbe:      nsHandler.(*ns.ProbeHandler),
		containerMap: make(map[string]containerInfo),
		Ctx:          ctx,
	}

	return probes.NewProbeWrapper(p), nil
}

// Register registers graph metadata decoders
func Register() {
	graph.NodeMetadataDecoders["LXD"] = MetadataDecoder
}
