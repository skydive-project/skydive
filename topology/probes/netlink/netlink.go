// +build linux

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

package netlink

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/safchain/ethtool"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	maxEpollEvents = 32
)

var flagNames = []string{
	"UP",
	"BROADCAST",
	"LOOPBACK",
	"POINTTOPOINT",
	"MULTICAST",
}

type pendingLink struct {
	ID       graph.Identifier
	Metadata graph.Metadata
}

// Neighbor describes a member of the forwarding database
// easyjson:json
type Neighbor struct {
	Flags   []string `json:"Flags,omitempty"`
	MAC     string
	IP      string   `json:"IP,omitempty"`
	State   []string `json:"State,omitempty"`
	Vlan    int64    `json:"Vlan,omitempty"`
	VNI     int64    `json:"VNI,omitempty"`
	IfIndex int64
}

// NetNsProbe describes a topology probe based on netlink in a network namespace
type NetNsProbe struct {
	common.RWMutex
	Graph                *graph.Graph
	Root                 *graph.Node
	NsPath               string
	epollFd              int
	ethtool              *ethtool.Ethtool
	handle               *netlink.Handle
	socket               *nl.NetlinkSocket
	indexToChildrenQueue map[int64][]pendingLink
	links                map[string]*graph.Node
	state                int64
	wg                   sync.WaitGroup
	quit                 chan bool
}

// Probe describes a list NetLink NameSpace probe to enhance the graph
type Probe struct {
	common.RWMutex
	Graph   *graph.Graph
	epollFd int
	probes  map[int32]*NetNsProbe
	state   int64
	wg      sync.WaitGroup
}

func (u *NetNsProbe) linkPendingChildren(intf *graph.Node, index int64) {
	// ignore ovs-system interface as it doesn't make any sense according to
	// the following thread:
	// http://openvswitch.org/pipermail/discuss/2013-October/011657.html
	if name, _ := intf.GetFieldString("Name"); name == "ovs-system" {
		return
	}

	// add children of this interface that was previously added
	if children, ok := u.indexToChildrenQueue[index]; ok {
		for _, link := range children {
			child := u.Graph.GetNode(link.ID)
			if child != nil {
				topology.AddLayer2Link(u.Graph, intf, child, link.Metadata)
			}
		}
		delete(u.indexToChildrenQueue, index)
	}
}

func (u *NetNsProbe) linkIntfToIndex(intf *graph.Node, index int64, m graph.Metadata) {
	// assuming we have only one master with this index
	parent := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if parent != nil {
		// ignore ovs-system interface as it doesn't make any sense according to
		// the following thread:
		// http://openvswitch.org/pipermail/discuss/2013-October/011657.html
		if name, _ := parent.GetFieldString("Name"); name == "ovs-system" {
			return
		}

		if !topology.HaveLayer2Link(u.Graph, parent, intf) {
			topology.AddLayer2Link(u.Graph, parent, intf, m)
		}
	} else {
		// not yet the bridge so, enqueue for a later add
		u.indexToChildrenQueue[index] = append(u.indexToChildrenQueue[index], pendingLink{ID: intf.ID, Metadata: m})
	}
}

func (u *NetNsProbe) handleIntfIsChild(intf *graph.Node, link netlink.Link) {
	// handle pending relationship
	u.linkPendingChildren(intf, int64(link.Attrs().Index))

	// interface being a part of a bridge
	if link.Attrs().MasterIndex != 0 {
		u.linkIntfToIndex(intf, int64(link.Attrs().MasterIndex), nil)
	}

	if link.Attrs().ParentIndex != 0 {
		if _, err := intf.GetFieldInt64("Vlan"); err == nil {
			u.linkIntfToIndex(intf, int64(int64(link.Attrs().ParentIndex)), graph.Metadata{"Type": "vlan"})
		}
	}
}

func (u *NetNsProbe) handleIntfIsVeth(intf *graph.Node, link netlink.Link) {
	if link.Type() != "veth" {
		return
	}

	ifIndex, err := intf.GetFieldInt64("IfIndex")
	if err != nil {
		return
	}

	linkMetadata := graph.Metadata{"Type": "veth"}

	if peerIndex, err := intf.GetFieldInt64("PeerIfIndex"); err == nil {
		peerResolver := func(root *graph.Node) error {
			u.Graph.Lock()
			defer u.Graph.Unlock()

			// re get the interface from the graph since the interface could have been deleted
			if u.Graph.GetNode(intf.ID) == nil {
				return errors.New("Node not found")
			}

			var peer *graph.Node
			if root == nil {
				peer = u.Graph.LookupFirstNode(graph.Metadata{"IfIndex": peerIndex, "Type": "veth"})
			} else {
				peer = u.Graph.LookupFirstChild(root, graph.Metadata{"IfIndex": peerIndex, "Type": "veth"})
			}
			if peer == nil {
				return errors.New("Peer not found")
			}
			if !topology.HaveLayer2Link(u.Graph, peer, intf) {
				topology.AddLayer2Link(u.Graph, peer, intf, linkMetadata)
			}

			return nil
		}

		if peerIndex > ifIndex {
			go func() {
				// lookup first in the local namespace then in the whole graph
				// since we can have the exact same interface (name/index) in different namespaces
				// we always take first the closer one.
				localFnc := func() error {
					if u.isRunning() == false {
						return nil
					}
					return peerResolver(u.Root)
				}
				if err := common.Retry(localFnc, 10, 100*time.Millisecond); err != nil {
					peerResolver(nil)
				}
			}()
		}
	}
}

func (u *NetNsProbe) addGenericLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	index := int64(link.Attrs().Index)

	var intf *graph.Node
	intf = u.Graph.LookupFirstChild(u.Root, graph.Metadata{
		"IfIndex": index,
	})

	if intf == nil {
		intf = u.Graph.NewNode(graph.GenID(), m)
	}

	if !topology.HaveOwnershipLink(u.Graph, u.Root, intf) {
		topology.AddOwnershipLink(u.Graph, u.Root, intf, nil)
	}

	return intf
}

func (u *NetNsProbe) addBridgeLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	index := int64(link.Attrs().Index)
	intf := u.addGenericLinkToTopology(link, m)

	u.linkPendingChildren(intf, index)

	return intf
}

func (u *NetNsProbe) addOvsLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	name := link.Attrs().Name
	attrs := link.Attrs()

	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Name", name),
		filters.NewOrFilter(
			filters.NewTermStringFilter("MAC", attrs.HardwareAddr.String()),
			filters.NewTermStringFilter("ExtID.attached-mac", attrs.HardwareAddr.String()),
		),
		filters.NewNotNullFilter("UUID"),
	)

	intf := u.Graph.LookupFirstNode(graph.NewElementFilter(filter))
	if intf != nil {
		if !topology.HaveOwnershipLink(u.Graph, u.Root, intf) {
			topology.AddOwnershipLink(u.Graph, u.Root, intf, nil)
		}
	}

	return intf
}

func (u *NetNsProbe) getLinkIPs(link netlink.Link, family int) (ips []string) {
	addrs, err := u.handle.AddrList(link, family)
	if err != nil {
		return
	}

	for _, addr := range addrs {
		ips = append(ips, addr.IPNet.String())
	}

	return
}

var neighStates = []string{
	"NUD_INCOMPLETE",
	"NUD_REACHABLE",
	"NUD_STALE",
	"NUD_DELAY",
	"NUD_PROBE",
	"NUD_FAILED",
	"NUD_NOARP",
	"NUD_PERMANENT",
	"NUD_NONE",
}

var neighFlags = []string{
	"NTF_USE",
	"NTF_SELF",
	"NTF_MASTER",
	"NTF_PROXY",
	"NTF_EXT_LEARNED",
	"NTF_OFFLOADED",
	"NTF_ROUTER",
}

func getFlagsString(flags []string, state int) (a []string) {
	for i, s := range flags {
		if state&(1<<uint(i)) != 0 {
			a = append(a, s)
		}
	}
	return
}

func (u *NetNsProbe) getNeighbors(index, family int) (neighbors []Neighbor) {
	neighList, err := u.handle.NeighList(index, family)
	if err == nil && len(neighList) > 0 {
		for i, neigh := range neighList {
			neighbors = append(neighbors, Neighbor{
				Flags:   getFlagsString(neighFlags, neigh.Flags),
				MAC:     neigh.HardwareAddr.String(),
				State:   getFlagsString(neighStates, neigh.State),
				IfIndex: int64(neigh.LinkIndex),
				Vlan:    int64(neigh.Vlan),
				VNI:     int64(neigh.VNI),
			})
			if neigh.IP != nil {
				neighbors[i].IP = neigh.IP.String()
			}
		}
	}
	return
}

func newInterfaceMetricsFromNetlink(link netlink.Link) *topology.InterfaceMetric {
	statistics := link.Attrs().Statistics
	if statistics == nil {
		return nil
	}

	return &topology.InterfaceMetric{
		Collisions:        int64(statistics.Collisions),
		Multicast:         int64(statistics.Multicast),
		RxBytes:           int64(statistics.RxBytes),
		RxCompressed:      int64(statistics.RxCompressed),
		RxCrcErrors:       int64(statistics.RxCrcErrors),
		RxDropped:         int64(statistics.RxDropped),
		RxErrors:          int64(statistics.RxErrors),
		RxFifoErrors:      int64(statistics.RxFifoErrors),
		RxFrameErrors:     int64(statistics.RxFrameErrors),
		RxLengthErrors:    int64(statistics.RxLengthErrors),
		RxMissedErrors:    int64(statistics.RxMissedErrors),
		RxOverErrors:      int64(statistics.RxOverErrors),
		RxPackets:         int64(statistics.RxPackets),
		TxAbortedErrors:   int64(statistics.TxAbortedErrors),
		TxBytes:           int64(statistics.TxBytes),
		TxCarrierErrors:   int64(statistics.TxCarrierErrors),
		TxCompressed:      int64(statistics.TxCompressed),
		TxDropped:         int64(statistics.TxDropped),
		TxErrors:          int64(statistics.TxErrors),
		TxFifoErrors:      int64(statistics.TxFifoErrors),
		TxHeartbeatErrors: int64(statistics.TxHeartbeatErrors),
		TxPackets:         int64(statistics.TxPackets),
		TxWindowErrors:    int64(statistics.TxWindowErrors),
	}
}

func (u *NetNsProbe) addLinkToTopology(link netlink.Link) {
	driver, _ := u.ethtool.DriverName(link.Attrs().Name)
	if driver == "" && link.Type() == "bridge" {
		driver = "bridge"
	}

	attrs := link.Attrs()
	linkType := link.Type()

	// force the veth type when driver if veth as a veth in bridge can have device type
	if driver == "veth" {
		linkType = "veth"
	}

	metadata := graph.Metadata{
		"Name":      attrs.Name,
		"Type":      linkType,
		"EncapType": attrs.EncapType,
		"IfIndex":   int64(attrs.Index),
		"Driver":    driver,
	}

	if attrs.MTU != 0 {
		metadata["MTU"] = int64(attrs.MTU)
	}

	if mac := attrs.HardwareAddr.String(); mac != "" {
		metadata["MAC"] = mac
	}

	if attrs.MasterIndex != 0 {
		metadata["MasterIndex"] = int64(attrs.MasterIndex)
	}

	if attrs.ParentIndex != 0 {
		metadata["ParentIndex"] = int64(attrs.ParentIndex)
	}

	if speed, err := u.ethtool.CmdGet(&ethtool.EthtoolCmd{}, attrs.Name); err == nil {
		if speed != math.MaxUint32 {
			metadata["Speed"] = int64(speed)
		}
	}

	if neighbors := u.getNeighbors(attrs.Index, syscall.AF_BRIDGE); len(neighbors) > 0 {
		metadata["FDB"] = neighbors
	}

	neighbors := u.getNeighbors(attrs.Index, syscall.AF_INET)
	neighbors = append(neighbors, u.getNeighbors(attrs.Index, syscall.AF_INET6)...)
	if len(neighbors) > 0 {
		metadata["Neighbors"] = neighbors
	}

	if rt := u.getRoutingTable(link, syscall.RTA_UNSPEC); rt != nil {
		metadata["RoutingTable"] = rt
	}

	if metric := newInterfaceMetricsFromNetlink(link); metric != nil {
		metadata["Metric"] = metric
	}

	if linkType == "veth" {
		stats, err := u.ethtool.Stats(attrs.Name)
		if err != nil && err != syscall.ENODEV {
			logging.GetLogger().Errorf("Unable get stats from ethtool (%s): %s", attrs.Name, err.Error())
		} else if index, ok := stats["peer_ifindex"]; ok {
			metadata["PeerIfIndex"] = int64(index)
		}
	}

	ipv4 := u.getLinkIPs(link, netlink.FAMILY_V4)
	if len(ipv4) > 0 {
		metadata["IPV4"] = ipv4
	}

	ipv6 := u.getLinkIPs(link, netlink.FAMILY_V6)
	if len(ipv6) > 0 {
		metadata["IPV6"] = ipv6
	}

	if vlan, ok := link.(*netlink.Vlan); ok {
		if vlan, err := common.ToInt64(vlan.VlanId); err == nil {
			metadata["Vlan"] = vlan
		}
	}

	metadata["State"] = strings.ToUpper(attrs.OperState.String())

	var flags []string
	for i, name := range flagNames {
		if attrs.RawFlags&(1<<uint(i)) != 0 {
			flags = append(flags, name)
		}
	}
	metadata["LinkFlags"] = flags

	if link.Type() == "bond" {
		metadata["BondMode"] = link.(*netlink.Bond).Mode.String()
	}

	var intf *graph.Node

	switch driver {
	case "bridge":
		intf = u.addBridgeLinkToTopology(link, metadata)
	default:
		intf = u.addOvsLinkToTopology(link, metadata)
		// always prefer Type from ovs
		if intf != nil {
			if tp, _ := intf.GetFieldString("Type"); tp != "" {
				metadata["Type"] = tp
			}
		} else {
			intf = u.addGenericLinkToTopology(link, metadata)
		}
	}

	if intf == nil {
		return
	}

	u.Lock()
	u.links[attrs.Name] = intf
	u.Unlock()

	// merge metadata
	tr := u.Graph.StartMetadataTransaction(intf)
	for k, v := range metadata {
		tr.AddMetadata(k, v)
	}
	tr.Commit()

	u.handleIntfIsChild(intf, link)
	u.handleIntfIsVeth(intf, link)
}

func (u *NetNsProbe) getRoutingTable(link netlink.Link, table int) []RoutingTable {
	routeFilter := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Table:     table,
	}
	routeList, err := u.handle.RouteListFiltered(netlink.FAMILY_ALL, routeFilter, netlink.RT_FILTER_OIF|netlink.RT_FILTER_TABLE)
	if err != nil {
		logging.GetLogger().Errorf("Unable to retrieve routing table: %s", err)
		return nil
	}

	if len(routeList) == 0 {
		return nil
	}

	routingTableList := make(map[int]RoutingTable)
	for _, r := range routeList {
		routingTable, ok := routingTableList[r.Table]
		if !ok {
			routingTable = RoutingTable{ID: int64(r.Table), Src: r.Src}
		}

		protocol, prefix := int64(r.Protocol), ""
		if r.Dst != nil {
			prefix = (*r.Dst).String()
		}

		route := routingTable.GetOrCreateRoute(protocol, prefix)
		if len(r.MultiPath) > 0 {
			for _, nh := range r.MultiPath {
				route.GetOrCreateNexthop(nh.Gw, int64(nh.LinkIndex), int64(r.Priority))
			}
		} else {
			route.GetOrCreateNexthop(r.Gw, int64(r.LinkIndex), int64(r.Priority))
		}
		routingTableList[r.Table] = routingTable
	}

	var result []RoutingTable
	for _, r := range routingTableList {
		result = append(result, r)
	}
	return result
}

func (u *NetNsProbe) onLinkAdded(link netlink.Link) {
	if u.isRunning() == true {
		// has been deleted
		index := link.Attrs().Index
		if _, err := u.handle.LinkByIndex(index); err != nil {
			return
		}

		u.Graph.Lock()
		u.addLinkToTopology(link)
		u.Graph.Unlock()
	}
}

func (u *NetNsProbe) onLinkDeleted(link netlink.Link) {
	index := int64(link.Attrs().Index)

	u.Graph.Lock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})

	// case of removing the interface from a bridge
	if intf != nil {
		parents := u.Graph.LookupParents(intf, graph.Metadata{"Type": "bridge"}, nil)
		for _, parent := range parents {
			u.Graph.Unlink(parent, intf)
		}
	}

	// check whether the interface has been deleted or not
	// we get a delete event when an interface is removed from a bridge
	if _, err := u.handle.LinkByIndex(int(index)); err != nil && intf != nil {
		// if openvswitch do not remove let's do the job by ovs piece of code
		driver, _ := intf.GetFieldString("Driver")
		uuid, _ := intf.GetFieldString("UUID")

		if driver == "openvswitch" && uuid != "" {
			u.Graph.Unlink(u.Root, intf)
		} else {
			u.Graph.DelNode(intf)
		}
	}
	u.Graph.Unlock()

	u.Lock()
	delete(u.indexToChildrenQueue, index)
	delete(u.links, link.Attrs().Name)
	u.Unlock()
}

func getFamilyKey(family int) string {
	switch family {
	case netlink.FAMILY_V4:
		return "IPV4"
	case netlink.FAMILY_V6:
		return "IPV6"
	}
	return ""
}

func (u *NetNsProbe) onRouteChanged(index int64, rt []RoutingTable) {
	u.Graph.Lock()
	defer u.Graph.Unlock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if intf == nil {
		logging.GetLogger().Errorf("No interface with index %d to add a new Route", index)
		return
	}
	_, err := intf.GetField("RoutingTable")
	if rt == nil && err == nil {
		u.Graph.DelMetadata(intf, "RoutingTable")
	} else if rt != nil {
		u.Graph.AddMetadata(intf, "RoutingTable", rt)
	}
}

func (u *NetNsProbe) onAddressAdded(addr netlink.Addr, family int, index int64) {
	u.Graph.Lock()
	defer u.Graph.Unlock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if intf == nil {
		logging.GetLogger().Errorf("No interface with index %d for new address %s", index, addr.IPNet.String())
		return
	}

	var ips []string
	key := getFamilyKey(family)
	if v, err := intf.GetField(key); err == nil {
		ips, ok := v.([]string)
		if !ok {
			logging.GetLogger().Errorf("Failed to get IP addresses for node %s", intf.ID)
			return
		}
		for _, ip := range ips {
			if ip == addr.IPNet.String() {
				return
			}
		}
	}

	u.Graph.AddMetadata(intf, key, append(ips, addr.IPNet.String()))
}

func (u *NetNsProbe) onAddressDeleted(addr netlink.Addr, family int, index int64) {
	u.Graph.Lock()
	defer u.Graph.Unlock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if intf == nil {
		logging.GetLogger().Debugf("No interface with index %d for new address %s", index, addr.IPNet.String())
		return
	}

	key := getFamilyKey(family)
	if v, err := intf.GetField(key); err == nil {
		ips, ok := v.([]string)
		if !ok {
			logging.GetLogger().Errorf("Failed to get IP addresses for node %s", intf.ID)
			return
		}
		for i, ip := range ips {
			if ip == addr.IPNet.String() {
				ips = append(ips[:i], ips[i+1:]...)
				break
			}
		}

		if len(ips) == 0 {
			u.Graph.DelMetadata(intf, key)
		} else {
			u.Graph.AddMetadata(intf, key, ips)
		}
	}
}

func (u *NetNsProbe) initialize() {
	logging.GetLogger().Debugf("Initialize Netlink interfaces for %s", u.Root.ID)
	links, err := u.handle.LinkList()
	if err != nil {
		logging.GetLogger().Errorf("Unable to list interfaces: %s", err)
		return
	}

	for _, link := range links {
		logging.GetLogger().Debugf("Initialize ADD %s(%d,%s) within %s", link.Attrs().Name, link.Attrs().Index, link.Type(), u.Root.ID)
		u.Graph.Lock()
		if u.Graph.LookupFirstChild(u.Root, graph.Metadata{"Name": link.Attrs().Name, "IfIndex": int64(link.Attrs().Index)}) == nil {
			u.addLinkToTopology(link)
		}
		u.Graph.Unlock()
	}
}

func (u *NetNsProbe) getRoutingTables(m []byte) ([]RoutingTable, int, error) {
	msg := nl.DeserializeRtMsg(m)
	attrs, err := nl.ParseRouteAttr(m[msg.Len():])
	if err != nil {
		return nil, -1, err
	}
	native := nl.NativeEndian()
	var linkIndex int
	for _, attr := range attrs {
		switch attr.Attr.Type {
		case syscall.RTA_OIF:
			linkIndex = int(native.Uint32(attr.Value[0:4]))
			break
		}
	}

	link, err := u.handle.LinkByIndex(linkIndex)
	if err != nil {
		return nil, linkIndex, err
	}

	return u.getRoutingTable(link, syscall.RTA_UNSPEC), linkIndex, err
}

func parseAddr(m []byte) (addr netlink.Addr, family, index int, err error) {
	msg := nl.DeserializeIfAddrmsg(m)

	family = -1
	index = -1

	attrs, err1 := nl.ParseRouteAttr(m[msg.Len():])
	if err1 != nil {
		err = err1
		return
	}

	family = int(msg.Family)
	index = int(msg.Index)

	var local, dst *net.IPNet
	for _, attr := range attrs {
		switch attr.Attr.Type {
		case syscall.IFA_ADDRESS:
			dst = &net.IPNet{
				IP:   attr.Value,
				Mask: net.CIDRMask(int(msg.Prefixlen), 8*len(attr.Value)),
			}
			addr.Peer = dst
		case syscall.IFA_LOCAL:
			local = &net.IPNet{
				IP:   attr.Value,
				Mask: net.CIDRMask(int(msg.Prefixlen), 8*len(attr.Value)),
			}
			addr.IPNet = local
		}
	}

	// IFA_LOCAL should be there but if not, fall back to IFA_ADDRESS
	if local != nil {
		addr.IPNet = local
	} else {
		addr.IPNet = dst
	}
	addr.Scope = int(msg.Scope)

	return
}

func (u *NetNsProbe) isRunning() bool {
	return atomic.LoadInt64(&u.state) == common.RunningState
}

func (u *NetNsProbe) cloneLinkNodes() map[string]*graph.Node {
	// do a copy of the original in order to avoid inter locks
	// between graph lock and netlink lock while iterating
	u.RLock()
	links := make(map[string]*graph.Node)
	for k, v := range u.links {
		links[k] = v
	}
	u.RUnlock()

	return links
}

func (u *NetNsProbe) updateIntfMetric(now, last time.Time) {
	for name, node := range u.cloneLinkNodes() {
		if link, err := u.handle.LinkByName(name); err == nil {
			currMetric := newInterfaceMetricsFromNetlink(link)
			if currMetric == nil || currMetric.IsZero() {
				continue
			}
			currMetric.Last = int64(common.UnixMillis(now))

			u.Graph.Lock()
			tr := u.Graph.StartMetadataTransaction(node)

			var lastUpdateMetric *topology.InterfaceMetric

			prevMetric, err := node.GetField("Metric")
			if err == nil {
				lastUpdateMetric = currMetric.Sub(prevMetric.(*topology.InterfaceMetric)).(*topology.InterfaceMetric)
			}

			// nothing changed since last update
			if lastUpdateMetric != nil && lastUpdateMetric.IsZero() {
				u.Graph.Unlock()
				continue
			}

			tr.AddMetadata("Metric", currMetric)
			if lastUpdateMetric != nil {
				lastUpdateMetric.Start = int64(common.UnixMillis(last))
				lastUpdateMetric.Last = int64(common.UnixMillis(now))
				tr.AddMetadata("LastUpdateMetric", lastUpdateMetric)
			}

			tr.Commit()
			u.Graph.Unlock()
		}
	}
}

func (u *NetNsProbe) updateIntfFeatures() {
	for name, node := range u.cloneLinkNodes() {
		u.Graph.RLock()
		driver, _ := node.GetFieldString("Driver")
		u.Graph.RUnlock()

		if driver == "" {
			continue
		}

		features, err := u.ethtool.Features(name)
		if err != nil {
			logging.GetLogger().Warningf("Unable to retrieve feature of %s: %s", name, err)
		}

		if len(features) > 0 {
			u.Graph.Lock()
			u.Graph.AddMetadata(node, "Features", features)
			u.Graph.Unlock()
		}
	}
}

func (u *NetNsProbe) start(nlProbe *Probe) {
	u.wg.Add(1)
	defer u.wg.Done()

	// wait for Probe ready
Ready:
	for {
		switch atomic.LoadInt64(&nlProbe.state) {
		case common.StoppingState, common.StoppedState:
			return
		case common.RunningState:
			break Ready
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	atomic.StoreInt64(&u.state, common.RunningState)

	fd := u.socket.GetFd()

	logging.GetLogger().Debugf("Start polling netlink event for %s", u.Root.ID)

	event := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(fd)}
	if err := syscall.EpollCtl(u.epollFd, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
		logging.GetLogger().Errorf("Failed to set the netlink fd as non-blocking: %s", err)
		return
	}
	u.initialize()

	seconds := config.GetInt("agent.topology.netlink.metrics_update")
	metricTicker := time.NewTicker(time.Duration(seconds) * time.Second)
	defer metricTicker.Stop()

	featureTicker := time.NewTicker(5 * time.Second)
	defer featureTicker.Stop()

	last := time.Now().UTC()
	for {
		select {
		case <-featureTicker.C:
			u.updateIntfFeatures()
		case t := <-metricTicker.C:
			now := t.UTC()
			u.updateIntfMetric(now, last)
			last = now
		case <-u.quit:
			return
		}
	}
}

func (u *NetNsProbe) onMessageAvailable() {
	msgs, err := u.socket.Receive()
	if err != nil {
		if errno, ok := err.(syscall.Errno); !ok || !errno.Temporary() {
			logging.GetLogger().Errorf("Failed to receive from netlink messages: %s", err)
		}
		return
	}

	for _, msg := range msgs {
		switch msg.Header.Type {
		case syscall.RTM_NEWLINK:
			link, err := netlink.LinkDeserialize(nil, msg.Data)
			if err != nil {
				logging.GetLogger().Warningf("Failed to deserialize netlink message: %s", err)
				continue
			}
			logging.GetLogger().Debugf("Netlink ADD event for %s(%d,%s) within %s", link.Attrs().Name, link.Attrs().Index, link.Type(), u.Root.ID)
			u.onLinkAdded(link)
		case syscall.RTM_DELLINK:
			link, err := netlink.LinkDeserialize(nil, msg.Data)
			if err != nil {
				logging.GetLogger().Warningf("Failed to deserialize netlink message: %s", err)
				continue
			}
			logging.GetLogger().Debugf("Netlink DEL event for %s(%d) within %s", link.Attrs().Name, link.Attrs().Index, u.Root.ID)
			u.onLinkDeleted(link)
		case syscall.RTM_NEWADDR:
			addr, family, ifindex, err := parseAddr(msg.Data)
			if err != nil {
				logging.GetLogger().Warningf("Failed to parse newlink message: %s", err)
				continue
			}
			u.onAddressAdded(addr, family, int64(ifindex))
		case syscall.RTM_DELADDR:
			addr, family, ifindex, err := parseAddr(msg.Data)
			if err != nil {
				logging.GetLogger().Warningf("Failed to parse newlink message: %s", err)
				continue
			}
			u.onAddressDeleted(addr, family, int64(ifindex))
		case syscall.RTM_NEWROUTE, syscall.RTM_DELROUTE:
			rt, index, err := u.getRoutingTables(msg.Data)
			if err != nil {
				logging.GetLogger().Warningf("Failed to get Routes: %s", err)
				continue
			}
			u.onRouteChanged(int64(index), rt)
		}
	}
}

func (u *NetNsProbe) closeFds() {
	if u.handle != nil {
		u.handle.Delete()
	}
	if u.socket != nil {
		u.socket.Close()
	}
	if u.ethtool != nil {
		u.ethtool.Close()
	}
	if u.epollFd != 0 {
		syscall.Close(u.epollFd)
	}
}

func (u *NetNsProbe) stop() {
	if atomic.CompareAndSwapInt64(&u.state, common.RunningState, common.StoppingState) {
		u.quit <- true
		u.wg.Wait()
	}
	u.closeFds()
}

func newNetNsProbe(g *graph.Graph, root *graph.Node, nsPath string) (*NetNsProbe, error) {
	probe := &NetNsProbe{
		Graph:                g,
		Root:                 root,
		NsPath:               nsPath,
		indexToChildrenQueue: make(map[int64][]pendingLink),
		links:                make(map[string]*graph.Node),
		quit:                 make(chan bool),
	}

	var context *common.NetNSContext
	var err error

	errFnc := func(err error) (*NetNsProbe, error) {
		probe.closeFds()
		context.Close()

		return nil, err
	}

	// Enter the network namespace if necessary
	if nsPath != "" {
		context, err = common.NewNetNsContext(nsPath)
		if err != nil {
			return errFnc(fmt.Errorf("Failed to switch namespace: %s", err))
		}
	}

	// Both NewHandle and Subscribe need to done in the network namespace.
	if probe.handle, err = netlink.NewHandle(syscall.NETLINK_ROUTE); err != nil {
		return errFnc(fmt.Errorf("Failed to create netlink handle: %s", err))
	}

	if probe.socket, err = nl.Subscribe(syscall.NETLINK_ROUTE, syscall.RTNLGRP_LINK, syscall.RTNLGRP_IPV4_IFADDR, syscall.RTNLGRP_IPV6_IFADDR, syscall.RTNLGRP_IPV4_MROUTE, syscall.RTNLGRP_IPV4_ROUTE, syscall.RTNLGRP_IPV6_MROUTE, syscall.RTNLGRP_IPV6_ROUTE); err != nil {
		return errFnc(fmt.Errorf("Failed to subscribe to netlink messages: %s", err))
	}

	if probe.ethtool, err = ethtool.NewEthtool(); err != nil {
		return errFnc(fmt.Errorf("Failed to create ethtool object: %s", err))
	}

	if probe.epollFd, err = syscall.EpollCreate1(0); err != nil {
		return errFnc(fmt.Errorf("Failed to create epoll: %s", err))
	}

	// Leave the network namespace
	context.Close()

	return probe, nil
}

// Register a new network netlink/namespace probe in the graph
func (u *Probe) Register(nsPath string, root *graph.Node) (*NetNsProbe, error) {
	probe, err := newNetNsProbe(u.Graph, root, nsPath)
	if err != nil {
		return nil, err
	}

	event := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(probe.epollFd)}
	if err := syscall.EpollCtl(u.epollFd, syscall.EPOLL_CTL_ADD, probe.epollFd, &event); err != nil {
		probe.closeFds()
		return nil, fmt.Errorf("Failed to add fd to epoll events set for %s: %s", root.String(), err.Error())
	}

	u.Lock()
	u.probes[int32(probe.epollFd)] = probe
	u.Unlock()

	go probe.start(u)

	return probe, nil
}

// Unregister a probe from a network namespace
func (u *Probe) Unregister(nsPath string) error {
	u.Lock()
	defer u.Unlock()

	for fd, probe := range u.probes {
		if probe.NsPath == nsPath {
			if err := syscall.EpollCtl(u.epollFd, syscall.EPOLL_CTL_DEL, int(fd), nil); err != nil {
				return fmt.Errorf("Failed to del fd from epoll events set for %s: %s", probe.Root.ID, err.Error())
			}
			delete(u.probes, fd)

			probe.stop()

			return nil
		}
	}
	return fmt.Errorf("failed to unregister, probe not found for %s", nsPath)
}

func (u *Probe) start() {
	u.wg.Add(1)
	defer u.wg.Done()

	events := make([]syscall.EpollEvent, maxEpollEvents)

	atomic.StoreInt64(&u.state, common.RunningState)
	for atomic.LoadInt64(&u.state) == common.RunningState {
		nevents, err := syscall.EpollWait(u.epollFd, events[:], 200)
		if err != nil {
			if errno, ok := err.(syscall.Errno); ok && errno != syscall.EINTR {
				logging.GetLogger().Errorf("Failed to receive from events from netlink: %s", err)
			}
			continue
		}

		if nevents == 0 {
			continue
		}

		u.RLock()
		for ev := 0; ev < nevents; ev++ {
			if probe, ok := u.probes[events[ev].Fd]; ok {
				probe.onMessageAvailable()
			}
		}
		u.RUnlock()
	}
}

// Start the probe
func (u *Probe) Start() {
	go u.start()
}

// Stop the probe
func (u *Probe) Stop() {
	if atomic.CompareAndSwapInt64(&u.state, common.RunningState, common.StoppingState) {
		u.wg.Wait()

		u.RLock()
		defer u.RUnlock()

		for _, probe := range u.probes {
			go probe.stop()
		}

		for _, probe := range u.probes {
			probe.wg.Wait()
		}
	}
}

// NewProbe creates a new netlink probe
func NewProbe(g *graph.Graph, n *graph.Node) (*Probe, error) {
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, fmt.Errorf("Failed to create epoll: %s", err)
	}

	nlProbe := &Probe{
		Graph:   g,
		epollFd: epfd,
		probes:  make(map[int32]*NetNsProbe),
	}

	nlProbe.Register("", n)

	return nlProbe, nil
}
