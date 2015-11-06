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
	Topology     *Topology
	ifNameToPort map[string]string
	portToIfName map[string]string
	portToBridge map[string]string
}

func (o *OvsTopoUpdater) OnOvsBridgeUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsBridgeAdd(monitor, uuid, row)
}

func (o *OvsTopoUpdater) OnOvsBridgeAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	name, ok := row.New.Fields["name"]
	if !ok {
		return
	}
	c := o.Topology.NewContainer(name.(string), OvsBridge)

	o.Lock()
	defer o.Unlock()

	switch row.New.Fields["ports"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["ports"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUuid
			o.portToBridge[u] = name.(string)

			ifName, ok := o.portToIfName[u]
			if ok {
				c.NewNode(ifName)
			}
		}

	case libovsdb.UUID:
		u := row.New.Fields["ports"].(libovsdb.UUID).GoUuid
		o.portToBridge[u] = name.(string)
	}
}

func (o *OvsTopoUpdater) OnOvsBridgeDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	name, ok := row.Old.Fields["name"]
	if !ok {
		return
	}
	o.Topology.DelContainer(name.(string))

	o.Lock()
	defer o.Unlock()

	switch row.Old.Fields["ports"].(type) {
	case libovsdb.OvsSet:
		set := row.Old.Fields["ports"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUuid
			delete(o.portToBridge, u)
		}
	case libovsdb.UUID:
		u := row.Old.Fields["ports"].(libovsdb.UUID).GoUuid

		delete(o.portToBridge, u)
	}
}

func (o *OvsTopoUpdater) OnOvsInterfaceAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsTopoUpdater) OnOvsInterfaceUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsTopoUpdater) OnOvsInterfaceDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsTopoUpdater) OnOvsPortAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	name := row.New.Fields["name"].(string)

	o.Lock()
	defer o.Unlock()

	o.ifNameToPort[name] = uuid
	o.portToIfName[uuid] = name

	bridge, ok := o.portToBridge[uuid]
	if !ok {
		return
	}

	c := o.Topology.GetContainer(bridge)
	if c != nil {
		c.NewNode(name)
	}
}

func (o *OvsTopoUpdater) OnOvsPortUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

func (o *OvsTopoUpdater) OnOvsPortDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	name := row.Old.Fields["name"].(string)

	o.Lock()
	defer o.Unlock()

	delete(o.ifNameToPort, name)
	delete(o.portToIfName, uuid)

	bridge, ok := o.portToBridge[uuid]
	if !ok {
		return
	}

	c := o.Topology.GetContainer(bridge)
	if c != nil {
		c.DelNode(name)
	}
}

func (o *OvsTopoUpdater) Start() {
}

func (o *OvsTopoUpdater) GetBridgeByIntfName(name string) string {
	port, ok := o.ifNameToPort[name]
	if !ok {
		return ""
	}

	bridge, ok := o.portToBridge[port]
	if !ok {
		return ""
	}

	return bridge
}

func NewOvsTopoUpdater(topo *Topology, ovsmon *ovsdb.OvsMonitor) *OvsTopoUpdater {
	u := &OvsTopoUpdater{
		Topology:     topo,
		ifNameToPort: make(map[string]string),
		portToIfName: make(map[string]string),
		portToBridge: make(map[string]string),
	}
	ovsmon.AddMonitorHandler(u)

	return u
}
