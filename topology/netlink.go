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
	"net"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/safchain/ethtool"

	"github.com/redhat-cip/skydive/logging"
)

const (
	maxEpollEvents = 32
)

type NetLinkTopoUpdater struct {
	NetNs             *NetNs
	linkCache         map[int]netlink.LinkAttrs
	nlSocket          *nl.NetlinkSocket
	doneChan          chan struct{}
	indexTointfsQueue map[uint32][]*Interface
}

func (u *NetLinkTopoUpdater) handleIntfIsBridgeMember(intf *Interface, link netlink.Link) {
	index := uint32(link.Attrs().Index)

	// add children of this interface that haven previously added
	if children, ok := u.indexTointfsQueue[index]; ok {
		for _, child := range children {
			intf.AddInterface(child)
		}
		delete(u.indexTointfsQueue, index)
	}

	// interface being a part of a bridge
	if link.Attrs().MasterIndex != 0 {
		index := uint32(link.Attrs().MasterIndex)

		parent := u.NetNs.Topology.LookupInterface(LookupByIfIndex(index), NetNSScope|OvsScope)
		if parent != nil {
			parent.AddInterface(intf)
		} else {
			// not yet the bridge so, enqueue for a later add
			u.indexTointfsQueue[index] = append(u.indexTointfsQueue[index], intf)
		}
	}
}

func (u *NetLinkTopoUpdater) handleIntfIsVeth(intf *Interface, link netlink.Link) {
	if link.Type() != "veth" {
		return
	}

	stats, err := ethtool.Stats(link.Attrs().Name)
	if err != nil {
		logging.GetLogger().Error("Unable get stats from ethtool: %s", err.Error())
		return
	}

	if index, ok := stats["peer_ifindex"]; ok {
		peer := u.NetNs.Topology.LookupInterface(LookupByIfIndex(uint32(index)), NetNSScope|OvsScope)

		// set the peer only if it is of type veth
		if peer != nil && peer.Type == "veth" {
			intf.SetPeer(peer)
		}
	}
}

func (u *NetLinkTopoUpdater) addGenericLinkToTopology(link netlink.Link) *Interface {
	name := link.Attrs().Name
	mac := link.Attrs().HardwareAddr.String()

	var intf *Interface
	if name != "lo" {
		intf = u.NetNs.Topology.LookupInterface(LookupByMac(name, mac), NetNSScope|OvsScope)
	}

	if intf == nil {
		index := uint32(link.Attrs().Index)
		intf = u.NetNs.NewInterface(name, index)
	} else {
		u.NetNs.AddInterface(intf)
	}

	u.handleIntfIsBridgeMember(intf, link)
	u.handleIntfIsVeth(intf, link)

	return intf
}

func (u *NetLinkTopoUpdater) addOvsLinkToTopology(link netlink.Link) *Interface {
	name := link.Attrs().Name

	intf := u.NetNs.Topology.LookupInterface(LookupByType(name, "openvswitch"), OvsScope)
	if intf == nil {
		intf = u.NetNs.NewInterface(name, uint32(link.Attrs().Index))
	} else {
		u.NetNs.AddInterface(intf)
	}

	return intf
}

func (u *NetLinkTopoUpdater) addLinkToTopology(link netlink.Link) {
	var intf *Interface

	switch link.Type() {
	case "openvswitch":
		intf = u.addOvsLinkToTopology(link)
	default:
		intf = u.addGenericLinkToTopology(link)
	}

	if intf != nil {
		intf.SetType(link.Type())
		intf.SetIndex(uint32(link.Attrs().Index))
		intf.SetMac(link.Attrs().HardwareAddr.String())
		intf.SetMetadata("MTU", uint32(link.Attrs().MTU))

		if (link.Attrs().Flags & net.FlagUp) > 0 {
			intf.SetMetadata("State", "UP")
		} else {
			intf.SetMetadata("State", "DOWN")
		}
	}

	u.linkCache[link.Attrs().Index] = *link.Attrs()
}

func (u *NetLinkTopoUpdater) onLinkAdded(index int) {
	logging.GetLogger().Debug("Link added: %d", index)
	link, err := netlink.LinkByIndex(index)
	if err != nil {
		logging.GetLogger().Error("Failed to find interface %d: %s", index, err.Error())
		return
	}

	u.addLinkToTopology(link)
}

func (u *NetLinkTopoUpdater) onLinkDeleted(index int) {
	logging.GetLogger().Debug("Link deleted: %d", index)

	attrs, ok := u.linkCache[index]
	if !ok {
		return
	}

	// case of removing the interface from a bridge
	intf := u.NetNs.Topology.LookupInterface(LookupByIfIndex(uint32(index)), NetNSScope)
	if intf != nil && intf.Parent != nil {
		intf.Parent.DelInterface(attrs.Name)
	}

	// check wheter the interface has been deleted or not
	// we get a delete event when an interace is removed from a bridge
	_, err := netlink.LinkByIndex(index)
	if err != nil {
		u.NetNs.DelInterface(attrs.Name)
	}

	delete(u.linkCache, index)
	delete(u.indexTointfsQueue, uint32(index))
}

func (u *NetLinkTopoUpdater) initialize() {
	links, err := netlink.LinkList()
	if err != nil {
		logging.GetLogger().Error("Unable to list interfaces: %s", err.Error())
		return
	}

	for _, link := range links {
		u.addLinkToTopology(link)
	}
}

func (u *NetLinkTopoUpdater) start() {
	logging.GetLogger().Debug("Start NetLink Topo Updater for NetNs: %s", u.NetNs.ID)

	s, err := nl.Subscribe(syscall.NETLINK_ROUTE, syscall.RTNLGRP_LINK)
	if err != nil {
		logging.GetLogger().Error("Failed to subscribe to netlink RTNLGRP_LINK messages: %s", err.Error())
		return
	}
	u.nlSocket = s

	fd := u.nlSocket.GetFd()

	err = syscall.SetNonblock(fd, true)
	if err != nil {
		logging.GetLogger().Error("Failed to set the netlink fd as non-blocking: %s", err.Error())
		return
	}

	epfd, e := syscall.EpollCreate1(0)
	if e != nil {
		logging.GetLogger().Error("Failed to set the netlink fd as non-blocking: %s", err.Error())
		return
	}
	defer syscall.Close(epfd)

	u.initialize()

	event := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(fd)}
	if e = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); e != nil {
		logging.GetLogger().Error("Failed to set the netlink fd as non-blocking: %s", err.Error())
		return
	}

	events := make([]syscall.EpollEvent, maxEpollEvents)

Loop:
	for {
		n, err := syscall.EpollWait(epfd, events[:], 1000)
		if err != nil {
			logging.GetLogger().Error("Failed to receive from netlink messages: %s", err.Error())
			continue
		}

		if n == 0 {
			select {
			case <-u.doneChan:
				logging.GetLogger().Debug("WHOU")

				break Loop
			default:
				continue
			}
		}

		msgs, err := s.Receive()
		if err != nil {
			logging.GetLogger().Error("Failed to receive from netlink messages: %s", err.Error())

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

	u.nlSocket.Close()
}

func (u *NetLinkTopoUpdater) Start() {
	go u.start()
}

func (u *NetLinkTopoUpdater) Run() {
	u.start()
}

func (u *NetLinkTopoUpdater) Stop() {
	u.doneChan <- struct{}{}
}

func NewNetLinkTopoUpdater(n *NetNs) *NetLinkTopoUpdater {
	return &NetLinkTopoUpdater{
		NetNs:             n,
		linkCache:         make(map[int]netlink.LinkAttrs),
		doneChan:          make(chan struct{}),
		indexTointfsQueue: make(map[uint32][]*Interface),
	}
}
