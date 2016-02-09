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
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netns"
	"golang.org/x/exp/inotify"

	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/topology/graph"
)

const (
	runBaseDir = "/var/run/netns"
)

type NetNSProbe struct {
	Graph      *graph.Graph
	Root       *graph.Node
	nsnlProbes map[string]*NetNsNetLinkTopoUpdater
}

type NetNsNetLinkTopoUpdater struct {
	sync.RWMutex
	Graph   *graph.Graph
	Root    *graph.Node
	nlProbe *NetLinkProbe
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
	nu.nlProbe = NewNetLinkProbe(nu.Graph, nu.Root)
	nu.Unlock()

	/* NOTE(safchain) don't Start just Run, need to keep it alive for the time life of the netns
	 * and there is no need to have a new goroutine here
	 */
	nu.nlProbe.Run()

	nu.Lock()
	nu.nlProbe = nil
	nu.Unlock()

	logging.GetLogger().Debug("NetLinkTopoUpdater stopped for NetNS: %s", name)

	netns.Set(origns)
}

func (nu *NetNsNetLinkTopoUpdater) Stop() {
	nu.Lock()
	if nu.nlProbe != nil {
		nu.nlProbe.Stop()
	}
	nu.Unlock()
}

func NewNetNsNetLinkTopoUpdater(g *graph.Graph, n *graph.Node) *NetNsNetLinkTopoUpdater {
	return &NetNsNetLinkTopoUpdater{
		Graph: g,
		Root:  n,
	}
}

func (u *NetNSProbe) onNetNsCreated(path string) {
	name := getNetNSName(path)

	_, ok := u.nsnlProbes[name]
	if ok {
		return
	}

	u.Graph.Lock()
	defer u.Graph.Unlock()

	logging.GetLogger().Debug("Network Namespace added: %s", name)
	n := u.Graph.NewNode(graph.GenID(), graph.Metadatas{"Name": name, "Type": "netns"})
	u.Graph.Link(u.Root, n)

	nu := NewNetNsNetLinkTopoUpdater(u.Graph, n)
	go nu.Start(path)

	u.nsnlProbes[name] = nu
}

func (u *NetNSProbe) onNetNsDeleted(path string) {
	name := getNetNSName(path)

	logging.GetLogger().Debug("Network Namespace deleted: %s", name)

	nu, ok := u.nsnlProbes[name]
	if !ok {
		return
	}
	nu.Stop()

	u.Graph.Lock()
	defer u.Graph.Unlock()

	children := nu.Graph.LookupChildren(nu.Root, graph.Metadatas{})
	for _, child := range children {
		u.Graph.DelNode(child)
	}
	u.Graph.DelNode(nu.Root)

	delete(u.nsnlProbes, name)
}

func (u *NetNSProbe) initialize() {
	files, _ := ioutil.ReadDir(runBaseDir)
	for _, f := range files {

		u.onNetNsCreated(runBaseDir + "/" + f.Name())
	}
}

func (u *NetNSProbe) start() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	watcher, err := inotify.NewWatcher()
	if err != nil {
		logging.GetLogger().Error("Unable to create a new Watcher: %s", err.Error())
		return
	}

	// wait for the path creation
	for {
		_, err := os.Stat(runBaseDir)
		if err == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	err = watcher.Watch(runBaseDir)
	if err != nil {
		logging.GetLogger().Error("Unable to Watch %s: %s", runBaseDir, err.Error())
		return
	}

	u.initialize()

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

func (u *NetNSProbe) Start() {
	go u.start()
}

func (u *NetNSProbe) Stop() {
	for _, probe := range u.nsnlProbes {
		probe.Stop()
	}
}

func NewNetNSProbe(g *graph.Graph, n *graph.Node) *NetNSProbe {
	return &NetNSProbe{
		Graph:      g,
		Root:       n,
		nsnlProbes: make(map[string]*NetNsNetLinkTopoUpdater),
	}
}
