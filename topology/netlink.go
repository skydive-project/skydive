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
	"github.com/redhat-cip/skydive/topology/graph"
)

const (
	maxEpollEvents = 32
)

type NetLinkTopoUpdater struct {
	Graph             *graph.Graph
	Root              *graph.Node
	nlSocket          *nl.NetlinkSocket
	doneChan          chan struct{}
	indexTointfsQueue map[uint32][]*graph.Node
}

func (u *NetLinkTopoUpdater) handleIntfIsBridgeMember(intf *graph.Node, link netlink.Link) {
	index := uint32(link.Attrs().Index)

	// add children of this interface that haven previously added
	if children, ok := u.indexTointfsQueue[index]; ok {
		for _, child := range children {
			u.Graph.NewEdge(graph.GenID(), intf, child, nil)
		}
		delete(u.indexTointfsQueue, index)
	}

	// interface being a part of a bridge
	if link.Attrs().MasterIndex != 0 {
		index := uint32(link.Attrs().MasterIndex)

		parent := u.Graph.LookupNode(graph.Metadatas{"IfIndex": index})
		if parent != nil {
			// check the type of the parent since the index can be wrong in case of ovs
			if parent.Metadatas["Type"] == "bridge" && !parent.IsLinkedTo(intf) {
				u.Graph.NewEdge(graph.GenID(), parent, intf, nil)
			}
		} else {
			// not yet the bridge so, enqueue for a later add
			u.indexTointfsQueue[index] = append(u.indexTointfsQueue[index], intf)
		}
	}
}

func (u *NetLinkTopoUpdater) handleIntfIsVeth(intf *graph.Node, link netlink.Link) {
	if link.Type() != "veth" {
		return
	}

	stats, err := ethtool.Stats(link.Attrs().Name)
	if err != nil {
		logging.GetLogger().Error("Unable get stats from ethtool: %s", err.Error())
		return
	}

	if index, ok := stats["peer_ifindex"]; ok {
		peerResolver := func() bool {
			// re get the interface from the graph since the interface could have been deleted
			if u.Graph.GetNode(intf.ID) == nil {
				return false
			}

			// got more than 1 peer, unable to find the right one, wait for the other to discover
			peer := u.Graph.LookupNode(graph.Metadatas{"IfIndex": uint32(index), "Type": "veth"})
			if peer != nil && !peer.IsLinkedTo(intf) {
				u.Graph.NewEdge(graph.GenID(), peer, intf, graph.Metadatas{"Type": "veth"})
				return true
			}
			return false
		}

		if uint32(index) > intf.Metadatas["IfIndex"].(uint32) {
			ok := peerResolver()
			if !ok {
				// retry few second later since the right peer can be insert later
				go func() {
					time.Sleep(2 * time.Second)

					u.Graph.Lock()
					defer u.Graph.Unlock()
					peerResolver()
				}()
			}
		}
	}
}

func (u *NetLinkTopoUpdater) addGenericLinkToTopology(link netlink.Link, m graph.Metadatas) *graph.Node {
	name := link.Attrs().Name
	index := uint32(link.Attrs().Index)

	var intf *graph.Node
	if name != "lo" {
		intf = u.Graph.LookupNode(graph.Metadatas{
			"Name":    name,
			"IfIndex": index,
			"MAC":     link.Attrs().HardwareAddr.String()})
	}

	if intf == nil {
		intf = u.Graph.NewNode(graph.GenID(), m)
	}

	if !u.Root.IsLinkedTo(intf) {
		u.Root.LinkTo(intf)
	}

	u.handleIntfIsBridgeMember(intf, link)
	u.handleIntfIsVeth(intf, link)

	return intf
}

func (u *NetLinkTopoUpdater) addBridgeLinkToTopology(link netlink.Link, m graph.Metadatas) *graph.Node {
	index := uint32(link.Attrs().Index)

	intf := u.Graph.LookupNode(graph.Metadatas{"IfIndex": index})
	if intf == nil {
		intf = u.Graph.NewNode(graph.GenID(), m)
	}

	if !u.Root.IsLinkedTo(intf) {
		u.Root.LinkTo(intf)
	}

	return intf
}

func (u *NetLinkTopoUpdater) addOvsLinkToTopology(link netlink.Link, m graph.Metadatas) *graph.Node {
	name := link.Attrs().Name

	intf := u.Graph.LookupNode(graph.Metadatas{"Name": name, "Driver": "openvswitch"})
	if intf == nil {
		intf = u.Graph.NewNode(graph.GenID(), m)
	}

	if !u.Root.IsLinkedTo(intf) {
		u.Root.LinkTo(intf)
	}

	return intf
}

func (u *NetLinkTopoUpdater) addLinkToTopology(link netlink.Link) {
	logging.GetLogger().Debug("Link \"%s(%d)\" added", link.Attrs().Name, link.Attrs().Index)

	u.Graph.Lock()
	defer u.Graph.Unlock()

	driver, _ := ethtool.DriverName(link.Attrs().Name)

	metadatas := graph.Metadatas{
		"Name":    link.Attrs().Name,
		"Type":    link.Type(),
		"IfIndex": uint32(link.Attrs().Index),
		"MAC":     link.Attrs().HardwareAddr.String(),
		"MTU":     uint32(link.Attrs().MTU),
		"Driver":  driver,
	}

	if (link.Attrs().Flags & net.FlagUp) > 0 {
		metadatas["State"] = "UP"
	} else {
		metadatas["State"] = "DOWN"
	}

	var intf *graph.Node

	switch driver {
	case "bridge":
		intf = u.addBridgeLinkToTopology(link, metadatas)
	case "openvswitch":
		intf = u.addOvsLinkToTopology(link, metadatas)
	default:
		intf = u.addGenericLinkToTopology(link, metadatas)
	}

	if intf != nil {
		// update metadates if needed
		updated := true
		for k, metadata := range metadatas {
			if intf.Metadatas[k] != metadata {
				intf.Metadatas[k] = metadata
				updated = true
			}
		}

		if updated {
			u.Graph.NotifyNodeUpdated(intf)
		}
	}
}

func (u *NetLinkTopoUpdater) onLinkAdded(index int) {
	link, err := netlink.LinkByIndex(index)
	if err != nil {
		logging.GetLogger().Error("Failed to find interface %d: %s", index, err.Error())
		return
	}

	u.addLinkToTopology(link)
}

func (u *NetLinkTopoUpdater) onLinkDeleted(index int) {
	logging.GetLogger().Debug("Link %d deleted", index)

	u.Graph.Lock()
	defer u.Graph.Unlock()

	// case of removing the interface from a bridge
	intf := u.Graph.LookupNode(graph.Metadatas{"IfIndex": uint32(index)})
	if intf != nil {
		parent := intf.LookupParentNode(graph.Metadatas{"Type": "bridge"})
		if parent != nil {
			intf.UnlinkFrom(parent)
		}
	}

	// check wheter the interface has been deleted or not
	// we get a delete event when an interace is removed from a bridge
	_, err := netlink.LinkByIndex(index)
	if err != nil && intf != nil {
		if driver, ok := intf.Metadatas["Driver"]; ok {
			// if openvswitch do not remove let's do the job by ovs piece of code
			if driver == "openvswitch" {
				u.Root.UnlinkFrom(intf)
			} else {
				u.Graph.DelNode(intf)
			}
		}
	}

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

func NewNetLinkTopoUpdater(g *graph.Graph, n *graph.Node) *NetLinkTopoUpdater {
	return &NetLinkTopoUpdater{
		Graph:             g,
		Root:              n,
		doneChan:          make(chan struct{}),
		indexTointfsQueue: make(map[uint32][]*graph.Node),
	}
}
