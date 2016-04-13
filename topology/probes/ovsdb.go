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

package probes

import (
	"sync"

	"github.com/socketplane/libovsdb"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/ovs"
	"github.com/redhat-cip/skydive/topology/graph"
)

type OvsdbProbe struct {
	sync.Mutex
	Graph           *graph.Graph
	Root            *graph.Node
	OvsMon          *ovsdb.OvsMonitor
	uuidToIntf      map[string]*graph.Node
	uuidToPort      map[string]*graph.Node
	intfPortQueue   map[string]*graph.Node
	portBridgeQueue map[string]*graph.Node
}

func (o *OvsdbProbe) OnOvsBridgeUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsBridgeAdd(monitor, uuid, row)
}

func (o *OvsdbProbe) OnOvsBridgeAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	name := row.New.Fields["name"].(string)

	o.Graph.Lock()
	defer o.Graph.Unlock()

	bridge := o.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if bridge == nil {
		bridge = o.Graph.NewNode(graph.GenID(), graph.Metadata{"Name": name, "UUID": uuid, "Type": "ovsbridge"})
		o.Graph.Link(o.Root, bridge, graph.Metadata{"RelationType": "ownership"})
	}

	switch row.New.Fields["ports"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["ports"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUuid

			port, ok := o.uuidToPort[u]
			if ok && !o.Graph.AreLinked(bridge, port) {
				o.Graph.Link(bridge, port, graph.Metadata{"RelationType": "layer2"})
			} else {
				/* will be filled later when the port update for this port will be triggered */
				o.portBridgeQueue[u] = bridge
			}
		}

	case libovsdb.UUID:
		u := row.New.Fields["ports"].(libovsdb.UUID).GoUuid

		port, ok := o.uuidToPort[u]
		if ok && !o.Graph.AreLinked(bridge, port) {
			o.Graph.Link(bridge, port, graph.Metadata{"RelationType": "layer2"})
		} else {
			/* will be filled later when the port update for this port will be triggered */
			o.portBridgeQueue[u] = bridge
		}
	}
}

func (o *OvsdbProbe) OnOvsBridgeDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Graph.Lock()
	defer o.Graph.Unlock()

	bridge := o.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if bridge != nil {
		o.Graph.DelNode(bridge)
	}
}

func (o *OvsdbProbe) OnOvsInterfaceAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	// NOTE(safchain) is it a workaround ???, seems that the interface is not fully set
	switch row.New.Fields["ofport"].(type) {
	case float64:
	default:
		return
	}

	var mac string
	switch row.New.Fields["mac_in_use"].(type) {
	case string:
		mac = row.New.Fields["mac_in_use"].(string)
	}

	var index int64
	if i, ok := row.New.Fields["ifindex"]; ok {
		switch row.New.Fields["ifindex"].(type) {
		case float64:
			index = int64(i.(float64))
		case libovsdb.OvsSet:
			set := row.New.Fields["ifindex"].(libovsdb.OvsSet)
			if len(set.GoSet) > 0 {
				index = set.GoSet[0].(int64)
			}
		}
	}

	var driver string
	if d, ok := row.New.Fields["status"].(libovsdb.OvsMap).GoMap["driver_name"]; ok {
		driver = d.(string)
	}

	var itype string
	if t, ok := row.New.Fields["type"]; ok {
		itype = t.(string)
	}

	name := row.New.Fields["name"].(string)

	o.Graph.Lock()
	defer o.Graph.Unlock()

	intf := o.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if intf == nil {
		lm := graph.Metadata{"IfIndex": index}
		if mac != "" {
			lm["MAC"] = mac
		}
		intf = o.Graph.LookupFirstChild(o.Root, lm)
		if intf != nil {
			o.Graph.AddMetadata(intf, "UUID", uuid)
		}
	}

	if intf == nil {
		metadata := graph.Metadata{"Name": name, "UUID": uuid}

		if mac != "" {
			metadata["MAC"] = mac
		}

		if driver != "" {
			metadata["Driver"] = driver
		}

		if itype != "" {
			metadata["Type"] = itype
		}

		if index > 0 {
			metadata["IfIndex"] = index
		}

		intf = o.Graph.NewNode(graph.GenID(), metadata)
	}

	// check wether a interface with the same mac exist, could have been added by netlink
	// in such case, replace the netlink node by the ovs one keeping netlink metadata + uuid
	if index > 0 {
		nodes := o.Graph.LookupChildren(o.Root, graph.Metadata{"IfIndex": index})
		for _, node := range nodes {
			if node.Metadata()["UUID"] != uuid {
				m := node.Metadata()
				m["UUID"] = uuid
				intf = o.Graph.Replace(node, intf)
			}
		}

		o.Graph.AddMetadata(intf, "IfIndex", index)
	}

	if mac != "" {
		o.Graph.AddMetadata(intf, "MAC", mac)
	}

	if driver != "" {
		o.Graph.AddMetadata(intf, "Driver", driver)
	}

	if itype != "" {
		o.Graph.AddMetadata(intf, "Type", itype)
	}

	ext_ids := row.New.Fields["external_ids"].(libovsdb.OvsMap)
	for k, v := range ext_ids.GoMap {
		o.Graph.AddMetadata(intf, "ExtID."+k.(string), v.(string))
	}

	o.uuidToIntf[uuid] = intf

	switch itype {
	case "gre", "vxlan":
		o.Graph.AddMetadata(intf, "Driver", "openvswitch")

		m := row.New.Fields["options"].(libovsdb.OvsMap)
		if ip, ok := m.GoMap["local_ip"]; ok {
			o.Graph.AddMetadata(intf, "LocalIP", ip.(string))
		}
		if ip, ok := m.GoMap["remote_ip"]; ok {
			o.Graph.AddMetadata(intf, "RemoteIP", ip.(string))
		}
		m = row.New.Fields["status"].(libovsdb.OvsMap)
		if iface, ok := m.GoMap["tunnel_egress_iface"]; ok {
			o.Graph.AddMetadata(intf, "TunEgressIface", iface.(string))
		}
		if carrier, ok := m.GoMap["tunnel_egress_iface_carrier"]; ok {
			o.Graph.AddMetadata(intf, "TunEgressIfaceCarrier", carrier.(string))
		}

	case "patch":
		// force the driver as it is not defined and we need it to delete properly
		o.Graph.AddMetadata(intf, "Driver", "openvswitch")

		m := row.New.Fields["options"].(libovsdb.OvsMap)
		if p, ok := m.GoMap["peer"]; ok {

			peerName := p.(string)

			peer := o.Graph.LookupFirstNode(graph.Metadata{"Name": peerName, "Type": "patch"})
			if peer != nil {
				if !o.Graph.AreLinked(intf, peer) {
					o.Graph.Link(intf, peer, graph.Metadata{"RelationType": "layer2", "Type": "patch"})
				}
			} else {
				// lookup in the intf queue
				for _, peer := range o.uuidToIntf {
					if peer.Metadata()["Name"] == peerName && !o.Graph.AreLinked(intf, peer) {
						o.Graph.Link(intf, peer, graph.Metadata{"RelationType": "layer2", "Type": "patch"})
					}
				}
			}
		}
	}

	/* set pending interface for a port */
	if port, ok := o.intfPortQueue[uuid]; ok {
		o.Graph.Link(port, intf, graph.Metadata{"RelationType": "layer2"})
		delete(o.intfPortQueue, uuid)
	}
}

func (o *OvsdbProbe) OnOvsInterfaceUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsInterfaceAdd(monitor, uuid, row)
}

func (o *OvsdbProbe) OnOvsInterfaceDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	intf, ok := o.uuidToIntf[uuid]
	if !ok {
		return
	}

	o.Graph.Lock()
	defer o.Graph.Unlock()

	// do not delete if not an openvswitch interface
	if driver, ok := intf.Metadata()["Driver"]; ok && driver == "openvswitch" {
		o.Graph.DelNode(intf)
	}

	delete(o.uuidToIntf, uuid)
}

func (o *OvsdbProbe) OnOvsPortAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	o.Graph.Lock()
	defer o.Graph.Unlock()

	port, ok := o.uuidToPort[uuid]
	if !ok {
		port = o.Graph.NewNode(graph.GenID(), graph.Metadata{
			"UUID": uuid,
			"Name": row.New.Fields["name"].(string),
			"Type": "ovsport",
		})
		o.uuidToPort[uuid] = port
	}

	// bond mode
	if mode, ok := row.New.Fields["bond_mode"]; ok {
		switch mode.(type) {
		case string:
			o.Graph.AddMetadata(port, "BondMode", mode.(string))
		}
	}

	// lacp
	if lacp, ok := row.New.Fields["lacp"]; ok {
		switch lacp.(type) {
		case string:
			o.Graph.AddMetadata(port, "LACP", lacp.(string))
		}
	}

	// vlan tag
	if tag, ok := row.New.Fields["tag"]; ok {
		switch tag.(type) {
		case libovsdb.OvsSet:
			set := tag.(libovsdb.OvsSet)
			if len(set.GoSet) > 0 {
				o.Graph.AddMetadata(port, "Vlans", set.GoSet)
			}
		case float64:
			o.Graph.AddMetadata(port, "Vlans", int(tag.(float64)))
		}
	}

	switch row.New.Fields["interfaces"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["interfaces"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUuid
			intf, ok := o.uuidToIntf[u]
			if ok && !o.Graph.AreLinked(port, intf) {
				o.Graph.Link(port, intf, graph.Metadata{"RelationType": "layer2"})
			} else {
				/* will be filled later when the interface update for this interface will be triggered */
				o.intfPortQueue[u] = port
			}
		}
	case libovsdb.UUID:
		u := row.New.Fields["interfaces"].(libovsdb.UUID).GoUuid
		intf, ok := o.uuidToIntf[u]
		if ok && !o.Graph.AreLinked(port, intf) {
			o.Graph.Link(port, intf, graph.Metadata{"RelationType": "layer2"})
		} else {
			/* will be filled later when the interface update for this interface will be triggered */
			o.intfPortQueue[u] = port
		}
	}

	/* set pending port of a container */
	if bridge, ok := o.portBridgeQueue[uuid]; ok {
		o.Graph.Link(bridge, port, graph.Metadata{"RelationType": "layer2"})
		delete(o.portBridgeQueue, uuid)
	}
}

func (o *OvsdbProbe) OnOvsPortUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsPortAdd(monitor, uuid, row)
}

func (o *OvsdbProbe) OnOvsPortDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	port, ok := o.uuidToPort[uuid]
	if !ok {
		return
	}

	o.Graph.Lock()
	defer o.Graph.Unlock()

	o.Graph.DelNode(port)

	delete(o.uuidToPort, uuid)
}

func (o *OvsdbProbe) Start() {
	o.OvsMon.StartMonitoring()
}

func (o *OvsdbProbe) Stop() {
	o.OvsMon.StopMonitoring()
}

func NewOvsdbProbe(g *graph.Graph, n *graph.Node, addr string, port int) *OvsdbProbe {
	o := &OvsdbProbe{
		Graph:           g,
		Root:            n,
		uuidToIntf:      make(map[string]*graph.Node),
		uuidToPort:      make(map[string]*graph.Node),
		intfPortQueue:   make(map[string]*graph.Node),
		portBridgeQueue: make(map[string]*graph.Node),
		OvsMon:          ovsdb.NewOvsMonitor(addr, port),
	}
	o.OvsMon.AddMonitorHandler(o)

	return o
}

func NewOvsdbProbeFromConfig(g *graph.Graph, n *graph.Node) *OvsdbProbe {
	addr, port, err := config.GetHostPortAttributes("ovs", "ovsdb")
	if err != nil {
		logging.GetLogger().Errorf("Configuration error: %s", err.Error())
		return nil
	}

	return NewOvsdbProbe(g, n, addr, port)
}
