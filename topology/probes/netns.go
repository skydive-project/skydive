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
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
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
	sync.RWMutex

	Graph       *graph.Graph
	Root        *graph.Node
	nsnlProbes  map[string]*NetNsNetLinkTopoUpdater
	pathToNetNS map[string]*NetNs
}

type NetNs struct {
	path string
	dev  uint64
	ino  uint64
}

type NetNsNetLinkTopoUpdater struct {
	sync.RWMutex
	Graph    *graph.Graph
	Root     *graph.Node
	nlProbe  *NetLinkProbe
	useCount int
}

func getNetNSName(path string) string {
	s := strings.Split(path, "/")
	return s[len(s)-1]
}

func (ns *NetNs) String() string {
	return fmt.Sprintf("%d,%d", ns.dev, ns.ino)
}

func (nu *NetNsNetLinkTopoUpdater) Start(ns *NetNs) {
	logging.GetLogger().Debugf("Starting NetLinkTopoUpdater for NetNS: %s", ns.path)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		logging.GetLogger().Errorf("Error while switching from root ns to %s: %s", ns.path, err.Error())
		return
	}
	defer origns.Close()

	time.Sleep(1 * time.Second)

	newns, err := netns.GetFromPath(ns.path)
	if err != nil {
		logging.GetLogger().Errorf("Error while switching from root ns to %s: %s", ns.path, err.Error())
		return
	}
	defer newns.Close()

	err = netns.Set(newns)
	if err != nil {
		logging.GetLogger().Errorf("Error while switching from root ns to %s: %s", ns.path, err.Error())
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

	logging.GetLogger().Debugf("NetLinkTopoUpdater stopped for NetNS: %s", ns.path)

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
		Graph:    g,
		Root:     n,
		useCount: 1,
	}
}

func (u *NetNSProbe) Register(path string, extraMetadata graph.Metadata) *graph.Node {
	ns, ok := u.pathToNetNS[path]
	if !ok {
		var s syscall.Stat_t
		fd, err := syscall.Open(path, syscall.O_RDONLY, 0)
		if err != nil {
			return nil
		}
		defer syscall.Close(fd)
		if err := syscall.Fstat(fd, &s); err != nil {
			return nil
		}
		ns = &NetNs{path: path, dev: s.Dev, ino: s.Ino}
		u.pathToNetNS[path] = ns
	}

	u.Lock()
	defer u.Unlock()

	nsString := ns.String()
	probe, ok := u.nsnlProbes[nsString]
	if ok {
		probe.useCount++
		logging.GetLogger().Debugf("Increasing counter for namespace %s to %d", nsString, probe.useCount)
		return probe.Root
	}

	u.Graph.Lock()
	defer u.Graph.Unlock()

	logging.GetLogger().Debugf("Network Namespace added: %s", nsString)
	metadata := graph.Metadata{"Name": getNetNSName(path), "Type": "netns"}
	if extraMetadata != nil {
		for k, v := range extraMetadata {
			metadata[k] = v
		}
	}
	n := u.Graph.NewNode(graph.GenID(), metadata)
	u.Graph.Link(u.Root, n, graph.Metadata{"RelationType": "ownership"})

	nu := NewNetNsNetLinkTopoUpdater(u.Graph, n)
	go nu.Start(ns)

	u.nsnlProbes[nsString] = nu

	return n
}

func (u *NetNSProbe) Unregister(path string) {
	logging.GetLogger().Debugf("Unregister Network Namespace: %s", path)

	ns, ok := u.pathToNetNS[path]
	if !ok {
		return
	}

	u.Lock()
	defer u.Unlock()

	delete(u.pathToNetNS, path)
	nsString := ns.String()
	nu, ok := u.nsnlProbes[nsString]
	if !ok {
		logging.GetLogger().Debugf("No existing Network Namespace found: %s (%s)", nsString)
		return
	}

	if nu.useCount > 1 {
		nu.useCount--
		logging.GetLogger().Debugf("Decremented counter for namespace %s to %d", nsString, nu.useCount)
		return
	}

	nu.Stop()
	logging.GetLogger().Debugf("Network Namespace deleted: %s", nsString)

	u.Graph.Lock()
	defer u.Graph.Unlock()

	children := nu.Graph.LookupChildren(nu.Root, graph.Metadata{})
	for _, child := range children {
		u.Graph.DelNode(child)
	}
	u.Graph.DelNode(nu.Root)

	delete(u.nsnlProbes, nsString)
}

func (u *NetNSProbe) initialize() {
	files, _ := ioutil.ReadDir(runBaseDir)
	for _, f := range files {
		u.Register(runBaseDir+"/"+f.Name(), nil)
	}
}

func (u *NetNSProbe) start() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// wait for the path creation
	for {
		_, err := os.Stat(runBaseDir)
		if err == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	watcher, err := inotify.NewWatcher()
	if err != nil {
		logging.GetLogger().Errorf("Unable to create a new Watcher: %s", err.Error())
	}

	err = watcher.Watch(runBaseDir)
	if err != nil {
		logging.GetLogger().Errorf("Unable to Watch %s: %s", runBaseDir, err.Error())
		return
	}

	u.initialize()

	for {
		select {
		case ev := <-watcher.Event:
			if ev.Mask&inotify.IN_CREATE > 0 {
				u.Register(ev.Name, nil)
			}
			if ev.Mask&inotify.IN_DELETE > 0 {
				u.Unregister(ev.Name)
			}

		case err := <-watcher.Error:
			logging.GetLogger().Errorf("Error while watching network namespace: %s", err.Error())
		}
	}
}

func (u *NetNSProbe) Start() {
	go u.start()
}

func (u *NetNSProbe) Stop() {
	u.Lock()
	defer u.Unlock()

	for _, probe := range u.nsnlProbes {
		probe.Stop()
	}
}

func NewNetNSProbe(g *graph.Graph, n *graph.Node) *NetNSProbe {
	if uid := os.Geteuid(); uid != 0 {
		logging.GetLogger().Fatalf("NetNS probe has to be run as root")
	}

	return &NetNSProbe{
		Graph:       g,
		Root:        n,
		nsnlProbes:  make(map[string]*NetNsNetLinkTopoUpdater),
		pathToNetNS: make(map[string]*NetNs),
	}
}
