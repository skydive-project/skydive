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

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/safchain/ethtool"

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
	nlSocket             *nl.NetlinkSocket
	state                int64
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

	stats, err := ethtool.Stats(link.Attrs().Name)
	if err != nil {
		logging.GetLogger().Errorf("Unable get stats from ethtool: %s", err.Error())
		return
	}

	if index, ok := stats["peer_ifindex"]; ok {
		peerResolver := func() error {
			// re get the interface from the graph since the interface could have been deleted
			if u.Graph.GetNode(intf.ID) == nil {
				return errors.New("Node not found")
			}

			// got more than 1 peer, unable to find the right one, wait for the other to discover
			peer := u.Graph.LookupFirstNode(graph.Metadata{"IfIndex": int64(index), "Type": "veth"})
			if peer != nil && !u.Graph.AreLinked(peer, intf) {
				u.Graph.Link(peer, intf, graph.Metadata{"RelationType": "layer2", "Type": "veth"})
				return nil
			}
			return errors.New("Nodes not linked")
		}

		if int64(index) > intf.Metadata()["IfIndex"].(int64) {
			if err := peerResolver(); err != nil {
				retryFnc := func() error {
					u.Graph.Lock()
					defer u.Graph.Unlock()
					return peerResolver()
				}
				go common.Retry(retryFnc, 10, 200*time.Millisecond)
			}
		}
	}
}

func (u *NetLinkProbe) handleIntfIsBond(intf *graph.Node, link netlink.Link) {
	if link.Type() != "bond" {
		return
	}

	bond := link.(*netlink.Bond)
	u.Graph.AddMetadata(intf, "BondMode", bond.Mode.String())

	// TODO(safchain) Add more info there like xmit_hash_policy
}

func (u *NetLinkProbe) addGenericLinkToTopology(link netlink.Link, m graph.Metadata) *graph.Node {
	name := link.Attrs().Name
	index := int64(link.Attrs().Index)

	var intf *graph.Node
	intf = u.Graph.LookupFirstChild(u.Root, graph.Metadata{
		"IfIndex": index,
	})

	// could be a member of ovs
	intfs := u.Graph.LookupNodes(graph.Metadata{
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

	if intf == nil {
		return nil
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

	u.handleIntfIsChild(intf, link)
	u.handleIntfIsVeth(intf, link)
	u.handleIntfIsBond(intf, link)

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

func (u *NetLinkProbe) getLinkIPV4Addr(link netlink.Link) string {
	var ipv4 []string

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		ipv4 = append(ipv4, addr.IPNet.String())
	}

	return strings.Join(ipv4, ", ")
}

func (u *NetLinkProbe) addLinkToTopology(link netlink.Link) {
	logging.GetLogger().Debugf("Link \"%s(%d)\" added", link.Attrs().Name, link.Attrs().Index)

	u.Graph.Lock()
	defer u.Graph.Unlock()

	driver, _ := ethtool.DriverName(link.Attrs().Name)
	if driver == "" && link.Type() == "bridge" {
		driver = "bridge"
	}

	metadata := graph.Metadata{
		"Name":    link.Attrs().Name,
		"Type":    link.Type(),
		"IfIndex": int64(link.Attrs().Index),
		"MAC":     link.Attrs().HardwareAddr.String(),
		"MTU":     int64(link.Attrs().MTU),
		"Driver":  driver,
	}

	ipv4 := u.getLinkIPV4Addr(link)
	if len(ipv4) > 0 {
		metadata["IPV4"] = ipv4
	}

	if vlan, ok := link.(*netlink.Vlan); ok {
		metadata["Vlan"] = vlan.VlanId
	}

	if (link.Attrs().Flags & net.FlagUp) > 0 {
		metadata["State"] = "UP"
	} else {
		metadata["State"] = "DOWN"
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

	// merge metadata if the interface returned is not a new one
	if intf != nil {
		m := intf.Metadata()

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
	}
}

func (u *NetLinkProbe) onLinkAdded(index int) {
	fnc := func() error {
		link, err := netlink.LinkByIndex(index)
		if err != nil {
			return err
		}
		u.addLinkToTopology(link)
		return nil
	}

	if err := common.Retry(fnc, 1, time.Millisecond*200); err != nil {
		logging.GetLogger().Warningf("Failed to find interface %d: %s", index, err.Error())
	}
}

func (u *NetLinkProbe) onLinkDeleted(index int) {
	logging.GetLogger().Debugf("Link %d deleted", index)

	u.Graph.Lock()
	defer u.Graph.Unlock()

	var intf *graph.Node

	intfs := u.Graph.LookupNodes(graph.Metadata{"IfIndex": int64(index)})
	switch l := len(intfs); {
	case l == 1:
		intf = intfs[0]
	case l > 1:
	Loop:
		for _, i := range intfs {
			parents := u.Graph.LookupParents(i, nil)
			for _, parent := range parents {
				if parent.ID == u.Root.ID {
					intf = i
					break Loop
				}
			}
		}
	}

	// case of removing the interface from a bridge
	if intf != nil {
		parents := u.Graph.LookupParents(intf, graph.Metadata{"Type": "bridge"})
		for _, parent := range parents {
			u.Graph.Unlink(parent, intf)
		}
	}

	// check whether the interface has been deleted or not
	// we get a delete event when an interace is removed from a bridge
	_, err := netlink.LinkByIndex(index)
	if err != nil && intf != nil {
		// if openvswitch do not remove let's do the job by ovs piece of code
		if intf.Metadata()["Driver"] == "openvswitch" {
			u.Graph.Unlink(u.Root, intf)
		} else {
			u.Graph.DelNode(intf)
		}
	}

	delete(u.indexToChildrenQueue, int64(index))
}

func (u *NetLinkProbe) initialize() {
	links, err := netlink.LinkList()
	if err != nil {
		logging.GetLogger().Errorf("Unable to list interfaces: %s", err.Error())
		return
	}

	for _, link := range links {
		u.addLinkToTopology(link)
	}
}

func (u *NetLinkProbe) start() {
	s, err := nl.Subscribe(syscall.NETLINK_ROUTE, syscall.RTNLGRP_LINK)
	if err != nil {
		logging.GetLogger().Errorf("Failed to subscribe to netlink RTNLGRP_LINK messages: %s", err.Error())
		return
	}
	u.nlSocket = s
	defer u.nlSocket.Close()

	fd := u.nlSocket.GetFd()

	err = syscall.SetNonblock(fd, true)
	if err != nil {
		logging.GetLogger().Errorf("Failed to set the netlink fd as non-blocking: %s", err.Error())
		return
	}

	epfd, e := syscall.EpollCreate1(0)
	if e != nil {
		logging.GetLogger().Errorf("Failed to create epoll: %s", err.Error())
		return
	}
	defer syscall.Close(epfd)

	u.initialize()

	event := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(fd)}
	if e = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); e != nil {
		logging.GetLogger().Errorf("Failed to control epoll: %s", err.Error())
		return
	}

	events := make([]syscall.EpollEvent, maxEpollEvents)

	u.wg.Add(1)
	defer u.wg.Done()

	atomic.StoreInt64(&u.state, common.RunningState)
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
			logging.GetLogger().Errorf("Failed to receive from netlink messages: %s", err.Error())

			time.Sleep(1 * time.Second)
			continue
		}

		for _, msg := range msgs {
			switch msg.Header.Type {
			case syscall.RTM_NEWLINK:
				ifmsg := nl.DeserializeIfInfomsg(msg.Data)
				u.onLinkAdded(int(ifmsg.Index))
			case syscall.RTM_DELLINK:
				ifmsg := nl.DeserializeIfInfomsg(msg.Data)
				u.onLinkDeleted(int(ifmsg.Index))
			}
		}
	}
}

func (u *NetLinkProbe) Start() {
	go u.start()
}

func (u *NetLinkProbe) Run() {
	u.start()
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
