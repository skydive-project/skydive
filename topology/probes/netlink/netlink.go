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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/safchain/ethtool"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
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
	id           graph.Identifier
	relationType string
	metadata     graph.Metadata
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
	links                map[int]*graph.Node
	state                int64
	wg                   sync.WaitGroup
	quit                 chan bool
	netNsNameTry         map[graph.Identifier]int
	sriovProcessor       *graph.Processor
}

// Probe describes a list NetLink NameSpace probe to enhance the graph
type Probe struct {
	common.RWMutex
	Graph          *graph.Graph
	hostNode       *graph.Node
	epollFd        int
	probes         map[int32]*NetNsProbe
	state          int64
	wg             sync.WaitGroup
	sriovProcessor *graph.Processor
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
			child := u.Graph.GetNode(link.id)
			if child != nil {
				topology.AddLink(u.Graph, intf, child, link.relationType, link.metadata)
			}
		}
		delete(u.indexToChildrenQueue, index)
	}
}

func (u *NetNsProbe) linkIntfToIndex(intf *graph.Node, index int64, relationType string, m graph.Metadata) {
	// assuming we have only one master with this index
	parent := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if parent != nil {
		// ignore ovs-system interface as it doesn't make any sense according to
		// the following thread:
		// http://openvswitch.org/pipermail/discuss/2013-October/011657.html
		if name, _ := parent.GetFieldString("Name"); name == "ovs-system" {
			return
		}

		if !topology.HaveLink(u.Graph, parent, intf, relationType) {
			topology.AddLink(u.Graph, parent, intf, relationType, m)
		}
	} else {
		// not yet the bridge so, enqueue for a later add
		link := pendingLink{id: intf.ID, relationType: relationType, metadata: m}
		u.indexToChildrenQueue[index] = append(u.indexToChildrenQueue[index], link)
	}
}

func (u *NetNsProbe) handleIntfIsChild(intf *graph.Node, link netlink.Link) {
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
		var err error
		intf, err = u.Graph.NewNode(graph.GenID(), m)
		if err != nil {
			logging.GetLogger().Error(err)
			return nil
		}
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

func (u *NetNsProbe) getNeighbors(index, family int) *Neighbors {
	var neighbors Neighbors

	neighList, err := u.handle.NeighList(index, family)
	if err == nil && len(neighList) > 0 {
		for i, neigh := range neighList {
			neighbors = append(neighbors, &Neighbor{
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

func (u *NetNsProbe) updateLinkNetNsName(intf *graph.Node, link netlink.Link, metadata graph.Metadata) bool {
	var context *common.NetNSContext

	lnsid := link.Attrs().NetNsID

	nodes := u.Graph.GetNodes(graph.Metadata{"Type": "netns"})
	for _, n := range nodes {
		if path, err := n.GetFieldString("Path"); err == nil {
			if u.NsPath != "" {
				context, err = common.NewNetNsContext(u.NsPath)
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

func (u *NetNsProbe) updateLinkNetNs(intf *graph.Node, link netlink.Link, metadata graph.Metadata) {
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

func (u *NetNsProbe) addLinkToTopology(link netlink.Link) {
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

	if neighbors := u.getNeighbors(attrs.Index, syscall.AF_BRIDGE); len(*neighbors) > 0 {
		metadata["FDB"] = neighbors
	}

	neighbors := u.getNeighbors(attrs.Index, syscall.AF_INET)
	*neighbors = append(*neighbors, *u.getNeighbors(attrs.Index, syscall.AF_INET6)...)
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
			logging.GetLogger().Errorf("Unable get stats from ethtool (%s): %s", attrs.Name, err)
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

	businfo, err := u.ethtool.BusInfo(attrs.Name)
	if err != nil && err != syscall.ENODEV {
		logging.GetLogger().Debugf(
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

	go u.handleSriov(u.Graph, intf, attrs.Index, businfo, attrs.Vfs, attrs.Name)

	u.updateLinkNetNs(intf, link, metadata)

	// merge metadata
	tr := u.Graph.StartMetadataTransaction(intf)
	for k, v := range metadata {
		tr.AddMetadata(k, v)
	}
	tr.Commit()

	u.handleIntfIsChild(intf, link)
	u.handleIntfIsVeth(intf, link)
}

func (u *NetNsProbe) getRoutingTables(link netlink.Link, table int) *RoutingTables {
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

	routingTableMap := make(map[int]*RoutingTable)
	for _, r := range routeList {
		routingTable, ok := routingTableMap[r.Table]
		if !ok {
			routingTable = &RoutingTable{ID: int64(r.Table), Src: r.Src}
		}

		var prefix net.IPNet
		if r.Dst != nil {
			prefix = *r.Dst
		} else {
			if len(r.Gw) == net.IPv4len {
				prefix = IPv4DefaultRoute
			} else {
				prefix = IPv6DefaultRoute
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

	var result RoutingTables
	for _, r := range routingTableMap {
		result = append(result, r)
	}
	return &result
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
			if err := u.Graph.Unlink(parent, intf); err != nil {
				logging.GetLogger().Error(err)
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
			err = u.Graph.Unlink(u.Root, intf)
		} else {
			delete(u.netNsNameTry, intf.ID)

			err = u.Graph.DelNode(intf)
		}

		if err != nil {
			logging.GetLogger().Error(err)
		}
	}
	u.Graph.Unlock()

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

func (u *NetNsProbe) onRoutingTablesChanged(index int64, rts *RoutingTables) {
	u.Graph.Lock()
	defer u.Graph.Unlock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if intf == nil {
		if _, err := u.handle.LinkByIndex(int(index)); err == nil {
			logging.GetLogger().Errorf("Unable to find interface with index %d to add a new Route", index)
		}
		return
	}
	_, err := intf.GetField("RoutingTables")
	if rts == nil && err == nil {
		err = u.Graph.DelMetadata(intf, "RoutingTables")
	} else if rts != nil {
		err = u.Graph.AddMetadata(intf, "RoutingTables", rts)
	} else {
		err = nil
	}

	if err != nil {
		logging.GetLogger().Error(err)
	}
}

func (u *NetNsProbe) onAddressAdded(addr netlink.Addr, family int, index int64) {
	u.Graph.Lock()
	defer u.Graph.Unlock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if intf == nil {
		if _, err := u.handle.LinkByIndex(int(index)); err == nil {
			logging.GetLogger().Errorf("Unable to find interface with index %d to add address %s", index, addr.IPNet)
		}
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

	if err := u.Graph.AddMetadata(intf, key, append(ips, addr.IPNet.String())); err != nil {
		logging.GetLogger().Error(err)
	}
}

func (u *NetNsProbe) onAddressDeleted(addr netlink.Addr, family int, index int64) {
	u.Graph.Lock()
	defer u.Graph.Unlock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if intf == nil {
		if _, err := u.handle.LinkByIndex(int(index)); err == nil {
			logging.GetLogger().Errorf("Unable to find interface with index %d to del address %s", index, addr.IPNet)
		}
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
			err = u.Graph.DelMetadata(intf, key)
		} else {
			err = u.Graph.AddMetadata(intf, key, ips)
		}

		if err != nil {
			logging.GetLogger().Error(err)
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
		attrs := link.Attrs()

		logging.GetLogger().Debugf("Initialize ADD %s(%d,%s) within %s", attrs.Name, attrs.Index, link.Type(), u.Root.ID)
		u.Graph.Lock()
		if u.Graph.LookupFirstChild(u.Root, graph.Metadata{"Name": attrs.Name, "IfIndex": int64(attrs.Index)}) == nil {
			u.addLinkToTopology(link)
		}
		u.Graph.Unlock()
	}
}

func (u *NetNsProbe) parseRtMsg(m []byte) (*RoutingTables, int, error) {
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

func (u *NetNsProbe) isRunning() bool {
	return atomic.LoadInt64(&u.state) == common.RunningState
}

func (u *NetNsProbe) cloneLinkNodes() map[int]*graph.Node {
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

func (u *NetNsProbe) updateIntfMetric(now, last time.Time) {
	for index, node := range u.cloneLinkNodes() {
		if link, err := u.handle.LinkByIndex(index); err == nil {
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

func (u *NetNsProbe) updateIntfFeatures(name string, metadata graph.Metadata) {
	features, err := u.ethtool.Features(name)
	if err != nil {
		logging.GetLogger().Warningf("Unable to retrieve feature of %s: %s", name, err)
	}

	if len(features) > 0 {
		metadata["Features"] = features
	}
}

func (u *NetNsProbe) updateIntfs() {
	for _, node := range u.cloneLinkNodes() {
		u.Graph.RLock()
		driver, _ := node.GetFieldString("Driver")
		name, _ := node.GetFieldString("Name")
		u.Graph.RUnlock()

		if driver == "" {
			continue
		}

		metadata := make(graph.Metadata)
		u.updateIntfFeatures(name, metadata)

		if link, err := u.handle.LinkByName(name); err == nil {
			u.Graph.RLock()
			u.updateLinkNetNs(node, link, metadata)
			u.Graph.RUnlock()
		}

		u.Graph.Lock()
		tr := u.Graph.StartMetadataTransaction(node)
		for k, v := range metadata {
			tr.AddMetadata(k, v)
		}
		tr.Commit()
		u.Graph.Unlock()
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
			rts, index, err := u.parseRtMsg(msg.Data)
			if err != nil {
				logging.GetLogger().Warningf("Failed to get Routes: %s", err)
				continue
			}
			u.onRoutingTablesChanged(int64(index), rts)
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

func newNetNsProbe(g *graph.Graph, root *graph.Node, nsPath string, sriovProcessor *graph.Processor) (*NetNsProbe, error) {
	probe := &NetNsProbe{
		Graph:                g,
		Root:                 root,
		NsPath:               nsPath,
		indexToChildrenQueue: make(map[int64][]pendingLink),
		links:                make(map[int]*graph.Node),
		quit:                 make(chan bool),
		netNsNameTry:         make(map[graph.Identifier]int),
		sriovProcessor:       sriovProcessor,
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
	probe, err := newNetNsProbe(u.Graph, root, nsPath, u.sriovProcessor)
	if err != nil {
		return nil, err
	}

	event := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(probe.epollFd)}
	if err := syscall.EpollCtl(u.epollFd, syscall.EPOLL_CTL_ADD, probe.epollFd, &event); err != nil {
		probe.closeFds()
		return nil, fmt.Errorf("Failed to add fd to epoll events set for %s: %s", root.String(), err)
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
				return fmt.Errorf("Failed to del fd from epoll events set for %s: %s", probe.Root.ID, err)
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
	u.Register("", u.hostNode)
	go u.start()
}

// Stop the probe
func (u *Probe) Stop() {
	if atomic.CompareAndSwapInt64(&u.state, common.RunningState, common.StoppingState) {
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

// NewProbe creates a new netlink probe
func NewProbe(g *graph.Graph, hostNode *graph.Node) (*Probe, error) {
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, fmt.Errorf("Failed to create epoll: %s", err)
	}
	sriovProcessor := graph.NewProcessor(g, g, graph.Metadata{"Type": "device"}, "BusInfo")
	sriovProcessor.Start()
	return &Probe{
		Graph:          g,
		hostNode:       hostNode,
		epollFd:        epfd,
		probes:         make(map[int32]*NetNsProbe),
		sriovProcessor: sriovProcessor,
	}, nil
}
