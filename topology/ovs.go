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
	Topology           *Topology
	uuidToPort         map[string]*Port
	uuidToIntf         map[string]*Interface
	intfPortQueue      map[string]*Port
	portContainerQueue map[string]*Container
}

func (o *OvsTopoUpdater) OnOvsBridgeUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsBridgeAdd(monitor, uuid, row)
}

func (o *OvsTopoUpdater) OnOvsBridgeAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	name := row.New.Fields["name"].(string)

	container := o.Topology.GetContainer(name)
	if container == nil {
		container = o.Topology.NewContainer(name, OvsBridge)
	}

	switch row.New.Fields["ports"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["ports"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUuid

			if port, ok := o.uuidToPort[u]; ok {
				container.AddPort(port)
			} else {
				/* will be filled later when the port update for this port will be triggered */
				o.portContainerQueue[u] = container
			}
		}

	case libovsdb.UUID:
		u := row.New.Fields["ports"].(libovsdb.UUID).GoUuid
		if port, ok := o.uuidToPort[u]; ok {
			container.AddPort(port)
		} else {
			/* will be filled later when the port update for this port will be triggered */
			o.portContainerQueue[u] = container
		}
	}
}

func (o *OvsTopoUpdater) OnOvsBridgeDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Topology.DelContainer(row.Old.Fields["name"].(string))
}

func (o *OvsTopoUpdater) OnOvsInterfaceAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	intf, ok := o.uuidToIntf[uuid]
	if !ok {
		intf = o.Topology.NewInterface(row.New.Fields["name"].(string))
		o.uuidToIntf[uuid] = intf
	}

	switch row.New.Fields["mac_in_use"].(type) {
	case string:
		intf.SetMac(row.New.Fields["mac_in_use"].(string))
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

	port, ok := o.uuidToPort[uuid]
	if !ok {
		port = o.Topology.NewPort(row.New.Fields["name"].(string))
		o.uuidToPort[uuid] = port
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
	if container, ok := o.portContainerQueue[uuid]; ok {
		container.AddPort(port)
		delete(o.portContainerQueue, uuid)
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
		Topology:           topo,
		uuidToPort:         make(map[string]*Port),
		uuidToIntf:         make(map[string]*Interface),
		intfPortQueue:      make(map[string]*Port),
		portContainerQueue: make(map[string]*Container),
	}
	ovsmon.AddMonitorHandler(u)

	return u
}
