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
	ID        string            `json:"-"`
	Metadatas map[string]string `json:",omitempty	"`
	port      *Port             `json:"-"`
}

type Port struct {
	sync.RWMutex
	ID         string                `json:"-"`
	Metadatas  map[string]string     `json:",omitempty"`
	Interfaces map[string]*Interface `json:",omitempty"`
	container  *Container            `json:"-"`
	links      map[string]*Link      `json:"-"`
}

type Link struct {
	sync.RWMutex
	ID        string `json:"-"`
	Left      *Port
	Right     *Port
	Metadatas map[string]string `json:",omitempty"`
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
	interfaces map[string]*Interface `json:"-"`
	ports      map[string]*Port      `json:"-"`
	links      map[string]*Link      `json:"-"`
}

func (t *Topology) Log() {
	j, _ := json.Marshal(t)
	logging.GetLogger().Debug("Topology: %s", string(j))
}

func (topo *Topology) NewInterface(i string, p *Port) *Interface {
	topo.Lock()
	defer topo.Unlock()

	intf := &Interface{
		ID:        i,
		Metadatas: make(map[string]string),
	}
	topo.interfaces[i] = intf

	if p != nil {
		p.Interfaces[i] = intf
		intf.port = p
		topo.Log()
	}

	return intf
}

func (topo *Topology) DelInterface(i string) {
	topo.Lock()
	defer topo.Unlock()

	intf, ok := topo.interfaces[i]
	if !ok {
		return
	}

	if intf.port != nil {
		delete(intf.port.Interfaces, i)
		topo.Log()
	}

	delete(topo.interfaces, i)
}

func (topo *Topology) GetInterface(i string) *Interface {
	topo.Lock()
	defer topo.Unlock()

	if intf, ok := topo.interfaces[i]; ok {
		return intf
	}
	return nil
}

func (port *Port) AddInterface(intf *Interface) {
	port.Lock()
	defer port.Unlock()

	intf.Lock()
	defer intf.Unlock()

	port.Interfaces[intf.ID] = intf
	intf.port = port
}

func (port *Port) GetContainer() *Container {
	return port.container
}

func (topo *Topology) GetPort(i string) *Port {
	topo.Lock()
	defer topo.Unlock()

	if port, ok := topo.ports[i]; ok {
		return port
	}
	return nil
}

func (topo *Topology) DelPort(i string) {
	topo.Lock()
	defer topo.Unlock()

	port, ok := topo.ports[i]
	if !ok {
		return
	}

	for id, link := range port.links {
		delete(link.Left.links, id)
		delete(link.Right.links, id)
	}

	for _, intf := range port.Interfaces {
		intf.port = nil
	}

	if port.container != nil {
		delete(port.container.Ports, i)
		topo.Log()
	}

	delete(topo.ports, i)
}

func (topo *Topology) NewPort(i string, c *Container) *Port {
	topo.Lock()
	defer topo.Unlock()

	port := &Port{
		ID:         i,
		Metadatas:  make(map[string]string),
		Interfaces: make(map[string]*Interface),
		links:      make(map[string]*Link),
	}
	topo.ports[i] = port

	if c != nil {
		c.Ports[i] = port
		port.container = c
		topo.Log()
	}

	return port
}

func (topo *Topology) DelContainer(i string) {
	topo.Lock()
	defer topo.Unlock()

	container, ok := topo.Containers[i]
	if !ok {
		return
	}

	for _, port := range container.Ports {
		port.container = nil
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

func (container *Container) AddPort(port *Port) {
	container.Lock()
	defer container.Unlock()

	port.Lock()
	defer port.Unlock()

	container.Ports[port.ID] = port
	port.container = container
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
		interfaces: make(map[string]*Interface),
		ports:      make(map[string]*Port),
		links:      make(map[string]*Link),
	}
}
