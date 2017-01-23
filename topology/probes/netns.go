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
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/fsnotify.v1"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/vishvananda/netns"
)

type NetNSProbe struct {
	sync.RWMutex

	Graph       *graph.Graph
	Root        *graph.Node
	nsnlProbes  map[string]*NetNsNetLinkTopoUpdater
	pathToNetNS map[string]*NetNs
	runPath     string
	rootNsDev   uint64
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

func (nu *NetNsNetLinkTopoUpdater) Run(ns *NetNs) {
	logging.GetLogger().Debugf("Starting NetLinkTopoUpdater for NetNS: %s", ns.path)

	/* start a netlinks updater inside this namespace */
	nu.Lock()
	nu.nlProbe = NewNetLinkProbe(nu.Graph, nu.Root)
	nu.Unlock()

	/* NOTE(safchain) don't Start just Run, need to keep it alive for the time life of the netns
	 * and there is no need to have a new goroutine here
	 */
	nu.nlProbe.Run(ns.path)

	nu.Lock()
	nu.nlProbe = nil
	nu.Unlock()

	logging.GetLogger().Debugf("NetLinkTopoUpdater stopped for NetNS: %s", ns.path)
}

func (nu *NetNsNetLinkTopoUpdater) Start(ns *NetNs) {
	go nu.Run(ns)
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
	logging.GetLogger().Debugf("Register Network Namespace: %s", path)

	// When a new network namespace has been seen by inotify, the path to
	// the namespace may still be a regular file, not a bind mount to the
	// file in /proc/<pid>/tasks/<tid>/ns/net yet, so we wait a bit for the
	// bind mount to be set up
	var newns *NetNs
	err := common.Retry(func() error {
		var stats syscall.Stat_t
		fd, err := syscall.Open(path, syscall.O_RDONLY, 0)
		if err != nil {
			return err
		}

		err = syscall.Fstat(fd, &stats)
		syscall.Close(fd)
		if err != nil {
			return err
		}

		if stats.Dev != u.rootNsDev {
			return fmt.Errorf("%s does not seem to be a valid namespace", path)
		}

		newns = &NetNs{path: path, dev: stats.Dev, ino: stats.Ino}
		return nil
	}, 10, time.Millisecond*20)

	if err != nil {
		logging.GetLogger().Errorf("Could not register namespace: %s", err.Error())
		return nil
	}

	_, ok := u.pathToNetNS[path]
	if !ok {
		u.pathToNetNS[path] = newns
	}

	u.Lock()
	defer u.Unlock()

	nsString := newns.String()
	probe, ok := u.nsnlProbes[nsString]
	if ok {
		probe.useCount++
		logging.GetLogger().Debugf("Increasing counter for namespace %s to %d", nsString, probe.useCount)
		return probe.Root
	}

	u.Graph.Lock()
	defer u.Graph.Unlock()

	logging.GetLogger().Debugf("Network Namespace added: %s", nsString)
	metadata := graph.Metadata{"Name": getNetNSName(path), "Type": "netns", "Path": path}
	if extraMetadata != nil {
		for k, v := range extraMetadata {
			metadata[k] = v
		}
	}
	n := u.Graph.NewNode(graph.GenID(), metadata)
	u.Graph.Link(u.Root, n, graph.Metadata{"RelationType": "ownership"})

	nu := NewNetNsNetLinkTopoUpdater(u.Graph, n)
	nu.Start(newns)

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
	files, _ := ioutil.ReadDir(u.runPath)
	for _, f := range files {
		u.Register(u.runPath+"/"+f.Name(), nil)
	}
}

func (u *NetNSProbe) start() {
	// wait for the path creation
	for {
		_, err := os.Stat(u.runPath)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logging.GetLogger().Errorf("Unable to create a new Watcher: %s", err.Error())
		return
	}

	err = watcher.Add(u.runPath)
	if err != nil {
		logging.GetLogger().Errorf("Unable to Watch %s: %s", u.runPath, err.Error())
		return
	}

	u.initialize()
	logging.GetLogger().Debugf("NetNSProbe initialized")

	for {
		select {
		case ev := <-watcher.Events:
			if ev.Op&fsnotify.Create == fsnotify.Create {
				u.Register(ev.Name, nil)
			}
			if ev.Op&fsnotify.Remove == fsnotify.Remove {
				u.Unregister(ev.Name)
			}

		case err := <-watcher.Errors:
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

func NewNetNSProbe(g *graph.Graph, n *graph.Node, runPath ...string) (*NetNSProbe, error) {
	if uid := os.Geteuid(); uid != 0 {
		return nil, errors.New("NetNS probe has to be run as root")
	}

	path := "/var/run/netns"
	if len(runPath) > 0 && runPath[0] != "" {
		path = runPath[0]
	}

	rootNs, err := netns.Get()
	if err != nil {
		return nil, errors.New("Failed to get root namespace")
	}
	defer rootNs.Close()

	var stats syscall.Stat_t
	if err := syscall.Fstat(int(rootNs), &stats); err != nil {
		return nil, errors.New("Failed to stat root namespace")
	}

	return &NetNSProbe{
		Graph:       g,
		Root:        n,
		nsnlProbes:  make(map[string]*NetNsNetLinkTopoUpdater),
		pathToNetNS: make(map[string]*NetNs),
		runPath:     path,
		rootNsDev:   stats.Dev,
	}, nil
}

func NewNetNSProbeFromConfig(g *graph.Graph, n *graph.Node) (*NetNSProbe, error) {
	path := config.GetConfig().GetString("netns.run_path")
	return NewNetNSProbe(g, n, path)
}
