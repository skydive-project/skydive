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

package ovsdb

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/socketplane/libovsdb"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/ovs"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

var (
	patchMetadata = graph.Metadata{"Type": "patch"}
)

// OvsdbProbe describes a probe that reads OVS database and updates the graph
type OvsdbProbe struct {
	sync.Mutex
	Graph        *graph.Graph
	Root         *graph.Node
	OvsMon       *ovsdb.OvsMonitor
	OvsOfProbe   *OvsOfProbe
	uuidToIntf   map[string]*graph.Node
	uuidToPort   map[string]*graph.Node
	intfToPort   map[string]*graph.Node
	portToIntf   map[string]*graph.Node
	portToBridge map[string]*graph.Node
}

func isOvsLogicalInterface(intf *graph.Node) bool {
	if d, _ := intf.GetFieldString("Driver"); d != "openvswitch" {
		return false
	}

	t, _ := intf.GetFieldString("Type")
	switch t {
	case "gre", "vxlan", "geneve", "patch":
		return true
	}
	return true
}

// OnOvsBridgeUpdate event
func (o *OvsdbProbe) OnOvsBridgeUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsBridgeAdd(monitor, uuid, row)
}

// OnOvsBridgeAdd event
func (o *OvsdbProbe) OnOvsBridgeAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	name := row.New.Fields["name"].(string)

	o.Graph.Lock()
	defer o.Graph.Unlock()

	bridge := o.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if bridge == nil {
		bridge = o.Graph.NewNode(graph.GenID(), graph.Metadata{"Name": name, "UUID": uuid, "Type": "ovsbridge"})
		topology.AddOwnershipLink(o.Graph, o.Root, bridge, nil)
	}

	switch row.New.Fields["ports"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["ports"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUUID
			o.portToBridge[u] = bridge

			if port, ok := o.uuidToPort[u]; ok {
				if !topology.HaveOwnershipLink(o.Graph, bridge, port, nil) {
					topology.AddOwnershipLink(o.Graph, bridge, port, nil)
					topology.AddLayer2Link(o.Graph, bridge, port, nil)
				}

				// internal ovs interface that have to be linked to the bridge as they
				// are just logical interface.
				if intf, ok := o.portToIntf[uuid]; ok && isOvsLogicalInterface(intf) {
					if !topology.IsOwnershipLinked(o.Graph, intf) {
						topology.AddOwnershipLink(o.Graph, bridge, intf, nil)
					}
				}
			}
		}

	case libovsdb.UUID:
		u := row.New.Fields["ports"].(libovsdb.UUID).GoUUID
		o.portToBridge[u] = bridge

		if port, ok := o.uuidToPort[u]; ok {
			if !topology.HaveOwnershipLink(o.Graph, bridge, port, nil) {
				topology.AddOwnershipLink(o.Graph, bridge, port, nil)
				topology.AddLayer2Link(o.Graph, bridge, port, nil)
			}
		}
	}
	if o.OvsOfProbe != nil {
		o.OvsOfProbe.OnOvsBridgeAdd(bridge)
	}
}

// OnOvsBridgeDel event
func (o *OvsdbProbe) OnOvsBridgeDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Graph.Lock()
	defer o.Graph.Unlock()

	bridge := o.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if o.OvsOfProbe != nil {
		o.OvsOfProbe.OnOvsBridgeDel(uuid, bridge)
	}
	if bridge != nil {
		o.Graph.DelNode(bridge)
	}
}

// NewInterfaceMetricsFromNetlink returns a new InterfaceMetric object using
// values of netlink.
func newInterfaceMetricsFromOVSDB(stats libovsdb.OvsMap) *topology.InterfaceMetric {
	setInt64 := func(k string) int64 {
		if v, ok := stats.GoMap[k]; ok {
			return int64(v.(float64))
		}
		return 0
	}

	return &topology.InterfaceMetric{
		Collisions:    setInt64("collisions"),
		RxBytes:       setInt64("rx_bytes"),
		RxCrcErrors:   setInt64("rx_crc_err"),
		RxDropped:     setInt64("rx_dropped"),
		RxErrors:      setInt64("rx_errors"),
		RxFrameErrors: setInt64("rx_frame_err"),
		RxOverErrors:  setInt64("rx_over_err"),
		RxPackets:     setInt64("rx_packets"),
		TxBytes:       setInt64("tx_bytes"),
		TxDropped:     setInt64("tx_dropped"),
		TxErrors:      setInt64("tx_errors"),
		TxPackets:     setInt64("tx_packets"),
	}
}

// OnOvsInterfaceAdd event
func (o *OvsdbProbe) OnOvsInterfaceAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	var ofport int64
	switch row.New.Fields["ofport"].(type) {
	case float64:
		ofport = int64(row.New.Fields["ofport"].(float64))
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

	var itype string
	if t, ok := row.New.Fields["type"]; ok {
		itype = t.(string)
	}

	var driver string
	if d, ok := row.New.Fields["status"].(libovsdb.OvsMap).GoMap["driver_name"]; ok {
		driver = d.(string)
	}

	if driver == "" {
		// force the driver as it is not defined and we need it to delete properly
		switch itype {
		case "gre", "vxlan", "geneve", "patch":
			driver = "openvswitch"
		default:
			driver = "unknown"
		}
	}

	name := row.New.Fields["name"].(string)

	o.Graph.Lock()
	defer o.Graph.Unlock()

	intf := o.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if intf == nil {
		lm := graph.Metadata{"Name": name, "Driver": driver}
		if index > 0 {
			lm["IfIndex"] = index
		}

		// added before by netlink ?
		if intf = o.Graph.LookupFirstNode(lm); intf != nil {
			o.Graph.AddMetadata(intf, "UUID", uuid)
		}
	}

	if intf == nil {
		intf = o.Graph.NewNode(graph.GenID(), graph.Metadata{"Name": name, "UUID": uuid, "Driver": driver})
	}

	tr := o.Graph.StartMetadataTransaction(intf)
	defer tr.Commit()

	if ofport != 0 {
		tr.AddMetadata("OfPort", ofport)
	}

	if index > 0 {
		tr.AddMetadata("IfIndex", index)
	}

	if mac != "" {
		tr.AddMetadata("MAC", mac)
	}

	if itype != "" {
		tr.AddMetadata("Type", itype)
	}

	extIds := row.New.Fields["external_ids"].(libovsdb.OvsMap)
	for k, v := range extIds.GoMap {
		tr.AddMetadata("ExtID."+k.(string), v.(string))
	}

	options := row.New.Fields["options"].(libovsdb.OvsMap)
	for k, v := range options.GoMap {
		tr.AddMetadata("Ovs.Options."+k.(string), v.(string))
	}

	otherConfig := row.New.Fields["other_config"].(libovsdb.OvsMap)
	for k, v := range otherConfig.GoMap {
		tr.AddMetadata("Ovs.OtherConfig."+k.(string), v.(string))
	}

	o.uuidToIntf[uuid] = intf

	switch itype {
	case "gre", "vxlan", "geneve":
		m := row.New.Fields["options"].(libovsdb.OvsMap)
		if ip, ok := m.GoMap["local_ip"]; ok {
			tr.AddMetadata("LocalIP", ip.(string))
		}
		if ip, ok := m.GoMap["remote_ip"]; ok {
			tr.AddMetadata("RemoteIP", ip.(string))
		}
		m = row.New.Fields["status"].(libovsdb.OvsMap)
		if iface, ok := m.GoMap["tunnel_egress_iface"]; ok {
			tr.AddMetadata("TunEgressIface", iface.(string))
		}
		if carrier, ok := m.GoMap["tunnel_egress_iface_carrier"]; ok {
			tr.AddMetadata("TunEgressIfaceCarrier", carrier.(string))
		}

	case "patch":
		m := row.New.Fields["options"].(libovsdb.OvsMap)
		if p, ok := m.GoMap["peer"]; ok {

			peerName := p.(string)

			peer := o.Graph.LookupFirstNode(graph.Metadata{"Name": peerName, "Type": "patch"})
			if peer != nil {
				if !topology.HaveLayer2Link(o.Graph, intf, peer, nil) {
					topology.AddLayer2Link(o.Graph, intf, peer, patchMetadata)
				}
			} else {
				// lookup in the intf queue
				for _, peer := range o.uuidToIntf {
					if name, _ := peer.GetFieldString("Name"); name == peerName && !topology.HaveLayer2Link(o.Graph, intf, peer, patchMetadata) {
						topology.AddLayer2Link(o.Graph, intf, peer, patchMetadata)
					}
				}
			}
		}
	}

	if field, ok := row.New.Fields["statistics"]; ok {
		now := time.Now()

		statistics := field.(libovsdb.OvsMap)
		currMetric := newInterfaceMetricsFromOVSDB(statistics)
		currMetric.Last = int64(common.UnixMillis(now))

		var prevMetric, lastUpdateMetric *topology.InterfaceMetric

		if ovs, ok := tr.Metadata["Ovs"]; ok {
			prevMetric, ok = ovs.(map[string]interface{})["Metric"].(*topology.InterfaceMetric)
			if ok {
				lastUpdateMetric = currMetric.Sub(prevMetric).(*topology.InterfaceMetric)
			}
		}

		tr.AddMetadata("Ovs.Metric", currMetric)

		// nothing changed since last update
		if lastUpdateMetric != nil && !lastUpdateMetric.IsZero() {
			lastUpdateMetric.Start = prevMetric.Last
			lastUpdateMetric.Last = int64(common.UnixMillis(now))
			tr.AddMetadata("Ovs.LastUpdateMetric", lastUpdateMetric)
		}
	}

	if port, ok := o.intfToPort[uuid]; ok {
		if !topology.HaveLayer2Link(o.Graph, port, intf, nil) {
			topology.AddLayer2Link(o.Graph, port, intf, nil)
		}

		puuid, _ := port.GetFieldString("UUID")
		if brige, ok := o.portToBridge[puuid]; ok && isOvsLogicalInterface(intf) {
			if !topology.IsOwnershipLinked(o.Graph, intf) {
				topology.AddOwnershipLink(o.Graph, brige, intf, nil)
			}
		}
	}
}

// OnOvsInterfaceUpdate event
func (o *OvsdbProbe) OnOvsInterfaceUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsInterfaceAdd(monitor, uuid, row)
}

// OnOvsInterfaceDel event
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
	if driver, _ := intf.GetFieldString("Driver"); driver == "openvswitch" {
		o.Graph.DelNode(intf)
	}

	delete(o.uuidToIntf, uuid)
	delete(o.intfToPort, uuid)
}

// OnOvsPortAdd event
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

	tr := o.Graph.StartMetadataTransaction(port)
	defer tr.Commit()

	// bond mode
	if mode, ok := row.New.Fields["bond_mode"]; ok {
		switch mode.(type) {
		case string:
			tr.AddMetadata("BondMode", mode.(string))
		}
	}

	// lacp
	if lacp, ok := row.New.Fields["lacp"]; ok {
		switch lacp.(type) {
		case string:
			tr.AddMetadata("LACP", lacp.(string))
		}
	}

	extIds := row.New.Fields["external_ids"].(libovsdb.OvsMap)
	for k, v := range extIds.GoMap {
		tr.AddMetadata("ExtID."+k.(string), v.(string))
	}

	// vlan tag
	if tag, ok := row.New.Fields["tag"]; ok {
		switch tag.(type) {
		case libovsdb.OvsSet:
			set := tag.(libovsdb.OvsSet)
			if len(set.GoSet) > 0 {
				var vlans []int64
				for _, vlan := range set.GoSet {
					if vlan, ok := vlan.(float64); ok {
						vlans = append(vlans, int64(vlan))
					}
				}
				if len(vlans) > 0 {
					tr.AddMetadata("Vlans", vlans)
				}
			}
		case float64:
			tr.AddMetadata("Vlans", int64(tag.(float64)))
		}
	}

	switch row.New.Fields["interfaces"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["interfaces"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUUID
			o.intfToPort[u] = port

			if intf, ok := o.uuidToIntf[u]; ok {
				o.portToIntf[uuid] = intf

				if !topology.HaveLayer2Link(o.Graph, port, intf, nil) {
					topology.AddLayer2Link(o.Graph, port, intf, nil)
				}
			}
		}
	case libovsdb.UUID:
		u := row.New.Fields["interfaces"].(libovsdb.UUID).GoUUID
		o.intfToPort[u] = port

		if intf, ok := o.uuidToIntf[u]; ok {
			o.portToIntf[uuid] = intf

			if !topology.HaveLayer2Link(o.Graph, port, intf, nil) {
				topology.AddLayer2Link(o.Graph, port, intf, nil)
			}
		}
	}

	if bridge, ok := o.portToBridge[uuid]; ok {
		if !topology.HaveOwnershipLink(o.Graph, bridge, port, nil) {
			topology.AddOwnershipLink(o.Graph, bridge, port, nil)
			topology.AddLayer2Link(o.Graph, bridge, port, nil)
		}

		if intf, ok := o.portToIntf[uuid]; ok && isOvsLogicalInterface(intf) {
			if !topology.IsOwnershipLinked(o.Graph, intf) {
				topology.AddOwnershipLink(o.Graph, bridge, intf, nil)
			}
		}
	}
}

// OnOvsPortUpdate event
func (o *OvsdbProbe) OnOvsPortUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsPortAdd(monitor, uuid, row)
}

// OnOvsPortDel event
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
	delete(o.portToBridge, uuid)
	delete(o.portToIntf, uuid)
}

// Start the probe
func (o *OvsdbProbe) Start() {
	o.OvsMon.StartMonitoring()
}

// Stop the probe
func (o *OvsdbProbe) Stop() {
	o.OvsMon.StopMonitoring()
}

// NewOvsdbProbe creates a new graph OVS database probe
func NewOvsdbProbe(g *graph.Graph, n *graph.Node, p string, t string) *OvsdbProbe {
	mon := ovsdb.NewOvsMonitor(p, t)
	mon.ExcludeColumn("*", "statistics")
	mon.IncludeColumn("Interface", "statistics")

	o := &OvsdbProbe{
		Graph:        g,
		Root:         n,
		uuidToIntf:   make(map[string]*graph.Node),
		uuidToPort:   make(map[string]*graph.Node),
		intfToPort:   make(map[string]*graph.Node),
		portToIntf:   make(map[string]*graph.Node),
		portToBridge: make(map[string]*graph.Node),
		OvsMon:       mon,
		OvsOfProbe:   NewOvsOfProbe(g, n, mon.Target),
	}
	o.OvsMon.AddMonitorHandler(o)

	return o
}

// NewOvsdbProbeFromConfig creates a new probe based on configuration
func NewOvsdbProbeFromConfig(g *graph.Graph, n *graph.Node) *OvsdbProbe {
	address := config.GetString("ovs.ovsdb")

	var protocol string
	var target string

	if strings.HasPrefix(address, "unix://") {
		target = strings.TrimPrefix(address, "unix://")
		protocol = "unix"
	} else if strings.HasPrefix(address, "tcp://") {
		target = strings.TrimPrefix(address, "tcp://")
		protocol = "tcp"
	} else {
		// fallback to the original address format addr:port
		sa, err := common.ServiceAddressFromString("ovs.ovsdb")
		if err != nil {
			logging.GetLogger().Errorf("Configuration error: %s", err.Error())
			return nil
		}

		protocol = "tcp"
		target = fmt.Sprintf("%s:%d", sa.Addr, sa.Port)
	}

	return NewOvsdbProbe(g, n, protocol, target)
}
