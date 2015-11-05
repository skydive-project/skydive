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
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netns"
	"golang.org/x/exp/inotify"

	"github.com/redhat-cip/skydive/logging"
)

const (
	runBaseDir = "/var/run/netns"
)

type NetNSTopoUpdater struct {
	Topology     *Topology
	nsNlUpdaters map[string]*NetNsNetLinkTopoUpdater
}

type NetNsNetLinkTopoUpdater struct {
	sync.RWMutex
	Container *Container
	nlUpdater *NetLinkTopoUpdater
}

func getNetNSName(path string) string {
	s := strings.Split(path, "/")
	return s[len(s)-1]
}

func (nu *NetNsNetLinkTopoUpdater) Start(path string) {
	name := getNetNSName(path)

	logging.GetLogger().Debug("Starting NetLinkTopoUpdater for NetNS: %s", name)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		logging.GetLogger().Error("Error while switching from root ns to %s: %s", name, err.Error())
		return
	}
	defer origns.Close()

	time.Sleep(1 * time.Second)

	newns, err := netns.GetFromPath(path)
	if err != nil {
		logging.GetLogger().Error("Error while switching from root ns to %s: %s", name, err.Error())
		return
	}
	defer newns.Close()

	err = netns.Set(newns)
	if err != nil {
		logging.GetLogger().Error("Error while switching from root ns to %s: %s", name, err.Error())
		return
	}

	/* start a netlinks updater inside this namespace */
	nu.Lock()
	nu.nlUpdater = NewNetLinkTopoUpdater(nu.Container)
	nu.Unlock()

	nu.nlUpdater.Start()

	nu.Lock()
	nu.nlUpdater = nil
	nu.Unlock()

	logging.GetLogger().Debug("NetLinkTopoUpdater stopped for NetNS: %s", name)

	netns.Set(origns)
}

func (nu *NetNsNetLinkTopoUpdater) Stop() {
	nu.Lock()
	if nu.nlUpdater != nil {
		nu.nlUpdater.Stop()
	}
	nu.Unlock()
}

func NewNetNsNetLinkTopoUpdater(c *Container) *NetNsNetLinkTopoUpdater {
	return &NetNsNetLinkTopoUpdater{
		Container: c,
	}
}

func (u *NetNSTopoUpdater) onNetNsCreated(path string) {
	logging.GetLogger().Debug("Network Namespace added: %s", getNetNSName(path))
	container := u.Topology.NewContainer(path, NetNs)

	nu := NewNetNsNetLinkTopoUpdater(container)
	go nu.Start(path)

	u.nsNlUpdaters[path] = nu
}

func (u *NetNSTopoUpdater) onNetNsDeleted(path string) {
	logging.GetLogger().Debug("Network Namespace deleted: %s", getNetNSName(path))

	nu, ok := u.nsNlUpdaters[path]
	if !ok {
		return
	}
	nu.Stop()

	u.Topology.DelContainer(nu.Container.ID)

	delete(u.nsNlUpdaters, path)
}

func (u *NetNSTopoUpdater) start() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	watcher, err := inotify.NewWatcher()
	if err != nil {
		logging.GetLogger().Error("Unable to create a new Watcher: %s", err.Error())
		return
	}
	err = watcher.Watch(runBaseDir)
	if err != nil {
		logging.GetLogger().Error("Unable to Watch %s: %s", runBaseDir, err.Error())
		return
	}
	for {
		select {
		case ev := <-watcher.Event:
			if ev.Mask&inotify.IN_CREATE > 0 {
				u.onNetNsCreated(ev.Name)
			}
			if ev.Mask&inotify.IN_DELETE > 0 {
				u.onNetNsDeleted(ev.Name)
			}

		case err := <-watcher.Error:
			logging.GetLogger().Error("Error while watching network namespace: %s", err.Error())
		}
	}
}

func (u *NetNSTopoUpdater) Start() {
	go u.start()
}

func NewNetNSTopoUpdater(topo *Topology) *NetNSTopoUpdater {
	return &NetNSTopoUpdater{
		Topology:     topo,
		nsNlUpdaters: make(map[string]*NetNsNetLinkTopoUpdater),
	}
}
