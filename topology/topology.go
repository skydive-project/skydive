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

	"github.com/nu7hatch/gouuid"

	"github.com/redhat-cip/skydive/logging"
)

const (
	NetNSScope = 1
	OvsScope   = 2
)

type LookupFunction func(*Interface) bool

type Interface struct {
	UUID       string
	ID         string                 `json:"-"`
	Type       string                 `json:",omitempty"`
	Mac        string                 `json:",omitempty"`
	MTU        uint32                 `json:",omitempty"`
	IfIndex    uint32                 `json:",omitempty"`
	Metadatas  map[string]interface{} `json:",omitempty"`
	Port       *Port                  `json:"-"`
	NetNs      *NetNs                 `json:"-"`
	Peer       *Interface             `json:",omitempty"`
	Interfaces map[string]*Interface  `json:",omitempty"`
	Parent     *Interface             `json:",omitempty"`
	Topology   *Topology              `json:"-"`
}

type Port struct {
	ID         string                 `json:"-"`
	Interfaces map[string]*Interface  `json:",omitempty"`
	Metadatas  map[string]interface{} `json:",omitempty"`
	OvsBridge  *OvsBridge             `json:"-"`
	Topology   *Topology              `json:"-"`
}

type OvsBridge struct {
	ID       string           `json:"-"`
	Ports    map[string]*Port `json:",omitempty"`
	Topology *Topology        `json:"-"`
}

type NetNs struct {
	ID         string                `json:"-"`
	Interfaces map[string]*Interface `json:",omitempty"`
	Topology   *Topology             `json:"-"`
}

type JTopology Topology

type Topology struct {
	sync.RWMutex
	OvsBridges map[string]*OvsBridge
	NetNss     map[string]*NetNs
}

func (intf *Interface) MarshalJSON() ([]byte, error) {
	var peer string
	if intf.Peer != nil {
		peer = intf.Peer.UUID
	}

	return json.Marshal(&struct {
		UUID       string
		Type       string                 `json:",omitempty"`
		Mac        string                 `json:",omitempty"`
		MTU        uint32                 `json:",omitempty"`
		Metadatas  map[string]interface{} `json:",omitempty"`
		Interfaces map[string]*Interface  `json:",omitempty"`
		Peer       string                 `json:",omitempty"`
		IfIndex    uint32                 `json:",omitempty"`
	}{
		UUID:       intf.UUID,
		Type:       intf.Type,
		Mac:        intf.Mac,
		MTU:        intf.MTU,
		Metadatas:  intf.Metadatas,
		Interfaces: intf.Interfaces,
		Peer:       peer,
		IfIndex:    intf.IfIndex,
	})
}

// SetPeer set the peer interface with the given interface
func (intf *Interface) SetPeer(i *Interface) {
	intf.Topology.Lock()
	defer intf.Topology.Unlock()

	intf.Peer = i
	i.Peer = intf

	intf.Topology.Log()
}

// SetMac set the mac address
func (intf *Interface) SetMac(mac string) {
	intf.Topology.Lock()
	defer intf.Topology.Unlock()

	intf.Mac = mac

	intf.Topology.Log()
}

// SetMetadata attach metadata to the interface
func (intf *Interface) SetMetadata(key string, value interface{}) {
	intf.Topology.Lock()
	defer intf.Topology.Unlock()

	intf.Metadatas[key] = value
}

// SetType set the type of the interface, could be device, openvswitch, veth, etc.
func (intf *Interface) SetType(t string) {
	intf.Topology.Lock()
	defer intf.Topology.Unlock()

	intf.Type = t

	intf.Topology.Log()
}

// SetIndex specifies the index of the interface, doesn't make sense for ovs internals
func (intf *Interface) SetIndex(i uint32) {
	intf.Topology.Lock()
	defer intf.Topology.Unlock()

	intf.IfIndex = i
}

// Del deletes the interface, remove the interface for the port or any other container
func (intf *Interface) Del() {
	intf.Topology.Lock()
	defer intf.Topology.Unlock()

	if intf.Port != nil {
		intf.Port.DelInterface(intf.ID)
	}
}

// AddInterface add a previously created interface
func (intf *Interface) AddInterface(i *Interface) {
	intf.Topology.Lock()
	defer intf.Topology.Unlock()

	intf.Interfaces[i.ID] = i
	i.Parent = intf

	intf.Topology.Log()
}

// DelInterface removes the interface with the given id from the port
func (intf *Interface) DelInterface(i string) {
	intf.Topology.Lock()
	defer intf.Topology.Unlock()

	delete(intf.Interfaces, i)

	intf.Topology.Log()
}

// NewInterface instantiate a new interface with a given index
func (intf *Interface) NewInterface(i string, index uint32) *Interface {
	intf.Topology.Lock()
	defer intf.Topology.Unlock()

	u, _ := uuid.NewV4()

	nIntf := &Interface{
		ID:         i,
		UUID:       u.String(),
		Metadatas:  make(map[string]interface{}),
		Interfaces: make(map[string]*Interface),
		IfIndex:    index,
		Parent:     intf,
		NetNs:      intf.NetNs,
		Topology:   intf.Topology,
	}
	intf.Interfaces[i] = nIntf

	intf.Topology.Log()

	return nIntf
}

// NewInterface instantiate a new interface with a given index
func (n *NetNs) NewInterface(i string, index uint32) *Interface {
	n.Topology.Lock()
	defer n.Topology.Unlock()

	u, _ := uuid.NewV4()

	intf := &Interface{
		ID:         i,
		UUID:       u.String(),
		Metadatas:  make(map[string]interface{}),
		Interfaces: make(map[string]*Interface),
		IfIndex:    index,
		NetNs:      n,
		Topology:   n.Topology,
	}
	n.Interfaces[i] = intf

	n.Topology.Log()

	return intf
}

// AddInterface add a previously created interface
func (n *NetNs) AddInterface(intf *Interface) {
	n.Topology.Lock()
	defer n.Topology.Unlock()

	n.Interfaces[intf.ID] = intf
	intf.NetNs = n

	n.Topology.Log()
}

// DelInterface removes the interface with the given id from the port
func (n *NetNs) DelInterface(i string) {
	n.Topology.Lock()
	defer n.Topology.Unlock()

	delete(n.Interfaces, i)

	n.Topology.Log()
}

// GetInterface returns the interface with the given ID from the port
func (n *NetNs) GetInterface(i string) *Interface {
	n.Topology.RLock()
	defer n.Topology.RUnlock()

	if intf, ok := n.Interfaces[i]; ok {
		return intf
	}
	return nil
}

// SetMetadata attach metadata to the port
func (p *Port) SetMetadata(key string, value interface{}) {
	p.Topology.Lock()
	defer p.Topology.Unlock()

	p.Metadatas[key] = value
}

// Del removes the port from the ovs bridge containing it
func (p *Port) Del() {
	p.Topology.Lock()
	defer p.Topology.Unlock()

	if p.OvsBridge != nil {
		p.OvsBridge.DelPort(p.ID)
	}
}

// NewInterface instantiate a new interface with a given index
func (p *Port) NewInterface(i string, index uint32) *Interface {
	p.Topology.Lock()
	defer p.Topology.Unlock()

	u, _ := uuid.NewV4()

	intf := &Interface{
		ID:         i,
		UUID:       u.String(),
		Metadatas:  make(map[string]interface{}),
		Interfaces: make(map[string]*Interface),
		IfIndex:    index,
		Port:       p,
		Topology:   p.Topology,
	}
	p.Interfaces[i] = intf

	p.Topology.Log()

	return intf
}

// AddInterface add a previously created interface
func (p *Port) AddInterface(intf *Interface) {
	p.Topology.Lock()
	defer p.Topology.Unlock()

	p.Interfaces[intf.ID] = intf
	intf.Port = p

	p.Topology.Log()
}

// DelInterface removes the interface with the given id from the port
func (p *Port) DelInterface(i string) {
	p.Topology.Lock()
	defer p.Topology.Unlock()

	delete(p.Interfaces, i)

	p.Topology.Log()
}

// GetInterface returns the interface with the given ID from the port
func (p *Port) GetInterface(i string) *Interface {
	p.Topology.RLock()
	defer p.Topology.RUnlock()

	if intf, ok := p.Interfaces[i]; ok {
		return intf
	}
	return nil
}

// GetPort returns the port with the given port ID from the container
func (o *OvsBridge) GetPort(i string) *Port {
	o.Topology.RLock()
	defer o.Topology.RUnlock()

	if port, ok := o.Ports[i]; ok {
		return port
	}
	return nil
}

// DelPort removes the port with the given id from the ovs bridge
func (o *OvsBridge) DelPort(i string) {
	o.Topology.Lock()
	defer o.Topology.Unlock()

	delete(o.Ports, i)

	o.Topology.Log()
}

// NewPort intentiates a new port and add it to the ovs bridge
func (o *OvsBridge) NewPort(i string) *Port {
	o.Topology.Lock()
	defer o.Topology.Unlock()

	port := &Port{
		ID:         i,
		Interfaces: make(map[string]*Interface),
		Metadatas:  make(map[string]interface{}),
		OvsBridge:  o,
		Topology:   o.Topology,
	}
	o.Ports[i] = port

	o.Topology.Log()

	return port
}

// AddPort add a previously created port to the ovs bridge
func (o *OvsBridge) AddPort(p *Port) {
	o.Topology.Lock()
	defer o.Topology.Unlock()

	o.Ports[p.ID] = p
	p.OvsBridge = o

	o.Topology.Log()
}

func LookupByType(name string, t string) LookupFunction {
	return func(intf *Interface) bool {
		if len(name) > 0 && intf.ID != name {
			return false
		}
		if intf.Type == t {
			return true
		}
		return false
	}
}

func LookupByMac(name string, mac string) LookupFunction {
	return func(intf *Interface) bool {
		if len(name) > 0 && intf.ID != name {
			return false
		}
		if intf.Mac == mac {
			return true
		}
		return false
	}
}

func LookupByID(i string) LookupFunction {
	return func(intf *Interface) bool {
		if intf.ID == i {
			return true
		}
		return false
	}
}

func LookupByIfIndex(i uint32) LookupFunction {
	return func(intf *Interface) bool {
		if intf.IfIndex == i {
			return true
		}
		return false
	}
}

func (topo *Topology) LookupInterface(f LookupFunction, scope int) *Interface {
	topo.RLock()
	defer topo.RUnlock()

	if (scope & NetNSScope) > 0 {
		for _, netns := range topo.NetNss {
			for _, intf := range netns.Interfaces {
				if f(intf) {
					return intf
				}
			}
		}
	}

	if (scope & OvsScope) > 0 {
		for _, bridge := range topo.OvsBridges {
			for _, port := range bridge.Ports {
				for _, intf := range port.Interfaces {
					if f(intf) {
						return intf
					}
				}
			}
		}
	}

	return nil
}

func (topo *Topology) NewPort(i string) *Port {
	port := &Port{
		ID:         i,
		Interfaces: make(map[string]*Interface),
		Metadatas:  make(map[string]interface{}),
		Topology:   topo,
	}

	return port
}

func (topo *Topology) NewInterface(i string, index uint32) *Interface {
	u, _ := uuid.NewV4()

	intf := &Interface{
		ID:         i,
		UUID:       u.String(),
		Metadatas:  make(map[string]interface{}),
		Interfaces: make(map[string]*Interface),
		IfIndex:    index,
		Topology:   topo,
	}

	return intf
}

func (topo *Topology) DelOvsBridge(i string) {
	topo.Lock()
	defer topo.Unlock()

	delete(topo.OvsBridges, i)

	topo.Log()
}

func (topo *Topology) GetOvsBridge(i string) *OvsBridge {
	topo.RLock()
	defer topo.RUnlock()

	c, ok := topo.OvsBridges[i]
	if !ok {
		return nil
	}
	return c
}

func (topo *Topology) NewOvsBridge(i string) *OvsBridge {
	topo.Lock()
	defer topo.Unlock()

	bridge := &OvsBridge{
		ID:       i,
		Ports:    make(map[string]*Port),
		Topology: topo,
	}
	topo.OvsBridges[i] = bridge

	topo.Log()

	return bridge
}

func (topo *Topology) DelNetNs(i string) {
	topo.Lock()
	defer topo.Unlock()

	delete(topo.NetNss, i)

	topo.Log()
}

func (topo *Topology) NewNetNs(i string) *NetNs {
	topo.Lock()
	defer topo.Unlock()

	netns := &NetNs{
		ID:         i,
		Interfaces: make(map[string]*Interface),
		Topology:   topo,
	}
	topo.NetNss[i] = netns

	topo.Log()

	return netns
}

func (topo *Topology) Log() {
	j, _ := json.Marshal(JTopology(*topo))
	logging.GetLogger().Debug("Topology: %s", string(j))
}

func (topo *Topology) MarshalJSON() ([]byte, error) {
	topo.RLock()
	defer topo.RUnlock()

	return json.Marshal(JTopology(*topo))
}

func NewTopology() *Topology {
	return &Topology{
		OvsBridges: make(map[string]*OvsBridge),
		NetNss:     make(map[string]*NetNs),
	}
}
