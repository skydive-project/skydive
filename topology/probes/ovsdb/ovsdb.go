//go:generate go run github.com/skydive-project/skydive/scripts/gendecoder -package github.com/skydive-project/skydive/topology/probes/ovsdb
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package ovsdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/socketplane/libovsdb"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/ovs/ovsdb"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	tp "github.com/skydive-project/skydive/topology/probes"
)

var (
	patchMetadata = graph.Metadata{"RelationType": "layer2", "Type": "patch"}
)

// Probe describes a probe that reads OVS database and updates the graph
type Probe struct {
	sync.Mutex
	Ctx          tp.Context
	OvsMon       *ovsdb.OvsMonitor
	Handler      *OvsOfProbeHandler
	uuidToIntf   map[string]*graph.Node
	uuidToPort   map[string]*graph.Node
	intfToPort   map[string]*graph.Node
	portToIntf   map[string]*graph.Node
	portToBridge map[string]*graph.Node
	cancelFunc   context.CancelFunc
	enableStats  bool
}

// OvsMetadata describe Ovs metadata sub section
// easyjson:json
// gendecoder
type OvsMetadata struct {
	OtherConfig      map[string]string         `json:"OtherConfig,omitempty"`
	Options          map[string]string         `json:"Options,omitempty"`
	Protocols        []string                  `json:"Protocols,omitempty"`
	DBVersion        string                    `json:"DBVersion,omitempty"`
	Version          string                    `json:"Version,omitempty"`
	Error            string                    `json:"Error,omitempty"`
	Metric           *topology.InterfaceMetric `json:"Metric,omitempty"`
	LastUpdateMetric *topology.InterfaceMetric `json:"LastUpdateMetric,omitempty"`
}

// OvsMetadataDecoder implements a json message raw decoder
func OvsMetadataDecoder(raw json.RawMessage) (common.Getter, error) {
	var ovs OvsMetadata
	if err := json.Unmarshal(raw, &ovs); err != nil {
		return nil, fmt.Errorf("unable to unmarshal ovs metadata %s: %s", string(raw), err)
	}

	return &ovs, nil
}

func isOvsInterfaceType(t string) bool {
	switch t {
	case "dpdk", "dpdkvhostuserclient", "patch", "internal":
		return true
	}

	return false
}

func isOvsDrivenInterface(intf *graph.Node) bool {
	if d, _ := intf.GetFieldString("Driver"); d == "openvswitch" {
		return true
	}

	t, _ := intf.GetFieldString("Type")
	return isOvsInterfaceType(t)
}

// OnConnected event
func (o *Probe) OnConnected(monitor *ovsdb.OvsMonitor) {
}

// OnDisconnected event
func (o *Probe) OnDisconnected(monitor *ovsdb.OvsMonitor) {
}

// OnOvsBridgeUpdate event
func (o *Probe) OnOvsBridgeUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsBridgeAdd(monitor, uuid, row)
}

// OnOvsBridgeAdd event
func (o *Probe) OnOvsBridgeAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	name := row.New.Fields["name"].(string)

	o.Ctx.Graph.Lock()
	defer o.Ctx.Graph.Unlock()

	bridge := o.Ctx.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if bridge == nil {
		var err error

		bridge, err = o.Ctx.Graph.NewNode(graph.GenID(), graph.Metadata{"Name": name, "UUID": uuid, "Type": "ovsbridge"})
		if err != nil {
			o.Ctx.Logger.Error(err)
			return
		}
		topology.AddOwnershipLink(o.Ctx.Graph, o.Ctx.RootNode, bridge, nil)
	}

	ovsMetadata := &OvsMetadata{
		OtherConfig: make(map[string]string),
	}

	otherConfig := row.New.Fields["other_config"].(libovsdb.OvsMap)
	for k, v := range otherConfig.GoMap {
		ovsMetadata.OtherConfig[k.(string)] = v.(string)
	}

	if protocolSet, ok := row.New.Fields["protocols"].(libovsdb.OvsSet); ok {
		ovsMetadata.Protocols = make([]string, len(protocolSet.GoSet))
		for i, protocol := range protocolSet.GoSet {
			ovsMetadata.Protocols[i] = protocol.(string)
		}

	}

	tr := o.Ctx.Graph.StartMetadataTransaction(bridge)
	tr.AddMetadata("Ovs", ovsMetadata)

	extIds := row.New.Fields["external_ids"].(libovsdb.OvsMap)
	for k, v := range extIds.GoMap {
		tr.AddMetadata("ExtID."+k.(string), v.(string))
	}
	tr.Commit()

	switch row.New.Fields["ports"].(type) {
	case libovsdb.OvsSet:
		set := row.New.Fields["ports"].(libovsdb.OvsSet)

		for _, i := range set.GoSet {
			u := i.(libovsdb.UUID).GoUUID
			o.portToBridge[u] = bridge

			if port, ok := o.uuidToPort[u]; ok {
				if !topology.HaveOwnershipLink(o.Ctx.Graph, bridge, port) {
					topology.AddOwnershipLink(o.Ctx.Graph, bridge, port, nil)
					topology.AddLayer2Link(o.Ctx.Graph, bridge, port, nil)
				}

				if intf, ok := o.portToIntf[uuid]; ok {
					o.linkIntfTOBridge(bridge, intf)
				}
			}
		}

	case libovsdb.UUID:
		u := row.New.Fields["ports"].(libovsdb.UUID).GoUUID
		o.portToBridge[u] = bridge

		if port, ok := o.uuidToPort[u]; ok {
			if !topology.HaveOwnershipLink(o.Ctx.Graph, bridge, port) {
				topology.AddOwnershipLink(o.Ctx.Graph, bridge, port, nil)
				topology.AddLayer2Link(o.Ctx.Graph, bridge, port, nil)
			}
		}
	}
	if o.Handler != nil {
		o.Handler.OnOvsBridgeAdd(bridge)
	}
}

// OnOvsBridgeDel event
func (o *Probe) OnOvsBridgeDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	if o.Handler != nil {
		o.Handler.OnOvsBridgeDel(uuid)
	}
	o.Ctx.Graph.Lock()
	defer o.Ctx.Graph.Unlock()
	bridge := o.Ctx.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if bridge != nil {
		if err := o.Ctx.Graph.DelNode(bridge); err != nil {
			o.Ctx.Logger.Error(err)
		}
	}
}

// linkIntfTOBridge having ifindex set to 0 (not handled by netlink) or being in
// error
func (o *Probe) linkIntfTOBridge(bridge, intf *graph.Node) {
	if isOvsDrivenInterface(intf) && !topology.IsOwnershipLinked(o.Ctx.Graph, intf) {
		topology.AddOwnershipLink(o.Ctx.Graph, bridge, intf, nil)
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

func columnStringValue(row *libovsdb.Row, col string) string {
	var value string

	if c, ok := row.Fields[col]; ok {
		switch row.Fields[col].(type) {
		case string:
			value = c.(string)
		case libovsdb.OvsSet:
			set := row.Fields[col].(libovsdb.OvsSet)
			if len(set.GoSet) > 0 {
				value = set.GoSet[0].(string)
			}
		}
	}

	return value
}

func goMapStringValue(row *libovsdb.Row, col string, subcol string) string {
	var value string

	if c, ok := row.Fields[col]; ok {
		switch c.(type) {
		case libovsdb.OvsMap:
			m := c.(libovsdb.OvsMap)
			if v, ok := m.GoMap[subcol]; ok {
				if vs, ok := v.(string); ok {
					value = vs
				}
			}
		}
	}

	return value
}

func columnInt64Value(row *libovsdb.Row, col string) int64 {
	var value int64

	if c, ok := row.Fields[col]; ok {
		switch row.Fields[col].(type) {
		case float64:
			value = int64(c.(float64))
		case libovsdb.OvsSet:
			set := row.Fields[col].(libovsdb.OvsSet)
			if len(set.GoSet) > 0 {
				switch set.GoSet[0].(type) {
				case int64:
					value = set.GoSet[0].(int64)
				case float64:
					value = int64(set.GoSet[0].(float64))

				}
			}
		}
	}

	return value
}

// OnOvsInterfaceAdd event
func (o *Probe) OnOvsInterfaceAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	name := columnStringValue(&row.New, "name")
	oerror := columnStringValue(&row.New, "error")
	ofport := columnInt64Value(&row.New, "ofport")
	mac := columnStringValue(&row.New, "mac_in_use")
	ifindex := columnInt64Value(&row.New, "ifindex")
	itype := columnStringValue(&row.New, "type")
	attachedMAC := goMapStringValue(&row.New, "external_ids", "attached-mac")

	o.Ctx.Graph.Lock()
	defer o.Ctx.Graph.Unlock()

	intf := o.Ctx.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if mac != "" || attachedMAC != "" {
		var macFilter *filters.Filter
		if mac != "" {
			macFilter = filters.NewTermStringFilter("MAC", mac)
		} else {
			macFilter = filters.NewTermStringFilter("MAC", attachedMAC)
		}
		andFilters := []*filters.Filter{
			filters.NewTermStringFilter("Name", name),
			macFilter,
		}

		if ifindex > 0 {
			andFilters = append(andFilters, filters.NewTermInt64Filter("IfIndex", ifindex))
		}

		andFilter := filters.NewAndFilter(andFilters...)

		if intf == nil {
			// no already inserted ovs interface but maybe already detected by netlink
			intf = o.Ctx.Graph.LookupFirstNode(graph.NewElementFilter(andFilter))
		} else {
			// if there is a interface with the same MAC, name and optionally
			// the same ifindex but having another ID, it means that ovs and
			// netlink have seen the same interface. In order to keep only
			// one interface we delete the ovs one and use the netlink one.
			if nintf := o.Ctx.Graph.LookupFirstNode(graph.NewElementFilter(andFilter)); nintf != nil && intf.ID != nintf.ID {
				if err := o.Ctx.Graph.DelNode(intf); err != nil {
					o.Ctx.Logger.Error(err)
				}
				intf = nintf
			}
		}
	}

	if intf == nil {
		var err error

		intf, err = o.Ctx.Graph.NewNode(graph.GenID(), graph.Metadata{"Name": name, "UUID": uuid})
		if err != nil {
			o.Ctx.Logger.Error(err)
			return
		}
	}

	var ovsMetadata OvsMetadata
	if ovs, err := intf.GetField("Ovs"); err == nil {
		// need to recopy metric as it is used to get the last update metric
		// and LastUpdateMetric as it could have been zero between two update
		ovsMetadata.Metric = ovs.(*OvsMetadata).Metric
		ovsMetadata.LastUpdateMetric = ovs.(*OvsMetadata).LastUpdateMetric
	}
	ovsMetadata.Options = make(map[string]string)
	ovsMetadata.OtherConfig = make(map[string]string)

	ovsMetadata.Error = oerror

	tr := o.Ctx.Graph.StartMetadataTransaction(intf)
	defer tr.Commit()

	tr.AddMetadata("UUID", uuid)

	driver := goMapStringValue(&row.New, "status", "driver_name")
	if driver == "" {
		driver = "openvswitch"
	}
	if _, err := intf.GetFieldString("Driver"); err != nil {
		tr.AddMetadata("Driver", driver)
	}

	if ofport != 0 {
		tr.AddMetadata("OfPort", ofport)
	}

	if ifindex > 0 {
		tr.AddMetadata("IfIndex", ifindex)
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
		ovsMetadata.Options[k.(string)] = v.(string)
	}

	otherConfig := row.New.Fields["other_config"].(libovsdb.OvsMap)
	for k, v := range otherConfig.GoMap {
		ovsMetadata.OtherConfig[k.(string)] = v.(string)
	}

	o.uuidToIntf[uuid] = intf

	switch itype {
	case "gre", "vxlan", "geneve":
		if ip := goMapStringValue(&row.New, "options", "local_ip"); ip != "" {
			tr.AddMetadata("LocalIP", ip)
		}
		if ip := goMapStringValue(&row.New, "options", "remote_ip"); ip != "" {
			tr.AddMetadata("RemoteIP", ip)
		}

		if iface := goMapStringValue(&row.New, "status", "tunnel_egress_iface"); iface != "" {
			tr.AddMetadata("TunEgressIface", iface)
		}
		if iface := goMapStringValue(&row.New, "status", "tunnel_egress_iface_carrier"); iface != "" {
			tr.AddMetadata("TunEgressIfaceCarrier", iface)
		}

	case "patch":
		if peerName := goMapStringValue(&row.New, "options", "peer"); peerName != "" {
			peer := o.Ctx.Graph.LookupFirstNode(graph.Metadata{"Name": peerName, "Type": "patch"})
			if peer != nil {
				if !topology.HaveLayer2Link(o.Ctx.Graph, intf, peer) {
					topology.AddLayer2Link(o.Ctx.Graph, intf, peer, patchMetadata)
				}
			} else {
				// lookup in the intf queue
				for _, peer := range o.uuidToIntf {
					if name, _ := peer.GetFieldString("Name"); name == peerName && !topology.HaveLayer2Link(o.Ctx.Graph, intf, peer) {
						topology.AddLayer2Link(o.Ctx.Graph, intf, peer, patchMetadata)
					}
				}
			}
		}
	}

	if field, ok := row.New.Fields["statistics"]; ok && o.enableStats {
		now := int64(common.UnixMillis(time.Now()))

		statistics := field.(libovsdb.OvsMap)
		currMetric := newInterfaceMetricsFromOVSDB(statistics)
		currMetric.Last = now

		var prevMetric, lastUpdateMetric *topology.InterfaceMetric

		if ovsMetadata.Metric != nil {
			prevMetric = ovsMetadata.Metric
			lastUpdateMetric = currMetric.Sub(prevMetric).(*topology.InterfaceMetric)
		}

		ovsMetadata.Metric = currMetric

		// nothing changed since last update
		if lastUpdateMetric != nil && !lastUpdateMetric.IsZero() {
			lastUpdateMetric.Start = prevMetric.Last
			lastUpdateMetric.Last = now

			ovsMetadata.LastUpdateMetric = lastUpdateMetric
		}
	}

	if port, ok := o.intfToPort[uuid]; ok {
		if !topology.HaveLayer2Link(o.Ctx.Graph, port, intf) {
			topology.AddLayer2Link(o.Ctx.Graph, port, intf, nil)
		}

		puuid, _ := port.GetFieldString("UUID")
		if bridge, ok := o.portToBridge[puuid]; ok {
			o.linkIntfTOBridge(bridge, intf)
		}
	}

	tr.AddMetadata("Ovs", &ovsMetadata)
}

// OnOvsInterfaceUpdate event
func (o *Probe) OnOvsInterfaceUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsInterfaceAdd(monitor, uuid, row)
}

// OnOvsInterfaceDel event
func (o *Probe) OnOvsInterfaceDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	intf, ok := o.uuidToIntf[uuid]
	if !ok {
		return
	}

	o.Ctx.Graph.Lock()
	defer o.Ctx.Graph.Unlock()

	// do not delete if not an openvswitch interface
	if driver, _ := intf.GetFieldString("Driver"); driver == "openvswitch" {
		if err := o.Ctx.Graph.DelNode(intf); err != nil && err != graph.ErrNodeNotFound {
			o.Ctx.Logger.Error(err)
		}
	}

	delete(o.uuidToIntf, uuid)
	delete(o.intfToPort, uuid)
}

// OnOvsPortAdd event
func (o *Probe) OnOvsPortAdd(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	o.Ctx.Graph.Lock()
	defer o.Ctx.Graph.Unlock()

	port, ok := o.uuidToPort[uuid]
	if !ok {
		var err error

		port, err = o.Ctx.Graph.NewNode(graph.GenID(), graph.Metadata{
			"UUID": uuid,
			"Name": row.New.Fields["name"].(string),
			"Type": "ovsport",
		})

		if err != nil {
			o.Ctx.Logger.Error(err)
			return
		}

		o.uuidToPort[uuid] = port
	}

	tr := o.Ctx.Graph.StartMetadataTransaction(port)
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

	ovsMetadata := &OvsMetadata{
		OtherConfig: make(map[string]string),
	}

	otherConfig := row.New.Fields["other_config"].(libovsdb.OvsMap)
	for k, v := range otherConfig.GoMap {
		ovsMetadata.OtherConfig[k.(string)] = v.(string)
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

				if !topology.HaveLayer2Link(o.Ctx.Graph, port, intf) {
					topology.AddLayer2Link(o.Ctx.Graph, port, intf, nil)
				}
			}
		}
	case libovsdb.UUID:
		u := row.New.Fields["interfaces"].(libovsdb.UUID).GoUUID
		o.intfToPort[u] = port

		if intf, ok := o.uuidToIntf[u]; ok {
			o.portToIntf[uuid] = intf

			if !topology.HaveLayer2Link(o.Ctx.Graph, port, intf) {
				topology.AddLayer2Link(o.Ctx.Graph, port, intf, nil)
			}
		}
	}

	if bridge, ok := o.portToBridge[uuid]; ok {
		if !topology.HaveOwnershipLink(o.Ctx.Graph, bridge, port) {
			topology.AddOwnershipLink(o.Ctx.Graph, bridge, port, nil)
			topology.AddLayer2Link(o.Ctx.Graph, bridge, port, nil)
		}

		if intf, ok := o.portToIntf[uuid]; ok {
			o.linkIntfTOBridge(bridge, intf)
		}
	}

	tr.AddMetadata("Ovs", ovsMetadata)
}

// OnOvsPortUpdate event
func (o *Probe) OnOvsPortUpdate(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.OnOvsPortAdd(monitor, uuid, row)
}

// OnOvsPortDel event
func (o *Probe) OnOvsPortDel(monitor *ovsdb.OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
	o.Lock()
	defer o.Unlock()

	port, ok := o.uuidToPort[uuid]
	if !ok {
		return
	}

	o.Ctx.Graph.Lock()
	defer o.Ctx.Graph.Unlock()

	if err := o.Ctx.Graph.DelNode(port); err != nil {
		o.Ctx.Logger.Error(err)
	}

	delete(o.uuidToPort, uuid)
	delete(o.portToBridge, uuid)
	delete(o.portToIntf, uuid)
}

// OnOvsUpdate event
func (o *Probe) OnOvsUpdate(monitor *ovsdb.OvsMonitor, row *libovsdb.RowUpdate) {
	// retry as for the first bridge created the interface can be seen by netlink before the
	// db update
	retryFn := func() error {
		o.Ctx.Graph.Lock()
		defer o.Ctx.Graph.Unlock()

		ovsSys := o.Ctx.Graph.LookupFirstChild(o.Ctx.RootNode, graph.Metadata{"Name": "ovs-system", "Type": "openvswitch"})
		if ovsSys == nil {
			return errors.New("ovs-system not found")
		}

		tr := o.Ctx.Graph.StartMetadataTransaction(ovsSys)
		defer tr.Commit()

		dbVersion := columnStringValue(&row.New, "db_version")
		ovsVersion := columnStringValue(&row.New, "ovs_version")

		ovsMetadata := &OvsMetadata{
			OtherConfig: make(map[string]string),
			Version:     ovsVersion,
			DBVersion:   dbVersion,
		}

		otherConfig := row.New.Fields["other_config"].(libovsdb.OvsMap)
		for k, v := range otherConfig.GoMap {
			ovsMetadata.OtherConfig[k.(string)] = v.(string)
		}
		tr.AddMetadata("Ovs", ovsMetadata)

		extIds := row.New.Fields["external_ids"].(libovsdb.OvsMap)
		for k, v := range extIds.GoMap {
			tr.AddMetadata("ExtID."+k.(string), v.(string))
		}

		return nil
	}
	go retry.Do(retryFn, retry.Attempts(8), retry.Delay(10*time.Millisecond))
}

// Start the probe
func (o *Probe) Start() error {
	o.OvsMon.AddMonitorHandler(o)
	o.OvsMon.StartMonitoring()
	return nil
}

// Stop the probe
func (o *Probe) Stop() {
	o.OvsMon.StopMonitoring()
	o.cancelFunc()
}

// NewProbe returns a new OVSDB topology probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	address := ctx.Config.GetString("ovs.ovsdb")
	enableStats := ctx.Config.GetBool("ovs.enable_stats")

	u, err := url.Parse(address)
	if err != nil {
		if u, err = url.Parse("tcp://" + address); err != nil {
			return nil, err
		}
	}

	var target string
	if u.Scheme == "unix" {
		target = u.Path
	} else {
		target = u.Host
	}

	mon := ovsdb.NewOvsMonitor(u.Scheme, target)
	mon.ExcludeColumn("*", "statistics")
	mon.ExcludeColumn("Port", "rstp_statistics")
	if enableStats {
		mon.IncludeColumn("Interface", "statistics")
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	ofHandler, err := NewOvsOfProbeHandler(cancelCtx, ctx, mon.Target, u.Scheme)
	if err != nil {
		cancelFunc()
		return nil, err
	}

	return &Probe{
		Ctx:          ctx,
		uuidToIntf:   make(map[string]*graph.Node),
		uuidToPort:   make(map[string]*graph.Node),
		intfToPort:   make(map[string]*graph.Node),
		portToIntf:   make(map[string]*graph.Node),
		portToBridge: make(map[string]*graph.Node),
		OvsMon:       mon,
		Handler:      ofHandler,
		cancelFunc:   cancelFunc,
		enableStats:  enableStats,
	}, nil
}

// Register registers graph metadata decoders
func Register() {
	graph.NodeMetadataDecoders["Ovs"] = OvsMetadataDecoder
}
