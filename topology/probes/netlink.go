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
	"errors"
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
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

const (
	maxEpollEvents = 32
)

type NetLinkProbe struct {
	Graph                *graph.Graph
	Root                 *graph.Node
	state                int64
	ethtool              *ethtool.Ethtool
	netlink              *netlink.Handle
	indexToChildrenQueue map[int64][]graph.Identifier
	wg                   sync.WaitGroup
}

func (u *NetLinkProbe) linkPendingChildren(intf *graph.Node, index int64) {
	// add children of this interface that was previously added
	if children, ok := u.indexToChildrenQueue[index]; ok {
		for _, id := range children {
			child := u.Graph.GetNode(id)
			if child != nil {
				u.Graph.Link(intf, child, graph.Metadata{"RelationType": "layer2"})
			}
		}
		delete(u.indexToChildrenQueue, index)
	}
}

func (u *NetLinkProbe) linkIntfToIndex(intf *graph.Node, index int64) {
	// assuming we have only one master with this index
	parent := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if parent != nil {
		// ignore ovs-system interface as it doesn't make any sense according to
		// the following thread:
		// http://openvswitch.org/pipermail/discuss/2013-October/011657.html
		if parent.Metadata()["Name"] == "ovs-system" {
			return
		}

		if !u.Graph.AreLinked(parent, intf) {
			u.Graph.Link(parent, intf, graph.Metadata{"RelationType": "layer2"})
		}
	} else {
		// not yet the bridge so, enqueue for a later add
		u.indexToChildrenQueue[index] = append(u.indexToChildrenQueue[index], intf.ID)
	}
}

func (u *NetLinkProbe) handleIntfIsChild(intf *graph.Node, link netlink.Link) {
	// handle pending relationship
	u.linkPendingChildren(intf, int64(link.Attrs().Index))

	// interface being a part of a bridge
	if link.Attrs().MasterIndex != 0 {
		u.linkIntfToIndex(intf, int64(link.Attrs().MasterIndex))
	}

	if link.Attrs().ParentIndex != 0 {
		if _, ok := intf.Metadata()["Vlan"]; ok {
			u.linkIntfToIndex(intf, int64(int64(link.Attrs().ParentIndex)))
		}
	}
}

func (u *NetLinkProbe) handleIntfIsVeth(intf *graph.Node, link netlink.Link) {
	if link.Type() != "veth" {
		return
	}

	if index, ok := intf.Metadata()["PeerIfIndex"]; ok {
		peerResolver := func() error {
			// re get the interface from the graph since the interface could have been deleted
			if u.Graph.GetNode(intf.ID) == nil {
				return errors.New("Node not found")
			}

			// got more than 1 peer, unable to find the right one, wait for the other to discover
			peer := u.Graph.LookupFirstNode(graph.Metadata{"IfIndex": index.(int64), "Type": "veth"})
			if peer != nil && !u.Graph.AreLinked(peer, intf) {
				u.Graph.Link(peer, intf, graph.Metadata{"RelationType": "layer2", "Type": "veth"})
				return nil
			}
			return errors.New("Nodes not linked")
		}

		if index.(int64) > intf.Metadata()["IfIndex"].(int64) {
			if err := peerResolver(); err != nil {
				retryFnc := func() error {
					if u.isRunning() == false {
						return nil
					}
					u.Graph.Lock()
					defer u.Graph.Unlock()
					return peerResolver()
				}
				go common.Retry(retryFnc, 10, 200*time.Millisecond)
			}
		}
	}
}

func (u *NetLinkProbe) addGenericLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	name := link.Attrs().Name
	index := int64(link.Attrs().Index)

	var intf *graph.Node
	intf = u.Graph.LookupFirstChild(u.Root, graph.Metadata{
		"IfIndex": index,
	})

	// could be a member of ovs
	intfs := u.Graph.GetNodes(graph.Metadata{
		"Name":    name,
		"IfIndex": index,
	})
	for _, i := range intfs {
		if _, ok := i.Metadata()["UUID"]; ok {
			intf = i
			break
		}
	}

	if intf == nil {
		intf = u.Graph.NewNode(graph.GenID(), m)
	}

	if !u.Graph.AreLinked(u.Root, intf) {
		u.Graph.Link(u.Root, intf, graph.Metadata{"RelationType": "ownership"})
	}

	// ignore ovs-system interface as it doesn't make any sense according to
	// the following thread:
	// http://openvswitch.org/pipermail/discuss/2013-October/011657.html
	if name == "ovs-system" {
		return intf
	}

	return intf
}

func (u *NetLinkProbe) addBridgeLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	name := link.Attrs().Name
	index := int64(link.Attrs().Index)

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{
		"Name":    name,
		"IfIndex": index,
	})

	if intf == nil {
		intf = u.Graph.NewNode(graph.GenID(), m)
	}

	if !u.Graph.AreLinked(u.Root, intf) {
		u.Graph.Link(u.Root, intf, graph.Metadata{"RelationType": "ownership"})
	}

	u.linkPendingChildren(intf, index)

	return intf
}

func (u *NetLinkProbe) addOvsLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	name := link.Attrs().Name

	intf := u.Graph.LookupFirstNode(graph.Metadata{"Name": name, "Driver": "openvswitch"})
	if intf == nil {
		intf = u.Graph.NewNode(graph.GenID(), m)
	}

	if !u.Graph.AreLinked(u.Root, intf) {
		u.Graph.Link(u.Root, intf, graph.Metadata{"RelationType": "ownership"})
	}

	return intf
}

func (u *NetLinkProbe) getLinkIPs(link netlink.Link, family int) string {
	var ips []string

	addrs, err := u.netlink.AddrList(link, family)
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		ips = append(ips, addr.IPNet.String())
	}

	return strings.Join(ips, ",")
}

func (u *NetLinkProbe) addLinkToTopology(link netlink.Link) {
	logging.GetLogger().Debugf("Link \"%s(%d)\" added", link.Attrs().Name, link.Attrs().Index)

	u.Graph.Lock()
	defer u.Graph.Unlock()

	driver, _ := u.ethtool.DriverName(link.Attrs().Name)
	if driver == "" && link.Type() == "bridge" {
		driver = "bridge"
	}

	metadata := graph.Metadata{
		"Name":      link.Attrs().Name,
		"Type":      link.Type(),
		"EncapType": link.Attrs().EncapType,
		"IfIndex":   int64(link.Attrs().Index),
		"MAC":       link.Attrs().HardwareAddr.String(),
		"MTU":       int64(link.Attrs().MTU),
		"Driver":    driver,
	}

	if link.Type() == "veth" {
		stats, err := u.ethtool.Stats(link.Attrs().Name)
		if err != nil {
			logging.GetLogger().Errorf("Unable get stats from ethtool (%s): %s", link.Attrs().Name, err.Error())
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
		metadata["Vlan"] = vlan.VlanId
	}

	if (link.Attrs().Flags & net.FlagUp) > 0 {
		metadata["State"] = "UP"
	} else {
		metadata["State"] = "DOWN"
	}

	if link.Type() == "bond" {
		metadata["BondMode"] = link.(*netlink.Bond).Mode.String()
	}

	var intf *graph.Node

	switch driver {
	case "bridge":
		intf = u.addBridgeLinkToTopology(link, metadata)
	case "openvswitch":
		intf = u.addOvsLinkToTopology(link, metadata)
		// always prefer Type from ovs
		metadata["Type"] = intf.Metadata()["Type"]
	default:
		intf = u.addGenericLinkToTopology(link, metadata)
	}

	if intf == nil {
		return
	}

	m := intf.Metadata()

	// update metadata in case of an old interface
	updated := false
	for k, nv := range metadata {
		if ov, ok := m[k]; ok && nv == ov {
			continue
		}
		m[k] = nv
		updated = true
	}

	if updated {
		u.Graph.SetMetadata(intf, m)
	}

	u.handleIntfIsChild(intf, link)
	u.handleIntfIsVeth(intf, link)
}

func (u *NetLinkProbe) onLinkAdded(link netlink.Link) {
	if u.isRunning() == true {
		u.addLinkToTopology(link)
	}
}

func (u *NetLinkProbe) onLinkDeleted(link netlink.Link) {
	index := link.Attrs().Index

	u.Graph.Lock()
	defer u.Graph.Unlock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})

	// case of removing the interface from a bridge
	if intf != nil {
		parents := u.Graph.LookupParents(intf, graph.Metadata{"Type": "bridge"})
		for _, parent := range parents {
			u.Graph.Unlink(parent, intf)
		}
	}

	// check whether the interface has been deleted or not
	// we get a delete event when an interface is removed from a bridge
	_, err := u.netlink.LinkByIndex(index)
	if err != nil && intf != nil {
		// if openvswitch do not remove let's do the job by ovs piece of code
		if intf.Metadata()["Driver"] == "openvswitch" && intf.Metadata()["UUID"] != "" {
			u.Graph.Unlink(u.Root, intf)
		} else {
			u.Graph.DelNode(intf)
		}
	}

	delete(u.indexToChildrenQueue, int64(index))
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

func (u *NetLinkProbe) onAddressAdded(addr netlink.Addr, family int, index int) {
	u.Graph.Lock()
	defer u.Graph.Unlock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if intf == nil {
		logging.GetLogger().Errorf("No interface with index %d for new address %s", index, addr.IPNet.String())
		return
	}

	key := getFamilyKey(family)
	m := intf.Metadata()
	if v, ok := m[key]; ok {
		if strings.Contains(v.(string)+",", addr.IPNet.String()+",") {
			return
		}
	}

	ips := addr.IPNet.String()
	if v, ok := intf.Metadata()[key]; ok {
		ips = v.(string) + "," + ips
	}
	u.Graph.AddMetadata(intf, key, ips)
}

func (u *NetLinkProbe) onAddressDeleted(addr netlink.Addr, family int, index int) {
	u.Graph.Lock()
	defer u.Graph.Unlock()

	intf := u.Graph.LookupFirstChild(u.Root, graph.Metadata{"IfIndex": index})
	if intf == nil {
		logging.GetLogger().Errorf("No interface with index %d for new address %s", index, addr.IPNet.String())
		return
	}

	key := getFamilyKey(family)
	m := intf.Metadata()
	if v, ok := m[key]; ok {
		ips := strings.Split(v.(string), ",")
		for i, ip := range ips {
			if ip == addr.IPNet.String() {
				ips = append(ips[:i], ips[i+1:]...)
				break
			}
		}

		if len(ips) == 0 {
			delete(m, key)
			u.Graph.SetMetadata(intf, m)
		} else {
			u.Graph.AddMetadata(intf, key, strings.Join(ips, ","))
		}
	}
}

func (u *NetLinkProbe) initialize() {
	links, err := u.netlink.LinkList()
	if err != nil {
		logging.GetLogger().Errorf("Unable to list interfaces: %s", err.Error())
		return
	}

	for _, link := range links {
		u.addLinkToTopology(link)
	}
}

func (u *NetLinkProbe) isRunning() bool {
	return atomic.LoadInt64(&u.state) == common.RunningState
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

func (u *NetLinkProbe) start(nsPath string) {
	var context *common.NetNSContext
	var err error

	// Enter the network namespace if necessary
	if nsPath != "" {
		context, err = common.NewNetNsContext(nsPath)
		if err != nil {
			logging.GetLogger().Errorf("Failed to switch namespace: %s", err.Error())
			return
		}
	}

	// Both NewHandle and Subscribe need to done in the network namespace.
	h, err := netlink.NewHandle(syscall.NETLINK_ROUTE)
	if err != nil {
		logging.GetLogger().Errorf("Failed to create netlink handle: %s", err.Error())
		context.Close()
		return
	}
	defer h.Delete()

	s, err := nl.Subscribe(syscall.NETLINK_ROUTE, syscall.RTNLGRP_LINK, syscall.RTNLGRP_IPV4_IFADDR, syscall.RTNLGRP_IPV6_IFADDR)
	if err != nil {
		logging.GetLogger().Errorf("Failed to subscribe to netlink messages: %s", err.Error())
		context.Close()
		return
	}
	defer s.Close()

	u.ethtool, err = ethtool.NewEthtool()
	if err != nil {
		logging.GetLogger().Errorf("Failed to create ethtool object: %s", err.Error())
		context.Close()
		return
	}
	defer u.ethtool.Close()

	epfd, e := syscall.EpollCreate1(0)
	if e != nil {
		logging.GetLogger().Errorf("Failed to create epoll: %s", err.Error())
		return
	}
	defer syscall.Close(epfd)

	// Leave the network namespace
	context.Close()

	u.wg.Add(1)
	defer u.wg.Done()

	atomic.StoreInt64(&u.state, common.RunningState)
	defer atomic.StoreInt64(&u.state, common.StoppedState)

	u.netlink = h
	u.initialize()

	fd := s.GetFd()
	err = syscall.SetNonblock(fd, true)
	if err != nil {
		logging.GetLogger().Errorf("Failed to set the netlink fd as non-blocking: %s", err.Error())
		return
	}

	event := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(fd)}
	if err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
		logging.GetLogger().Errorf("Failed to control epoll: %s", err.Error())
		return
	}
	events := make([]syscall.EpollEvent, maxEpollEvents)

	for atomic.LoadInt64(&u.state) == common.RunningState {
		n, err := syscall.EpollWait(epfd, events[:], 1000)
		if err != nil {
			errno, ok := err.(syscall.Errno)
			if ok && errno != syscall.EINTR {
				logging.GetLogger().Errorf("Failed to receive from events from netlink: %s", err.Error())
			}
			continue
		}
		if n == 0 {
			continue
		}

		msgs, err := s.Receive()
		if err != nil {
			if errno, ok := err.(syscall.Errno); !ok || !errno.Temporary() {
				logging.GetLogger().Errorf("Failed to receive from netlink messages: %s", err.Error())
				return
			}
			time.Sleep(1 * time.Second)
			continue
		}

		for _, msg := range msgs {
			switch msg.Header.Type {
			case syscall.RTM_NEWLINK:
				link, err := netlink.LinkDeserialize(msg.Data)
				if err != nil {
					logging.GetLogger().Warningf("Failed to deserialize netlink message: %s", err.Error())
					continue
				}
				u.onLinkAdded(link)
			case syscall.RTM_DELLINK:
				link, err := netlink.LinkDeserialize(msg.Data)
				if err != nil {
					logging.GetLogger().Warningf("Failed to deserialize netlink message: %s", err.Error())
					continue
				}
				u.onLinkDeleted(link)
			case syscall.RTM_NEWADDR:
				addr, family, ifindex, err := parseAddr(msg.Data)
				if err != nil {
					logging.GetLogger().Warningf("Failed to parse newlink message: %s", err.Error())
					continue
				}
				u.onAddressAdded(addr, family, ifindex)
			case syscall.RTM_DELADDR:
				addr, family, ifindex, err := parseAddr(msg.Data)
				if err != nil {
					logging.GetLogger().Warningf("Failed to parse newlink message: %s", err.Error())
					continue
				}
				u.onAddressDeleted(addr, family, ifindex)
			}
		}
	}
}

func (u *NetLinkProbe) Start() {
	go u.start("")
}

func (u *NetLinkProbe) Run(nsPath string) {
	u.start(nsPath)
}

func (u *NetLinkProbe) Stop() {
	if atomic.CompareAndSwapInt64(&u.state, common.RunningState, common.StoppingState) {
		u.wg.Wait()
	}
}

func NewNetLinkProbe(g *graph.Graph, n *graph.Node) *NetLinkProbe {
	np := &NetLinkProbe{
		Graph:                g,
		Root:                 n,
		indexToChildrenQueue: make(map[int64][]graph.Identifier),
		state:                common.StoppedState,
	}
	return np
}
