// +build linux

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

package netns

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/fsnotify.v1"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/probes/netlink"
	"github.com/vishvananda/netns"
)

// NetNSProbe describes a netlink probe in a network namespace
type NetNSProbe struct {
	sync.RWMutex
	Graph              *graph.Graph
	Root               *graph.Node
	NetLinkProbe       *netlink.NetLinkProbe
	pathToNetNS        map[string]*NetNs
	netNsNetLinkProbes map[string]*netNsNetLinkProbe
	rootNs             *NetNs
	watcher            *fsnotify.Watcher
	pending            chan string
}

// NetNs describes a network namespace path associated with a device / inode
type NetNs struct {
	path string
	dev  uint64
	ino  uint64
}

// extends the original struct to add use count number
type netNsNetLinkProbe struct {
	*netlink.NetNsNetLinkProbe
	useCount int
}

func getNetNSName(path string) string {
	s := strings.Split(path, "/")
	return s[len(s)-1]
}

func (ns *NetNs) String() string {
	return fmt.Sprintf("%d,%d", ns.dev, ns.ino)
}

// Equal compares two NetNs objects
func (ns *NetNs) Equal(o *NetNs) bool {
	return (ns.dev == o.dev && ns.ino == o.ino)
}

func (u *NetNSProbe) checkNamespace(path string) error {
	// When a new network namespace has been seen by inotify, the path to
	// the namespace may still be a regular file, not a bind mount to the
	// file in /proc/<pid>/tasks/<tid>/ns/net yet, so we wait a bit for the
	// bind mount to be set up

	return common.Retry(func() error {
		var stats, parentStats syscall.Stat_t
		fd, err := syscall.Open(path, syscall.O_RDONLY, 0)
		if err != nil {
			return err
		}

		err = syscall.Fstat(fd, &stats)
		syscall.Close(fd)
		if err != nil {
			return err
		}

		if parent := filepath.Dir(path); parent != "" {
			if err := syscall.Stat(parent, &parentStats); err == nil {
				if stats.Dev == parentStats.Dev {
					return fmt.Errorf("%s does not seem to be a valid namespace", path)
				}
			}
		}

		return nil
	}, 10, time.Millisecond*100)
}

// Register a new network namespace path
func (u *NetNSProbe) Register(path string, name string) (*graph.Node, error) {
	logging.GetLogger().Debugf("Register network namespace: %s", path)

	if err := u.checkNamespace(path); err != nil {
		return nil, err
	}

	var stats syscall.Stat_t
	if err := syscall.Stat(path, &stats); err != nil {
		return nil, fmt.Errorf("Failed to stat namespace %s: %s", path, err.Error())
	}

	newns := &NetNs{path: path, dev: stats.Dev, ino: stats.Ino}

	// avoid hard link to root ns
	if u.rootNs.Equal(newns) {
		return u.Root, nil
	}

	u.Lock()
	defer u.Unlock()

	_, ok := u.pathToNetNS[path]
	if !ok {
		u.pathToNetNS[path] = newns
	}

	nsString := newns.String()
	if probe, ok := u.netNsNetLinkProbes[nsString]; ok {
		probe.useCount++
		logging.GetLogger().Debugf("Increasing counter for namespace %s to %d", nsString, probe.useCount)
		return probe.Root, nil
	}

	u.Graph.Lock()

	logging.GetLogger().Debugf("Network namespace added: %s", nsString)
	metadata := graph.Metadata{
		"Name":   name,
		"Type":   "netns",
		"Path":   path,
		"Inode":  int64(newns.ino),
		"Device": int64(newns.dev),
	}

	n := u.Graph.NewNode(graph.GenID(), metadata)
	topology.AddOwnershipLink(u.Graph, u.Root, n, nil)

	u.Graph.Unlock()

	logging.GetLogger().Debugf("Registering namespace: %s", nsString)
	probe, err := u.NetLinkProbe.Register(path, n)
	if err != nil {
		logging.GetLogger().Errorf("Could not register netlink probe within namespace: %s", err.Error())
	}

	u.netNsNetLinkProbes[nsString] = &netNsNetLinkProbe{NetNsNetLinkProbe: probe, useCount: 1}

	return n, nil
}

// Unregister a network namespace path
func (u *NetNSProbe) Unregister(path string) {
	logging.GetLogger().Debugf("Unregister network Namespace: %s", path)

	u.Lock()
	defer u.Unlock()

	ns, ok := u.pathToNetNS[path]
	if !ok {
		return
	}

	delete(u.pathToNetNS, path)
	nsString := ns.String()
	probe, ok := u.netNsNetLinkProbes[nsString]
	if !ok {
		logging.GetLogger().Debugf("No existing network namespace found: %s (%s)", nsString)
		return
	}

	if probe.useCount > 1 {
		probe.useCount--
		logging.GetLogger().Debugf("Decremented counter for namespace %s to %d", nsString, probe.useCount)
		return
	}

	u.NetLinkProbe.Unregister(path)
	logging.GetLogger().Debugf("Network namespace deleted: %s", nsString)

	u.Graph.Lock()
	defer u.Graph.Unlock()

	for _, child := range u.Graph.LookupChildren(probe.Root, nil, nil) {
		u.Graph.DelNode(child)
	}
	u.Graph.DelNode(probe.Root)

	delete(u.netNsNetLinkProbes, nsString)
}

func (u *NetNSProbe) initializeRunPath(path string) {
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	if err := u.watcher.Add(path); err != nil {
		logging.GetLogger().Errorf("Unable to Watch %s: %s", path, err.Error())
	}

	files, _ := ioutil.ReadDir(path)
	for _, f := range files {
		if _, err := u.Register(path+"/"+f.Name(), f.Name()); err != nil {
			logging.GetLogger().Errorf("Failed to register namespace %s: %s", path+"/"+f.Name(), err.Error())
			continue
		}
	}
	logging.GetLogger().Debugf("NetNSProbe initialized %s", path)
}

func (u *NetNSProbe) start() {
	logging.GetLogger().Debugf("NetNSProbe initialized")
	for {
		select {
		case path := <-u.pending:
			go u.initializeRunPath(path)
		case ev := <-u.watcher.Events:
			if ev.Op&fsnotify.Create == fsnotify.Create {
				if _, err := u.Register(ev.Name, getNetNSName(ev.Name)); err != nil {
					logging.GetLogger().Errorf("Failed to register namespace %s: %s", ev.Name, err.Error())
					continue
				}
			}
			if ev.Op&fsnotify.Remove == fsnotify.Remove {
				u.Unregister(ev.Name)
			}

		case err := <-u.watcher.Errors:
			logging.GetLogger().Errorf("Error while watching network namespace: %s", err.Error())
		}
	}
}

// Watch add a path to the inotify watcher
func (u *NetNSProbe) Watch(path string) {
	u.pending <- path
}

// Start the probe
func (u *NetNSProbe) Start() {
	go u.start()
}

// Stop the probe
func (u *NetNSProbe) Stop() {
	u.NetLinkProbe.Stop()
}

// NewNetNSProbe creates a new network namespace probe
func NewNetNSProbe(g *graph.Graph, n *graph.Node, nlProbe *netlink.NetLinkProbe) (*NetNSProbe, error) {
	if uid := os.Geteuid(); uid != 0 {
		return nil, errors.New("NetNS probe has to be run as root")
	}

	ns, err := netns.Get()
	if err != nil {
		return nil, errors.New("Failed to get root namespace")
	}
	defer ns.Close()

	var stats syscall.Stat_t
	if err = syscall.Fstat(int(ns), &stats); err != nil {
		return nil, errors.New("Failed to stat root namespace")
	}
	rootNs := &NetNs{dev: stats.Dev, ino: stats.Ino}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("Unable to create a new Watcher: %s", err.Error())
	}

	nsProbe := &NetNSProbe{
		Graph:              g,
		Root:               n,
		NetLinkProbe:       nlProbe,
		pathToNetNS:        make(map[string]*NetNs),
		netNsNetLinkProbes: make(map[string]*netNsNetLinkProbe),
		rootNs:             rootNs,
		watcher:            watcher,
		pending:            make(chan string, 10),
	}

	if path := config.GetString("netns.run_path"); path != "" {
		nsProbe.Watch(path)
	}

	return nsProbe, nil
}
