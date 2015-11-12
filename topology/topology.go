/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package topology

import (
	"encoding/json"
	"sync"

	"github.com/redhat-cip/skydive/logging"
)

const (
	Root        = "root"
	NetNs       = "netns"
	LinuxBridge = "linuxbridge"
	OvsBridge   = "ovsbridge"
)

type Interface struct {
	sync.RWMutex
	ID        string `json:"-"`
	Type      string
	Mac       string            `json:",omitempty"`
	IfIndex   uint32            `json:"-"`
	Metadatas map[string]string `json:",omitempty"`
	Port      *Port             `json:"-"`
	Peer      string            `json:",omitempty"`
}

type Port struct {
	sync.RWMutex
	ID         string                `json:"-"`
	Metadatas  map[string]string     `json:",omitempty"`
	Interfaces map[string]*Interface `json:",omitempty"`
	Container  *Container            `json:"-"`
}

type Container struct {
	sync.RWMutex
	ID       string `json:"-"`
	Type     string
	Ports    map[string]*Port
	Topology *Topology `json:"-"`
}

type Topology struct {
	sync.RWMutex
	Containers map[string]*Container
}

func (t *Topology) Log() {
	j, _ := json.Marshal(t)
	logging.GetLogger().Debug("Topology: %s", string(j))
}

func (intf *Interface) SetPeer(i *Interface) {
	intf.Lock()
	defer intf.Unlock()
	i.Lock()
	defer i.Unlock()

	intf.Peer = i.Port.Container.ID + "/" + i.Port.ID + "/" + i.ID
	i.Peer = intf.Port.Container.ID + "/" + intf.Port.ID + "/" + intf.ID
}

func (intf *Interface) SetMac(mac string) {
	intf.Lock()
	defer intf.Unlock()

	intf.Mac = mac
}

func (intf *Interface) Del() {
	intf.Port.DelInterface(intf.ID)
}

func (p *Port) Del() {
	p.Container.DelPort(p.ID)
}

func (p *Port) newInterface(i string, index uint32) *Interface {
	p.Lock()
	defer p.Unlock()

	intf := &Interface{
		ID:        i,
		Metadatas: make(map[string]string),
		IfIndex:   index,
		Port:      p,
	}
	p.Interfaces[i] = intf

	if p.Container != nil {
		p.Container.Topology.Log()
	}

	return intf
}

func (p *Port) NewInterface(i string) *Interface {
	return p.newInterface(i, 0)
}

func (p *Port) NewInterfaceWithIndex(i string, index uint32) *Interface {
	return p.newInterface(i, index)
}

func (p *Port) AddInterface(intf *Interface) {
	p.Lock()
	defer p.Unlock()

	p.Interfaces[intf.ID] = intf

	if p.Container != nil {
		p.Container.Topology.Log()
	}
}

func (p *Port) DelInterface(i string) {
	p.Lock()
	defer p.Unlock()

	if _, ok := p.Interfaces[i]; !ok {
		return
	}

	delete(p.Interfaces, i)

	p.Container.Topology.Log()
}

func (p *Port) GetInterface(i string) *Interface {
	p.Lock()
	defer p.Unlock()

	if intf, ok := p.Interfaces[i]; ok {
		return intf
	}
	return nil
}

func (c *Container) GetPort(i string) *Port {
	c.Lock()
	defer c.Unlock()

	if port, ok := c.Ports[i]; ok {
		return port
	}
	return nil
}

func (c *Container) DelPort(i string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.Ports[i]; !ok {
		return
	}
	delete(c.Ports, i)

	c.Topology.Log()
}

func (c *Container) NewPort(i string) *Port {
	c.Lock()
	defer c.Unlock()

	port := &Port{
		ID:         i,
		Metadatas:  make(map[string]string),
		Interfaces: make(map[string]*Interface),
		Container:  c,
	}
	c.Ports[i] = port

	c.Topology.Log()

	return port
}

func (c *Container) AddPort(p *Port) {
	c.Lock()
	defer c.Unlock()

	c.Ports[p.ID] = p

	c.Topology.Log()
}

func (topo *Topology) LookupInterfaceByIndex(index uint32) *Interface {
	topo.Lock()
	defer topo.Unlock()

	for _, container := range topo.Containers {
		container.Lock()
		defer container.Unlock()
		for _, port := range container.Ports {
			port.Lock()
			defer port.Unlock()
			for _, intf := range port.Interfaces {
				intf.Lock()
				defer intf.Unlock()
				if intf.IfIndex == index {
					return intf
				}
			}
		}
	}

	return nil
}

func (topo *Topology) LookupInterfaceByMac(mac string) *Interface {
	topo.Lock()
	defer topo.Unlock()

	for _, container := range topo.Containers {
		container.Lock()
		defer container.Unlock()
		for _, port := range container.Ports {
			port.Lock()
			defer port.Unlock()
			for _, intf := range port.Interfaces {
				intf.Lock()
				defer intf.Unlock()
				if intf.Mac == mac {
					return intf
				}
			}
		}
	}

	return nil
}

func (topo *Topology) NewPort(i string) *Port {
	port := &Port{
		ID:         i,
		Metadatas:  make(map[string]string),
		Interfaces: make(map[string]*Interface),
	}

	return port
}

func (topo *Topology) NewInterface(i string) *Interface {
	intf := &Interface{
		ID:        i,
		Metadatas: make(map[string]string),
	}

	return intf
}

func (topo *Topology) DelContainer(i string) {
	topo.Lock()
	defer topo.Unlock()

	if _, ok := topo.Containers[i]; !ok {
		return
	}

	delete(topo.Containers, i)

	topo.Log()
}

func (topo *Topology) GetContainer(i string) *Container {
	topo.Lock()
	defer topo.Unlock()

	c, ok := topo.Containers[i]
	if !ok {
		return nil
	}
	return c
}

func (topo *Topology) NewContainer(i string, t string) *Container {
	topo.Lock()
	defer topo.Unlock()

	container := &Container{
		ID:       i,
		Type:     t,
		Ports:    make(map[string]*Port),
		Topology: topo,
	}
	topo.Containers[i] = container

	topo.Log()

	return container
}

func NewTopology() *Topology {
	return &Topology{
		Containers: make(map[string]*Container),
	}
}
