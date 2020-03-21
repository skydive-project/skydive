// +build linux

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

package netlink

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/avast/retry-go"
	"github.com/safchain/ethtool"
	"github.com/safchain/insanelock"
	"github.com/spf13/cast"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/netns"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	tp "github.com/skydive-project/skydive/topology/probes"
)

const (
	maxEpollEvents = 32

	// RtmNewNeigh Neighbor netlink message
	RtmNewNeigh = 28
	// RtmDelNeigh Neighbor netlink message
	RtmDelNeigh = 29
)

var flagNames = []string{
	"UP",
	"BROADCAST",
	"LOOPBACK",
	"POINTTOPOINT",
	"MULTICAST",
}

type pendingLink struct {
	id           graph.Identifier
	relationType string
	metadata     graph.Metadata
}

// Probe describes a topology probe based on netlink in a network namespace
type Probe struct {
	insanelock.RWMutex
	Ctx                  tp.Context
	NsPath               string
	epollFd              int
	ethtool              *ethtool.Ethtool
	handle               *netlink.Handle
	socket               *nl.NetlinkSocket
	indexToChildrenQueue map[int64][]pendingLink
	links                map[int]*graph.Node
	state                service.State
	wg                   sync.WaitGroup
	quit                 chan bool
	netNsNameTry         map[graph.Identifier]int
	sriovProcessor       *graph.Processor
}

// ProbeHandler describes a list NetLink NameSpace probe to enhance the graph
type ProbeHandler struct {
	insanelock.RWMutex
	Ctx            tp.Context
	epollFd        int
	probes         map[int32]*Probe
	state          service.State
	wg             sync.WaitGroup
	sriovProcessor *graph.Processor
}

func (u *Probe) linkPendingChildren(intf *graph.Node, index int64) {
	// ignore ovs-system interface as it doesn't make any sense according to
	// the following thread:
	// http://openvswitch.org/pipermail/discuss/2013-October/011657.html
	if name, _ := intf.GetFieldString("Name"); name == "ovs-system" {
		return
	}

	// add children of this interface that was previously added
	if children, ok := u.indexToChildrenQueue[index]; ok {
		for _, link := range children {
			child := u.Ctx.Graph.GetNode(link.id)
			if child != nil {
				topology.AddLink(u.Ctx.Graph, intf, child, link.relationType, link.metadata)
			}
		}
		delete(u.indexToChildrenQueue, index)
	}
}

func (u *Probe) linkIntfToIndex(intf *graph.Node, index int64, relationType string, m graph.Metadata) {
	// assuming we have only one master with this index
	parent := u.Ctx.Graph.LookupFirstChild(u.Ctx.RootNode, graph.Metadata{"IfIndex": index})
	if parent != nil {
		// ignore ovs-system interface as it doesn't make any sense according to
		// the following thread:
		// http://openvswitch.org/pipermail/discuss/2013-October/011657.html
		if name, _ := parent.GetFieldString("Name"); name == "ovs-system" {
			return
		}

		if !topology.HaveLink(u.Ctx.Graph, parent, intf, relationType) {
			topology.AddLink(u.Ctx.Graph, parent, intf, relationType, m)
		}
	} else {
		// not yet the bridge so, enqueue for a later add
		link := pendingLink{id: intf.ID, relationType: relationType, metadata: m}
		u.indexToChildrenQueue[index] = append(u.indexToChildrenQueue[index], link)
	}
}

func (u *Probe) handleIntfIsChild(intf *graph.Node, link netlink.Link) {
	// handle pending relationship
	u.linkPendingChildren(intf, int64(link.Attrs().Index))

	// interface being a part of a bridge
	if link.Attrs().MasterIndex != 0 {
		u.linkIntfToIndex(intf, int64(link.Attrs().MasterIndex), topology.Layer2Link, nil)
	}

	if link.Attrs().ParentIndex != 0 {
		if _, err := intf.GetFieldInt64("Vlan"); err == nil {
			u.linkIntfToIndex(intf, int64(int64(link.Attrs().ParentIndex)), topology.Layer2Link, graph.Metadata{"Type": "vlan"})
		}
	}
}

func (u *Probe) handleIntfIsVeth(intf *graph.Node, link netlink.Link) {
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
			u.Ctx.Graph.Lock()
			defer u.Ctx.Graph.Unlock()

			// re get the interface from the graph since the interface could have been deleted
			if u.Ctx.Graph.GetNode(intf.ID) == nil {
				return errors.New("Node not found")
			}

			var peer *graph.Node
			if root == nil {
				peer = u.Ctx.Graph.LookupFirstNode(graph.Metadata{"IfIndex": peerIndex, "Type": "veth"})
			} else {
				peer = u.Ctx.Graph.LookupFirstChild(root, graph.Metadata{"IfIndex": peerIndex, "Type": "veth"})
			}
			if peer == nil {
				return errors.New("Peer not found")
			}
			if !topology.HaveLayer2Link(u.Ctx.Graph, peer, intf) {
				topology.AddLayer2Link(u.Ctx.Graph, peer, intf, linkMetadata)
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
					return peerResolver(u.Ctx.RootNode)
				}
				if err := retry.Do(localFnc, retry.Delay(10*time.Millisecond)); err != nil {
					peerResolver(nil)
				}
			}()
		}
	}
}

func (u *Probe) addGenericLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	index := int64(link.Attrs().Index)

	var intf *graph.Node
	intf = u.Ctx.Graph.LookupFirstChild(u.Ctx.RootNode, graph.Metadata{
		"IfIndex": index,
	})

	if intf == nil {
		var err error
		intf, err = u.Ctx.Graph.NewNode(graph.GenID(), m)
		if err != nil {
			u.Ctx.Logger.Error(err)
			return nil
		}
	}

	if !topology.HaveOwnershipLink(u.Ctx.Graph, u.Ctx.RootNode, intf) {
		topology.AddOwnershipLink(u.Ctx.Graph, u.Ctx.RootNode, intf, nil)
	}

	return intf
}

func (u *Probe) addBridgeLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	index := int64(link.Attrs().Index)
	intf := u.addGenericLinkToTopology(link, m)

	u.linkPendingChildren(intf, index)

	return intf
}

func (u *Probe) addOvsLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	attrs := link.Attrs()
	name := attrs.Name

	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Name", name),
		filters.NewOrFilter(
			filters.NewTermStringFilter("MAC", attrs.HardwareAddr.String()),
			filters.NewTermStringFilter("ExtID.attached-mac", attrs.HardwareAddr.String()),
		),
		filters.NewNotNullFilter("UUID"),
	)

	intf := u.Ctx.Graph.LookupFirstNode(graph.NewElementFilter(filter))
	if intf != nil {
		if !topology.HaveOwnershipLink(u.Ctx.Graph, u.Ctx.RootNode, intf) {
			topology.AddOwnershipLink(u.Ctx.Graph, u.Ctx.RootNode, intf, nil)
		}
	}

	return intf
}

func (u *Probe) getLinkIPs(link netlink.Link, family int) (ips []string) {
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

func (u *Probe) getFamilyNeighbors(index, family int) *topology.Neighbors {
	var neighbors topology.Neighbors

	neighList, err := u.handle.NeighList(index, family)
	if err == nil && len(neighList) > 0 {
		for i, neigh := range neighList {
			neighbors = append(neighbors, &topology.Neighbor{
				Flags:   getFlagsString(neighFlags, neigh.Flags),
				MAC:     neigh.HardwareAddr.String(),
				State:   getFlagsString(neighStates, neigh.State),
				IfIndex: int64(neigh.LinkIndex),
				Vlan:    int64(neigh.Vlan),
				VNI:     int64(neigh.VNI),
			})
			if neigh.IP != nil {
				neighbors[i].IP = neigh.IP
			}
		}
	}
	return &neighbors
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

func (u *Probe) updateLinkNetNsName(intf *graph.Node, link netlink.Link, metadata graph.Metadata) bool {
	var context *netns.Context

	lnsid := link.Attrs().NetNsID

	nodes := u.Ctx.Graph.GetNodes(graph.Metadata{"Type": "netns"})
	for _, n := range nodes {
		if path, err := n.GetFieldString("Path"); err == nil {
			if u.NsPath != "" {
				context, err = netns.NewContext(u.NsPath)
				if err != nil {
					continue
				}
			}

			fd, err := syscall.Open(path, syscall.O_RDONLY, 0)

			if context != nil {
				context.Close()
			}

			if err != nil {
				continue
			}

			nsid, err := u.handle.GetNetNsIdByFd(fd)
			syscall.Close(fd)

			if err != nil {
				continue
			}

			if lnsid == nsid {
				if name, err := n.GetFieldString("Name"); err == nil {
					metadata["LinkNetNsName"] = name
				}

				return true
			}
		}
	}

	return false
}

func (u *Probe) updateLinkNetNs(intf *graph.Node, link netlink.Link, metadata graph.Metadata) {
	nnsid := int64(link.Attrs().NetNsID)
	if nnsid == -1 {
		return
	}

	onsid, err := intf.GetFieldInt64("LinkNetNsID")
	if err != nil {
		onsid = int64(-1)
	}

	onsname, _ := intf.GetFieldString("LinkNetNsName")

	if nnsid != onsid {
		metadata["LinkNetNsID"] = int64(nnsid)
		metadata["LinkNetNsName"] = ""
	} else if onsname != "" {
		return
	}

	const try = 3

	i := u.netNsNameTry[intf.ID]
	if i > try*3 {
		// try again after a while
		i = 0
	}

	if i < try {
		if u.updateLinkNetNsName(intf, link, metadata) {
			// avoid resolution if success
			i = try - 1
		}
	}

	u.netNsNameTry[intf.ID] = i + 1
}

func (u *Probe) addLinkToTopology(link netlink.Link) {
	attrs := link.Attrs()

	driver, _ := u.ethtool.DriverName(attrs.Name)
	if driver == "" && link.Type() == "bridge" {
		driver = "bridge"
	}

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

	if bondSlave := attrs.BondSlave; bondSlave != nil {
		slaveMetadata := map[string]interface{}{
			"Type":                   bondSlave.Type,
			"State":                  bondSlave.State.String(),
			"MiiStatus":              bondSlave.MiiStatus.String(),
			"LinkFailureCount":       int64(bondSlave.LinkFailureCount),
			"QueueId":                int64(bondSlave.QueueId),
			"AggregatorId":           int64(bondSlave.AggregatorId),
			"AdActorOperPortState":   int64(bondSlave.AdActorOperPortState),
			"AdPartnerOperPortState": int64(bondSlave.AdPartnerOperPortState),
		}

		if permMAC := bondSlave.PermHardwareAddr.String(); permMAC != "" {
			slaveMetadata["PermMAC"] = permMAC
		}

		metadata["BondSlave"] = slaveMetadata
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

	fdb, neighbors := u.getNeighbors(attrs.Index)
	if len(*fdb) > 0 {
		metadata["FDB"] = fdb
	}
	if len(*neighbors) > 0 {
		metadata["Neighbors"] = neighbors
	}

	if rts := u.getRoutingTables(link, syscall.RTA_UNSPEC); rts != nil {
		metadata["RoutingTables"] = &rts
	}

	if metric := newInterfaceMetricsFromNetlink(link); metric != nil {
		metadata["Metric"] = metric
	}

	if linkType == "veth" {
		stats, err := u.ethtool.Stats(attrs.Name)
		if err != nil && err != syscall.ENODEV {
			u.Ctx.Logger.Errorf("Unable get stats from ethtool (%s): %s", attrs.Name, err)
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
		if vlan, err := cast.ToInt64E(vlan.VlanId); err == nil {
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

	businfo, err := u.ethtool.BusInfo(attrs.Name)
	if err != nil && err != syscall.ENODEV {
		u.Ctx.Logger.Debugf(
			"Unable get Bus Info from ethtool (%s): %s", attrs.Name, err)
	} else {
		metadata["BusInfo"] = businfo
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
	u.links[attrs.Index] = intf
	u.Unlock()

	go u.handleSriov(u.Ctx.Graph, intf, attrs.Index, businfo, attrs.Vfs, attrs.Name)

	u.updateLinkNetNs(intf, link, metadata)

	// merge metadata
	tr := u.Ctx.Graph.StartMetadataTransaction(intf)
	for k, v := range metadata {
		tr.AddMetadata(k, v)
	}
	tr.Commit()

	u.handleIntfIsChild(intf, link)
	u.handleIntfIsVeth(intf, link)
}

func (u *Probe) getRoutingTables(link netlink.Link, table int) *topology.RoutingTables {
	routeFilter := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Table:     table,
	}
	routeList, err := u.handle.RouteListFiltered(netlink.FAMILY_ALL, routeFilter, netlink.RT_FILTER_OIF|netlink.RT_FILTER_TABLE)
	if err != nil {
		u.Ctx.Logger.Errorf("Unable to retrieve routing table: %s", err)
		return nil
	}

	if len(routeList) == 0 {
		return nil
	}

	routingTableMap := make(map[int]*topology.RoutingTable)
	for _, r := range routeList {
		routingTable, ok := routingTableMap[r.Table]
		if !ok {
			routingTable = &topology.RoutingTable{ID: int64(r.Table), Src: r.Src}
		}

		var prefix net.IPNet
		if r.Dst != nil {
			prefix = *r.Dst
		} else {
			if len(r.Gw) == net.IPv4len {
				prefix = topology.IPv4DefaultRoute
			} else {
				prefix = topology.IPv6DefaultRoute
			}
		}

		protocol := int64(r.Protocol)
		route := routingTable.GetOrCreateRoute(protocol, prefix)
		if len(r.MultiPath) > 0 {
			for _, nh := range r.MultiPath {
				route.GetOrCreateNextHop(nh.Gw, int64(nh.LinkIndex), int64(r.Priority))
			}
		} else {
			route.GetOrCreateNextHop(r.Gw, int64(r.LinkIndex), int64(r.Priority))
		}
		routingTableMap[r.Table] = routingTable
	}

	var result topology.RoutingTables
	for _, r := range routingTableMap {
		result = append(result, r)
	}
	return &result
}

func (u *Probe) onLinkAdded(link netlink.Link) {
	if u.isRunning() == true {
		// has been deleted
		index := link.Attrs().Index
		if _, err := u.handle.LinkByIndex(index); err != nil {
			return
		}

		u.Ctx.Graph.Lock()
		u.addLinkToTopology(link)
		u.Ctx.Graph.Unlock()
	}
}

func (u *Probe) onLinkDeleted(link netlink.Link) {
	index := int64(link.Attrs().Index)

	u.Ctx.Graph.Lock()

	intf := u.Ctx.Graph.LookupFirstChild(u.Ctx.RootNode, graph.Metadata{"IfIndex": index})

	// case of removing the interface from a bridge
	if intf != nil {
		parents := u.Ctx.Graph.LookupParents(intf, graph.Metadata{"Type": "bridge"}, nil)
		for _, parent := range parents {
			if err := u.Ctx.Graph.Unlink(parent, intf); err != nil {
				u.Ctx.Logger.Error(err)
			}
		}
	}

	// check whether the interface has been deleted or not
	// we get a delete event when an interface is removed from a bridge
	if _, err := u.handle.LinkByIndex(int(index)); err != nil && intf != nil {
		// if openvswitch do not remove let's do the job by ovs piece of code
		driver, _ := intf.GetFieldString("Driver")
		uuid, _ := intf.GetFieldString("UUID")

		if driver == "openvswitch" && uuid != "" {
			err = u.Ctx.Graph.Unlink(u.Ctx.RootNode, intf)
		} else {
			delete(u.netNsNameTry, intf.ID)

			err = u.Ctx.Graph.DelNode(intf)
		}

		if err != nil {
			u.Ctx.Logger.Error(err)
		}
	}
	u.Ctx.Graph.Unlock()

	delete(u.indexToChildrenQueue, index)

	u.Lock()
	delete(u.links, link.Attrs().Index)
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

func (u *Probe) onRoutingTablesChanged(index int64, rts *topology.RoutingTables) {
	u.Ctx.Graph.Lock()
	defer u.Ctx.Graph.Unlock()

	intf := u.Ctx.Graph.LookupFirstChild(u.Ctx.RootNode, graph.Metadata{"IfIndex": index})
	if intf == nil {
		if _, err := u.handle.LinkByIndex(int(index)); err == nil {
			u.Ctx.Logger.Errorf("Unable to find interface with index %d to add a new Route", index)
		}
		return
	}
	_, err := intf.GetField("RoutingTables")
	if rts == nil && err == nil {
		err = u.Ctx.Graph.DelMetadata(intf, "RoutingTables")
	} else if rts != nil {
		err = u.Ctx.Graph.AddMetadata(intf, "RoutingTables", rts)
	} else {
		err = nil
	}

	if err != nil {
		u.Ctx.Logger.Error(err)
	}
}

func (u *Probe) onAddressAdded(addr netlink.Addr, family int, index int64) {
	u.Ctx.Graph.Lock()
	defer u.Ctx.Graph.Unlock()

	intf := u.Ctx.Graph.LookupFirstChild(u.Ctx.RootNode, graph.Metadata{"IfIndex": index})
	if intf == nil {
		if _, err := u.handle.LinkByIndex(int(index)); err == nil {
			u.Ctx.Logger.Errorf("Unable to find interface with index %d to add address %s", index, addr.IPNet)
		}
		return
	}

	var ips []string
	key := getFamilyKey(family)
	if v, err := intf.GetField(key); err == nil {
		ips, ok := v.([]string)
		if !ok {
			u.Ctx.Logger.Errorf("Failed to get IP addresses for node %s", intf.ID)
			return
		}
		for _, ip := range ips {
			if ip == addr.IPNet.String() {
				return
			}
		}
	}

	if err := u.Ctx.Graph.AddMetadata(intf, key, append(ips, addr.IPNet.String())); err != nil {
		u.Ctx.Logger.Error(err)
	}
}

func (u *Probe) onAddressDeleted(addr netlink.Addr, family int, index int64) {
	u.Ctx.Graph.Lock()
	defer u.Ctx.Graph.Unlock()

	intf := u.Ctx.Graph.LookupFirstChild(u.Ctx.RootNode, graph.Metadata{"IfIndex": index})
	if intf == nil {
		if _, err := u.handle.LinkByIndex(int(index)); err == nil {
			u.Ctx.Logger.Errorf("Unable to find interface with index %d to del address %s", index, addr.IPNet)
		}
		return
	}

	key := getFamilyKey(family)
	if v, err := intf.GetField(key); err == nil {
		ips, ok := v.([]string)
		if !ok {
			u.Ctx.Logger.Errorf("Failed to get IP addresses for node %s", intf.ID)
			return
		}
		for i, ip := range ips {
			if ip == addr.IPNet.String() {
				ips = append(ips[:i], ips[i+1:]...)
				break
			}
		}

		if len(ips) == 0 {
			err = u.Ctx.Graph.DelMetadata(intf, key)
		} else {
			err = u.Ctx.Graph.AddMetadata(intf, key, ips)
		}

		if err != nil {
			u.Ctx.Logger.Error(err)
		}
	}
}

func (u *Probe) initialize() {
	u.Ctx.Logger.Debugf("Initialize Netlink interfaces for %s", u.Ctx.RootNode.ID)
	links, err := u.handle.LinkList()
	if err != nil {
		u.Ctx.Logger.Errorf("Unable to list interfaces: %s", err)
		return
	}

	for _, link := range links {
		attrs := link.Attrs()

		u.Ctx.Logger.Debugf("Initialize ADD %s(%d,%s) within %s", attrs.Name, attrs.Index, link.Type(), u.Ctx.RootNode.ID)
		u.Ctx.Graph.Lock()
		if u.Ctx.Graph.LookupFirstChild(u.Ctx.RootNode, graph.Metadata{"Name": attrs.Name, "IfIndex": int64(attrs.Index)}) == nil {
			u.addLinkToTopology(link)
		}
		u.Ctx.Graph.Unlock()
	}
}

func (u *Probe) onNeighborsChanged(index int64, fdb *topology.Neighbors, neighbors *topology.Neighbors) {
	u.Ctx.Graph.Lock()
	defer u.Ctx.Graph.Unlock()

	intf := u.Ctx.Graph.LookupFirstChild(u.Ctx.RootNode, graph.Metadata{"IfIndex": index})
	if intf == nil {
		if _, err := u.handle.LinkByIndex(int(index)); err == nil {
			u.Ctx.Logger.Errorf("Unable to find interface with index %d to add a new neighbors", index)
		}
		return
	}

	update := func(key string, nb *topology.Neighbors) {
		_, err := intf.GetField(key)
		if nb == nil && err == nil {
			err = u.Ctx.Graph.DelMetadata(intf, key)
		} else if nb != nil {
			err = u.Ctx.Graph.AddMetadata(intf, key, nb)
		} else {
			err = nil
		}

		if err != nil {
			u.Ctx.Logger.Error(err)
		}
	}

	update("FDB", fdb)
	update("Neighbors", neighbors)
}

func (u *Probe) getNeighbors(index int) (*topology.Neighbors, *topology.Neighbors) {
	fdb := u.getFamilyNeighbors(index, syscall.AF_BRIDGE)
	neighbors := u.getFamilyNeighbors(index, syscall.AF_INET)
	*neighbors = append(*neighbors, *u.getFamilyNeighbors(index, syscall.AF_INET6)...)

	return fdb, neighbors
}

func (u *Probe) parseNeighborMsg(m []byte) (*topology.Neighbors, *topology.Neighbors, int, error) {
	neigh, err := netlink.NeighDeserialize(m)
	if err != nil {
		return nil, nil, 0, err
	}

	fdb, neighbors := u.getNeighbors(neigh.LinkIndex)

	return fdb, neighbors, neigh.LinkIndex, nil
}

func (u *Probe) parseRtMsg(m []byte) (*topology.RoutingTables, int, error) {
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

	return u.getRoutingTables(link, syscall.RTA_UNSPEC), linkIndex, err
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

func (u *Probe) isRunning() bool {
	return u.state.Load() == service.RunningState
}

func (u *Probe) cloneLinkNodes() map[int]*graph.Node {
	// do a copy of the original in order to avoid inter locks
	// between graph lock and netlink lock while iterating
	u.RLock()
	links := make(map[int]*graph.Node)
	for k, v := range u.links {
		links[k] = v
	}
	u.RUnlock()

	return links
}

func (u *Probe) updateIntfMetric(now, last time.Time) {
	for index, node := range u.cloneLinkNodes() {
		if link, err := u.handle.LinkByIndex(index); err == nil {
			currMetric := newInterfaceMetricsFromNetlink(link)
			if currMetric == nil || currMetric.IsZero() {
				continue
			}
			currMetric.Last = int64(common.UnixMillis(now))

			u.Ctx.Graph.Lock()
			tr := u.Ctx.Graph.StartMetadataTransaction(node)

			var lastUpdateMetric *topology.InterfaceMetric

			prevMetric, err := node.GetField("Metric")
			if err == nil {
				lastUpdateMetric = currMetric.Sub(prevMetric.(*topology.InterfaceMetric)).(*topology.InterfaceMetric)
			}

			// nothing changed since last update
			if lastUpdateMetric != nil && lastUpdateMetric.IsZero() {
				u.Ctx.Graph.Unlock()
				continue
			}

			tr.AddMetadata("Metric", currMetric)
			if lastUpdateMetric != nil {
				lastUpdateMetric.Start = int64(common.UnixMillis(last))
				lastUpdateMetric.Last = int64(common.UnixMillis(now))
				tr.AddMetadata("LastUpdateMetric", lastUpdateMetric)
			}

			tr.Commit()
			u.Ctx.Graph.Unlock()
		}
	}
}

func (u *Probe) updateIntfFeatures(name string, metadata graph.Metadata) {
	features, err := u.ethtool.Features(name)
	if err != nil {
		u.Ctx.Logger.Warningf("Unable to retrieve feature of %s: %s", name, err)
	}

	if len(features) > 0 {
		metadata["Features"] = features
	}
}

func (u *Probe) updateIntfs() {
	for _, node := range u.cloneLinkNodes() {
		u.Ctx.Graph.RLock()
		driver, _ := node.GetFieldString("Driver")
		name, _ := node.GetFieldString("Name")
		u.Ctx.Graph.RUnlock()

		if driver == "" {
			continue
		}

		metadata := make(graph.Metadata)
		u.updateIntfFeatures(name, metadata)

		if link, err := u.handle.LinkByName(name); err == nil {
			u.Ctx.Graph.RLock()
			u.updateLinkNetNs(node, link, metadata)
			u.Ctx.Graph.RUnlock()
		}

		u.Ctx.Graph.Lock()
		tr := u.Ctx.Graph.StartMetadataTransaction(node)
		for k, v := range metadata {
			tr.AddMetadata(k, v)
		}
		tr.Commit()
		u.Ctx.Graph.Unlock()
	}
}

func (u *Probe) start(handler *ProbeHandler) {
	u.wg.Add(1)
	defer u.wg.Done()

	// wait for Probe ready
Ready:
	for {
		switch handler.state.Load() {
		case service.StoppingState, service.StoppedState:
			return
		case service.RunningState:
			break Ready
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	u.state.Store(service.RunningState)

	fd := u.socket.GetFd()

	u.Ctx.Logger.Debugf("Start polling netlink event for %s", u.Ctx.RootNode.ID)

	event := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(fd)}
	if err := syscall.EpollCtl(u.epollFd, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
		u.Ctx.Logger.Errorf("Failed to set the netlink fd as non-blocking: %s", err)
		return
	}
	u.initialize()

	seconds := u.Ctx.Config.GetInt("agent.topology.netlink.metrics_update")
	metricTicker := time.NewTicker(time.Duration(seconds) * time.Second)
	defer metricTicker.Stop()

	updateIntfsTicker := time.NewTicker(5 * time.Second)
	defer updateIntfsTicker.Stop()

	last := time.Now().UTC()
	for {
		select {
		case <-updateIntfsTicker.C:
			u.updateIntfs()
		case t := <-metricTicker.C:
			now := t.UTC()
			u.updateIntfMetric(now, last)
			last = now
		case <-u.quit:
			return
		}
	}
}

func (u *Probe) onMessageAvailable() {
	msgs, err := u.socket.Receive()
	if err != nil {
		if errno, ok := err.(syscall.Errno); !ok || !errno.Temporary() {
			u.Ctx.Logger.Errorf("Failed to receive from netlink messages: %s", err)
		}
		return
	}

	for _, msg := range msgs {
		switch msg.Header.Type {
		case syscall.RTM_NEWLINK:
			link, err := netlink.LinkDeserialize(nil, msg.Data)
			if err != nil {
				u.Ctx.Logger.Warningf("Failed to deserialize netlink message: %s", err)
				continue
			}
			u.Ctx.Logger.Debugf("Netlink ADD event for %s(%d,%s) within %s", link.Attrs().Name, link.Attrs().Index, link.Type(), u.Ctx.RootNode.ID)
			u.onLinkAdded(link)
		case syscall.RTM_DELLINK:
			link, err := netlink.LinkDeserialize(nil, msg.Data)
			if err != nil {
				u.Ctx.Logger.Warningf("Failed to deserialize netlink message: %s", err)
				continue
			}
			u.Ctx.Logger.Debugf("Netlink DEL event for %s(%d) within %s", link.Attrs().Name, link.Attrs().Index, u.Ctx.RootNode.ID)
			u.onLinkDeleted(link)
		case syscall.RTM_NEWADDR:
			addr, family, ifindex, err := parseAddr(msg.Data)
			if err != nil {
				u.Ctx.Logger.Warningf("Failed to parse newlink message: %s", err)
				continue
			}
			u.onAddressAdded(addr, family, int64(ifindex))
		case syscall.RTM_DELADDR:
			addr, family, ifindex, err := parseAddr(msg.Data)
			if err != nil {
				u.Ctx.Logger.Warningf("Failed to parse newlink message: %s", err)
				continue
			}
			u.onAddressDeleted(addr, family, int64(ifindex))
		case syscall.RTM_NEWROUTE, syscall.RTM_DELROUTE:
			rts, index, err := u.parseRtMsg(msg.Data)
			if err != nil {
				u.Ctx.Logger.Warningf("Failed to get Routes: %s", err)
				continue
			}
			u.onRoutingTablesChanged(int64(index), rts)

		case RtmNewNeigh, RtmDelNeigh:
			fdb, neighbors, index, err := u.parseNeighborMsg(msg.Data)
			if err != nil {
				u.Ctx.Logger.Warningf("Failed to get Neighbors: %s", err)
				continue
			}
			u.onNeighborsChanged(int64(index), fdb, neighbors)
		}
	}
}

func (u *Probe) closeFds() {
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

func (u *Probe) stop() {
	if u.state.CompareAndSwap(service.RunningState, service.StoppingState) {
		u.quit <- true
		u.wg.Wait()
	}
	u.closeFds()
}

func newProbe(ctx tp.Context, nsPath string, sriovProcessor *graph.Processor) (*Probe, error) {
	probe := &Probe{
		Ctx:                  ctx,
		NsPath:               nsPath,
		indexToChildrenQueue: make(map[int64][]pendingLink),
		links:                make(map[int]*graph.Node),
		quit:                 make(chan bool),
		netNsNameTry:         make(map[graph.Identifier]int),
		sriovProcessor:       sriovProcessor,
	}
	var context *netns.Context
	var err error

	errFnc := func(err error) (*Probe, error) {
		probe.closeFds()
		context.Close()

		return nil, err
	}

	// Enter the network namespace if necessary
	if nsPath != "" {
		context, err = netns.NewContext(nsPath)
		if err != nil {
			return errFnc(fmt.Errorf("Failed to switch namespace: %s", err))
		}
	}

	// Both NewHandle and Subscribe need to done in the network namespace.
	if probe.handle, err = netlink.NewHandle(syscall.NETLINK_ROUTE); err != nil {
		return errFnc(fmt.Errorf("Failed to create netlink handle: %s", err))
	}

	groups := []uint{
		syscall.RTNLGRP_LINK,
		syscall.RTNLGRP_IPV4_IFADDR,
		syscall.RTNLGRP_IPV6_IFADDR,
		syscall.RTNLGRP_IPV4_MROUTE,
		syscall.RTNLGRP_IPV4_ROUTE,
		syscall.RTNLGRP_IPV6_MROUTE,
		syscall.RTNLGRP_IPV6_ROUTE,
		syscall.RTNLGRP_NEIGH,
	}

	if probe.socket, err = nl.Subscribe(syscall.NETLINK_ROUTE, groups...); err != nil {
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
func (u *ProbeHandler) Register(nsPath string, ctx tp.Context) (*Probe, error) {
	probe, err := newProbe(ctx, nsPath, u.sriovProcessor)
	if err != nil {
		return nil, err
	}

	event := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(probe.epollFd)}
	if err := syscall.EpollCtl(u.epollFd, syscall.EPOLL_CTL_ADD, probe.epollFd, &event); err != nil {
		probe.closeFds()
		return nil, fmt.Errorf("Failed to add fd to epoll events set for %s: %s", ctx.RootNode.String(), err)
	}

	u.Lock()
	u.probes[int32(probe.epollFd)] = probe
	u.Unlock()

	go probe.start(u)

	return probe, nil
}

// Unregister a probe from a network namespace
func (u *ProbeHandler) Unregister(nsPath string) error {
	u.Lock()
	defer u.Unlock()

	for fd, probe := range u.probes {
		if probe.NsPath == nsPath {
			if err := syscall.EpollCtl(u.epollFd, syscall.EPOLL_CTL_DEL, int(fd), nil); err != nil {
				return fmt.Errorf("Failed to del fd from epoll events set for %s: %s", probe.Ctx.RootNode.ID, err)
			}
			delete(u.probes, fd)

			probe.stop()

			return nil
		}
	}
	return fmt.Errorf("failed to unregister, probe not found for %s", nsPath)
}

func (u *ProbeHandler) start() {
	u.wg.Add(1)
	defer u.wg.Done()

	events := make([]syscall.EpollEvent, maxEpollEvents)

	u.state.Store(service.RunningState)
	for u.state.Load() == service.RunningState {
		nevents, err := syscall.EpollWait(u.epollFd, events[:], 200)
		if err != nil {
			if errno, ok := err.(syscall.Errno); ok && errno != syscall.EINTR {
				u.Ctx.Logger.Errorf("Failed to receive from events from netlink: %s", err)
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
func (u *ProbeHandler) Start() error {
	if _, err := u.Register("", u.Ctx); err != nil {
		return err
	}
	go u.start()
	return nil
}

// Stop the probe
func (u *ProbeHandler) Stop() {
	if u.state.CompareAndSwap(service.RunningState, service.StoppingState) {
		u.wg.Wait()

		u.RLock()
		defer u.RUnlock()
		u.sriovProcessor.Stop()

		for _, probe := range u.probes {
			go probe.stop()
		}

		for _, probe := range u.probes {
			probe.wg.Wait()
		}
	}
}

// NewProbe returns a new topology netlink probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, fmt.Errorf("Failed to create epoll: %s", err)
	}

	sriovProcessor := graph.NewProcessor(ctx.Graph, ctx.Graph, graph.Metadata{"Type": "device"}, "BusInfo")
	sriovProcessor.Start()

	return &ProbeHandler{
		Ctx:            ctx,
		epollFd:        epfd,
		probes:         make(map[int32]*Probe),
		sriovProcessor: sriovProcessor,
	}, nil
}

// Register registers graph metadata decoders
func Register() {
	graph.NodeMetadataDecoders["Vfs"] = VFSMetadataDecoder

	graph.NodeMetadataDecoders["RoutingTables"] = topology.RoutingTablesMetadataDecoder
	graph.NodeMetadataDecoders["FDB"] = topology.NeighborMetadataDecoder
	graph.NodeMetadataDecoders["Neighbors"] = topology.NeighborMetadataDecoder

	graph.NodeMetadataDecoders["Metric"] = topology.InterfaceMetricMetadataDecoder
	graph.NodeMetadataDecoders["LastUpdateMetric"] = topology.InterfaceMetricMetadataDecoder
}
