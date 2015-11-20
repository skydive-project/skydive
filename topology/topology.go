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
	sync.RWMutex
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
}

type Port struct {
	sync.RWMutex
	ID         string                 `json:"-"`
	Interfaces map[string]*Interface  `json:",omitempty"`
	Metadatas  map[string]interface{} `json:",omitempty"`
	OvsBridge  *OvsBridge             `json:"-"`
}

type OvsBridge struct {
	sync.RWMutex
	ID       string           `json:"-"`
	Ports    map[string]*Port `json:",omitempty"`
	Topology *Topology        `json:"-"`
}

type NetNs struct {
	sync.RWMutex
	ID         string                `json:"-"`
	Interfaces map[string]*Interface `json:",omitempty"`
	Topology   *Topology             `json:"-"`
}

type Topology struct {
	sync.RWMutex
	OvsBridges map[string]*OvsBridge
	NetNss     map[string]*NetNs
}

func (topo *Topology) Log() {
	j, _ := json.Marshal(topo)
	logging.GetLogger().Debug("Topology: %s", string(j))
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
	intf.Lock()
	defer intf.Unlock()
	i.Lock()
	defer i.Unlock()

	intf.Peer = i
	i.Peer = intf

	if intf.Port != nil && intf.Port.OvsBridge != nil {
		intf.Port.OvsBridge.Topology.Log()
	} else if intf.NetNs != nil {
		intf.NetNs.Topology.Log()
	}
}

// SetMac set the mac address
func (intf *Interface) SetMac(mac string) {
	intf.Lock()
	defer intf.Unlock()

	intf.Mac = mac

	if intf.Port != nil && intf.Port.OvsBridge != nil {
		intf.Port.OvsBridge.Topology.Log()
	} else if intf.NetNs != nil {
		intf.NetNs.Topology.Log()
	}
}

// SetMetadata attach metadata to the interface
func (intf *Interface) SetMetadata(key string, value interface{}) {
	intf.Lock()
	defer intf.Unlock()

	intf.Metadatas[key] = value
}

// SetType set the type of the interface, could be device, openvswitch, veth, etc.
func (intf *Interface) SetType(t string) {
	intf.Lock()
	defer intf.Unlock()

	intf.Type = t

	if intf.Port != nil && intf.Port.OvsBridge != nil {
		intf.Port.OvsBridge.Topology.Log()
	} else if intf.NetNs != nil {
		intf.NetNs.Topology.Log()
	}
}

// SetIndex specifies the index of the interface, doesn't make sense for ovs internals
func (intf *Interface) SetIndex(i uint32) {
	intf.Lock()
	defer intf.Unlock()

	intf.IfIndex = i
}

// Del deletes the interface, remove the interface for the port or any other container
func (intf *Interface) Del() {
	if intf.Port != nil {
		intf.Port.DelInterface(intf.ID)
	}
}

// AddInterface add a previously created interface
func (intf *Interface) AddInterface(i *Interface) {
	intf.Lock()
	defer intf.Unlock()
	i.Lock()
	defer i.Unlock()

	intf.Interfaces[i.ID] = i
	i.Parent = intf

	if intf.NetNs != nil {
		intf.NetNs.Topology.Log()
	}
}

// DelInterface removes the interface with the given id from the port
func (intf *Interface) DelInterface(i string) {
	intf.Lock()
	defer intf.Unlock()

	delete(intf.Interfaces, i)

	if intf.NetNs != nil {
		intf.NetNs.Topology.Log()
	}
}

// NewInterface instantiate a new interface with a given index
func (intf *Interface) NewInterface(i string, index uint32) *Interface {
	intf.Lock()
	defer intf.Unlock()

	u, _ := uuid.NewV4()

	nIntf := &Interface{
		ID:         i,
		UUID:       u.String(),
		Metadatas:  make(map[string]interface{}),
		Interfaces: make(map[string]*Interface),
		IfIndex:    index,
		Parent:     intf,
		NetNs:      intf.NetNs,
	}
	intf.Interfaces[i] = nIntf

	if intf.NetNs != nil {
		intf.NetNs.Topology.Log()
	}

	return nIntf
}

// NewInterface instantiate a new interface with a given index
func (n *NetNs) NewInterface(i string, index uint32) *Interface {
	n.Lock()
	defer n.Unlock()

	u, _ := uuid.NewV4()

	intf := &Interface{
		ID:         i,
		UUID:       u.String(),
		Metadatas:  make(map[string]interface{}),
		Interfaces: make(map[string]*Interface),
		IfIndex:    index,
		NetNs:      n,
	}
	n.Interfaces[i] = intf

	n.Topology.Log()

	return intf
}

// AddInterface add a previously created interface
func (n *NetNs) AddInterface(intf *Interface) {
	n.Lock()
	defer n.Unlock()
	intf.Lock()
	defer intf.Unlock()

	n.Interfaces[intf.ID] = intf
	intf.NetNs = n

	n.Topology.Log()
}

// DelInterface removes the interface with the given id from the port
func (n *NetNs) DelInterface(i string) {
	n.Lock()
	defer n.Unlock()

	delete(n.Interfaces, i)

	n.Topology.Log()
}

// GetInterface returns the interface with the given ID from the port
func (n *NetNs) GetInterface(i string) *Interface {
	n.Lock()
	defer n.Unlock()

	if intf, ok := n.Interfaces[i]; ok {
		return intf
	}
	return nil
}

// SetMetadata attach metadata to the port
func (p *Port) SetMetadata(key string, value interface{}) {
	p.Lock()
	defer p.Unlock()

	p.Metadatas[key] = value
}

// Del removes the port from the ovs bridge containing it
func (p *Port) Del() {
	if p.OvsBridge != nil {
		p.OvsBridge.DelPort(p.ID)
	}
}

// NewInterface instantiate a new interface with a given index
func (p *Port) NewInterface(i string, index uint32) *Interface {
	p.Lock()
	defer p.Unlock()

	u, _ := uuid.NewV4()

	intf := &Interface{
		ID:         i,
		UUID:       u.String(),
		Metadatas:  make(map[string]interface{}),
		Interfaces: make(map[string]*Interface),
		IfIndex:    index,
		Port:       p,
	}
	p.Interfaces[i] = intf

	if p.OvsBridge != nil {
		p.OvsBridge.Topology.Log()
	}

	return intf
}

// AddInterface add a previously created interface
func (p *Port) AddInterface(intf *Interface) {
	p.Lock()
	defer p.Unlock()
	intf.Lock()
	intf.Unlock()

	p.Interfaces[intf.ID] = intf
	intf.Port = p

	if p.OvsBridge != nil {
		p.OvsBridge.Topology.Log()
	}
}

// DelInterface removes the interface with the given id from the port
func (p *Port) DelInterface(i string) {
	p.Lock()
	defer p.Unlock()

	delete(p.Interfaces, i)

	p.OvsBridge.Topology.Log()
}

// GetInterface returns the interface with the given ID from the port
func (p *Port) GetInterface(i string) *Interface {
	p.Lock()
	defer p.Unlock()

	if intf, ok := p.Interfaces[i]; ok {
		return intf
	}
	return nil
}

// GetPort returns the port with the given port ID from the container
func (o *OvsBridge) GetPort(i string) *Port {
	o.Lock()
	defer o.Unlock()

	if port, ok := o.Ports[i]; ok {
		return port
	}
	return nil
}

// DelPort removes the port with the given id from the ovs bridge
func (o *OvsBridge) DelPort(i string) {
	o.Lock()
	defer o.Unlock()

	delete(o.Ports, i)

	o.Topology.Log()
}

// NewPort intentiates a new port and add it to the ovs bridge
func (o *OvsBridge) NewPort(i string) *Port {
	o.Lock()
	defer o.Unlock()

	port := &Port{
		ID:         i,
		Interfaces: make(map[string]*Interface),
		Metadatas:  make(map[string]interface{}),
		OvsBridge:  o,
	}
	o.Ports[i] = port

	o.Topology.Log()

	return port
}

// AddPort add a previously created port to the ovs bridge
func (o *OvsBridge) AddPort(p *Port) {
	o.Lock()
	defer o.Unlock()
	p.Lock()
	defer p.Unlock()

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
	topo.Lock()
	defer topo.Unlock()

	if (scope & NetNSScope) > 0 {
		for _, netns := range topo.NetNss {
			netns.Lock()
			defer netns.Unlock()
			for _, intf := range netns.Interfaces {
				intf.Lock()
				if f(intf) {
					intf.Unlock()
					return intf
				}
				intf.Unlock()
			}
		}
	}

	if (scope & OvsScope) > 0 {
		for _, bridge := range topo.OvsBridges {
			bridge.Lock()
			defer bridge.Unlock()
			for _, port := range bridge.Ports {
				port.Lock()
				defer port.Unlock()
				for _, intf := range port.Interfaces {
					intf.Lock()
					if f(intf) {
						intf.Unlock()
						return intf
					}
					intf.Unlock()
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
	topo.Lock()
	defer topo.Unlock()

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

func NewTopology() *Topology {
	return &Topology{
		OvsBridges: make(map[string]*OvsBridge),
		NetNss:     make(map[string]*NetNs),
	}
}
