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
	"sync"

	"github.com/socketplane/libovsdb"

	"github.com/redhat-cip/skydive/ovs"
)

type OvsTopoUpdater struct {
	sync.Mutex
	Topology        *Topology
	uuidToPort      map[string]*Port
	uuidToIntf      map[string]*Interface
	intfPortQueue   map[string]*Port
	portBridgeQueue map[string]*OvsBridge
}

func (o *OvsTopoUpdater) OnOvsBridgeUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsBridgeAdd(monitor, uuid, row)
}

func (o *OvsTopoUpdater) OnOvsBridgeAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	name := row.New.Fields["name"].(string)

	bridge := o.Topology.GetOvsBridge(name)
	if bridge == nil {
		bridge = o.Topology.NewOvsBridge(name)
	}

	switch row.New.Fields["ports"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["ports"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUuid

			if port, ok := o.uuidToPort[u]; ok {
				bridge.AddPort(port)
			} else {
				/* will be filled later when the port update for this port will be triggered */
				o.portBridgeQueue[u] = bridge
			}
		}

	case libovsdb.UUID:
		u := row.New.Fields["ports"].(libovsdb.UUID).GoUuid
		if port, ok := o.uuidToPort[u]; ok {
			bridge.AddPort(port)
		} else {
			/* will be filled later when the port update for this port will be triggered */
			o.portBridgeQueue[u] = bridge
		}
	}
}

func (o *OvsTopoUpdater) OnOvsBridgeDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Topology.DelOvsBridge(row.Old.Fields["name"].(string))
}

func (o *OvsTopoUpdater) OnOvsInterfaceAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	var mac string
	switch row.New.Fields["mac_in_use"].(type) {
	case string:
		mac = row.New.Fields["mac_in_use"].(string)
	}

	var driver string
	if d, ok := row.New.Fields["status"].(libovsdb.OvsMap).GoMap["driver_name"]; ok {
		driver = d.(string)
	}

	name := row.New.Fields["name"].(string)

	o.Topology.StartMultipleOperations()
	defer o.Topology.StopMultipleOperations()

	intf := o.Topology.LookupInterface(LookupByUUID(uuid), OvsScope)
	if intf != nil {
		// mac has been set or changed
		if mac != intf.GetMac() {
			// FIX(safchain) since an ovs interface can be added without any mac address, we can have twice the same interface.
			// one in ovs and one in netns. Having now the mac we can check if the same interface is in both places. If so,
			// remove the ovs interface and add the netns interface to the ovs port.
			if i := o.Topology.LookupInterface(LookupByMac(name, mac), NetNSScope|OvsScope); i != nil {
				if port := intf.GetPort(); port != nil {
					port.DelInterface(intf.ID)
					port.AddInterface(i)
					intf = i
				}
			}
		}
	}

	// check if a new interface is needed
	if intf == nil {
		if intf = o.Topology.LookupInterface(LookupByMac(name, mac), NetNSScope|OvsScope); intf == nil {
			intf = o.Topology.NewInterfaceWithUUID(name, uuid)
		}
	}

	intf.SetType(driver)
	intf.SetMac(mac)
	o.uuidToIntf[uuid] = intf

	// type
	if t, ok := row.New.Fields["type"]; ok {
		tp := t.(string)
		if len(tp) > 0 {
			intf.SetMetadata("Type", tp)
		}

		// handle tunnels
		switch tp {
		case "gre":
			fallthrough
		case "vxlan":
			m := row.New.Fields["options"].(libovsdb.OvsMap)
			if ip, ok := m.GoMap["local_ip"]; ok {
				intf.SetMetadata("LocalIP", ip.(string))
			}
			if ip, ok := m.GoMap["remote_ip"]; ok {
				intf.SetMetadata("RemoteIP", ip.(string))
			}
		case "patch":
			m := row.New.Fields["options"].(libovsdb.OvsMap)
			if p, ok := m.GoMap["peer"]; ok {

				peer := o.Topology.LookupInterface(LookupByID(p.(string)), OvsScope)
				if peer != nil {
					intf.SetPeer(peer)
				} else {
					// lookup in the intf queue
					for _, peer = range o.uuidToIntf {
						if peer.ID == p.(string) {
							intf.SetPeer(peer)
						}
					}
				}
			}
		}
	}

	/* set pending interface for a port */
	if port, ok := o.intfPortQueue[uuid]; ok {
		port.AddInterface(intf)
		delete(o.intfPortQueue, uuid)
	}
}

func (o *OvsTopoUpdater) OnOvsInterfaceUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsInterfaceAdd(monitor, uuid, row)
}

func (o *OvsTopoUpdater) OnOvsInterfaceDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	intf, ok := o.uuidToIntf[uuid]
	if !ok {
		return
	}
	intf.Del()

	delete(o.uuidToIntf, uuid)
}

func (o *OvsTopoUpdater) OnOvsPortAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	o.Topology.StartMultipleOperations()
	defer o.Topology.StopMultipleOperations()

	port, ok := o.uuidToPort[uuid]
	if !ok {
		port = o.Topology.NewPort(row.New.Fields["name"].(string))
		o.uuidToPort[uuid] = port
	}

	// vlan tag
	if tag, ok := row.New.Fields["tag"]; ok {
		switch tag.(type) {
		case libovsdb.OvsSet:
			set := tag.(libovsdb.OvsSet)
			if len(set.GoSet) > 0 {
				port.SetMetadata("Vlans", set.GoSet)
			}
		case float64:
			port.SetMetadata("Vlans", int(tag.(float64)))
		}
	}

	switch row.New.Fields["interfaces"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["interfaces"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUuid
			intf, ok := o.uuidToIntf[u]
			if ok {
				port.AddInterface(intf)
			} else {
				/* will be filled later when the interface update for this interface will be triggered */
				o.intfPortQueue[u] = port
			}
		}
	case libovsdb.UUID:
		u := row.New.Fields["interfaces"].(libovsdb.UUID).GoUuid
		intf, ok := o.uuidToIntf[u]
		if ok {
			port.AddInterface(intf)
		} else {
			/* will be filled later when the interface update for this interface will be triggered */
			o.intfPortQueue[u] = port
		}
	}

	/* set pending port of a container */
	if bridge, ok := o.portBridgeQueue[uuid]; ok {
		bridge.AddPort(port)
		delete(o.portBridgeQueue, uuid)
	}
}

func (o *OvsTopoUpdater) OnOvsPortUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsPortAdd(monitor, uuid, row)
}

func (o *OvsTopoUpdater) OnOvsPortDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	port, ok := o.uuidToPort[uuid]
	if !ok {
		return
	}
	port.Del()

	delete(o.uuidToPort, uuid)
}

func (o *OvsTopoUpdater) Start() {
}

func NewOvsTopoUpdater(topo *Topology, ovsmon *ovsdb.OvsMonitor) *OvsTopoUpdater {
	u := &OvsTopoUpdater{
		Topology:        topo,
		uuidToPort:      make(map[string]*Port),
		uuidToIntf:      make(map[string]*Interface),
		intfPortQueue:   make(map[string]*Port),
		portBridgeQueue: make(map[string]*OvsBridge),
	}
	ovsmon.AddMonitorHandler(u)

	return u
}
