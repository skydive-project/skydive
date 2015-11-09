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
	"syscall"
	"time"

	/* TODO(safchain) has to be removed when the
	   PR https://github.com/vishvananda/netlink/pull/61
	   will be merged
	*/
	"github.com/safchain/netlink"
	"github.com/safchain/netlink/nl"
	//"github.com/vishvananda/netlink"
	//"github.com/vishvananda/netlink/nl"

	"github.com/redhat-cip/skydive/logging"
)

const (
	maxEpollEvents = 32
)

type NetLinkTopoUpdater struct {
	Container *Container
	linkCache map[int]netlink.LinkAttrs
	nlSocket  *nl.NetlinkSocket
	doneChan  chan struct{}
}

func (u *NetLinkTopoUpdater) addLinkToTopology(link netlink.Link) {
	u.linkCache[link.Attrs().Index] = *link.Attrs()

	/* create a port, attach it to the current container, then create attach
	   a single interface to this port */
	port := u.Container.Topology.NewPort(link.Attrs().Name, u.Container)
	intf := u.Container.Topology.NewInterface(link.Attrs().Name, port)

	/* TODO(safchain) Add more metadatas here */
	intf.Metadatas["mac"] = link.Attrs().HardwareAddr.String()
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
	u.Container.Topology.DelPort(attrs.Name)

	delete(u.linkCache, index)
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
	logging.GetLogger().Debug("Start NetLink Topo Updater for container: %s", u.Container.ID)

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

func NewNetLinkTopoUpdater(container *Container) *NetLinkTopoUpdater {
	return &NetLinkTopoUpdater{
		Container: container,
		linkCache: make(map[int]netlink.LinkAttrs),
		doneChan:  make(chan struct{}),
	}
}
