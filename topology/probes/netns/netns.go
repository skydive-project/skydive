// +build linux

/*
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package netns

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	fsnotify "gopkg.in/fsnotify/fsnotify.v1"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes/netlink"
	"github.com/vishvananda/netns"
)

// Probe describes a netlink probe in a network namespace
type Probe struct {
	common.RWMutex
	Graph       *graph.Graph
	Root        *graph.Node
	nlProbe     *netlink.Probe
	pathToNetNS map[string]*NetNs
	netNsProbes map[string]*netNsProbe
	rootNs      *NetNs
	watcher     *fsnotify.Watcher
	pending     chan string
	exclude     []string
	state       int64
	wg          sync.WaitGroup
}

// NetNs describes a network namespace path associated with a device / inode
type NetNs struct {
	path string
	dev  uint64
	ino  uint64
}

// extends the original struct to add use count number
type netNsProbe struct {
	*netlink.NetNsProbe
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

func (u *Probe) checkNamespace(path string) error {
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

		if (stats.Mode & syscall.S_IFLNK) > 0 {
			// check if the link is a regular link or could be netns
			b := make([]byte, 256)
			n, err := syscall.Readlink(path, b)
			if err == nil {
				var lstat syscall.Stat_t
				if err := syscall.Lstat(string(b[:n]), &lstat); err != nil && lstat.Dev == 0 {
					return nil
				}
			}
		}

		if parent := filepath.Dir(path); parent != "" {
			if err := syscall.Stat(parent, &parentStats); err == nil {
				if stats.Dev == parentStats.Dev {
					return fmt.Errorf("%s does not seem to be a valid namespace", path)
				}
			}
		}

		return nil
	}, 30, time.Millisecond*100)
}

// Register a new network namespace path
func (u *Probe) Register(path string, name string) (*graph.Node, error) {
	logging.GetLogger().Debugf("Register network namespace: %s", path)

	if err := u.checkNamespace(path); err != nil {
		return nil, err
	}

	var stats syscall.Stat_t
	if err := syscall.Stat(path, &stats); err != nil {
		return nil, fmt.Errorf("Failed to stat namespace %s: %s", path, err)
	}

	newns := &NetNs{path: path, dev: stats.Dev, ino: stats.Ino}

	// avoid hard link to root ns
	if u.rootNs.Equal(newns) {
		logging.GetLogger().Debugf("%s is a privileged namespace", path)
		return u.Root, nil
	}

	u.Lock()
	defer u.Unlock()

	_, ok := u.pathToNetNS[path]
	if !ok {
		u.pathToNetNS[path] = newns
	}

	nsString := newns.String()

	if probe, ok := u.netNsProbes[nsString]; ok {
		probe.useCount++
		logging.GetLogger().Debugf("Increasing counter for namespace %s to %d", nsString, probe.useCount)
		return probe.Root, nil
	}

	logging.GetLogger().Debugf("Network namespace added: %s", nsString)
	metadata := graph.Metadata{
		"Name":   name,
		"Type":   "netns",
		"Path":   path,
		"Inode":  int64(newns.ino),
		"Device": int64(newns.dev),
	}

	u.Graph.Lock()
	n, err := u.Graph.NewNode(graph.GenID(), metadata)
	if err != nil {
		u.Graph.Unlock()
		return nil, err
	}
	topology.AddOwnershipLink(u.Graph, u.Root, n, nil)
	u.Graph.Unlock()

	logging.GetLogger().Debugf("Registering namespace: %s", nsString)

	var probe *netlink.NetNsProbe
	err = common.Retry(func() error {
		var err error
		probe, err = u.nlProbe.Register(path, n)
		if err != nil {
			return fmt.Errorf("Could not register netlink probe within namespace: %s", err)
		}
		return nil
	}, 100, 10*time.Millisecond)
	if err != nil {
		return nil, err
	}

	u.netNsProbes[nsString] = &netNsProbe{NetNsProbe: probe, useCount: 1}

	return n, nil
}

// Unregister a network namespace path
func (u *Probe) Unregister(path string) {
	logging.GetLogger().Debugf("Unregister network Namespace: %s", path)

	u.Lock()
	defer u.Unlock()

	ns, ok := u.pathToNetNS[path]
	if !ok {
		return
	}

	delete(u.pathToNetNS, path)
	nsString := ns.String()
	probe, ok := u.netNsProbes[nsString]
	if !ok {
		logging.GetLogger().Debugf("No existing network namespace found: %s (%s)", nsString)
		return
	}

	if probe.useCount > 1 {
		probe.useCount--
		logging.GetLogger().Debugf("Decremented counter for namespace %s to %d", nsString, probe.useCount)
		return
	}

	u.nlProbe.Unregister(path)
	logging.GetLogger().Debugf("Network namespace deleted: %s", nsString)

	u.Graph.Lock()
	defer u.Graph.Unlock()

	for _, child := range u.Graph.LookupChildren(probe.Root, nil, nil) {
		if err := u.Graph.DelNode(child); err != nil {
			logging.GetLogger().Error(err)
		}
	}
	if err := u.Graph.DelNode(probe.Root); err != nil {
		logging.GetLogger().Error(err)
	}

	delete(u.netNsProbes, nsString)
}

func (u *Probe) initializeRunPath(path string) {
	defer u.wg.Done()

	err := common.Retry(func() error {
		if atomic.LoadInt64(&u.state) != common.RunningState {
			return nil
		}

		if _, err := os.Stat(path); err != nil {
			return err
		}

		if err := u.watcher.Add(path); err != nil {
			return fmt.Errorf("Unable to Watch %s: %s", path, err)
		}

		return nil
	}, math.MaxInt32, time.Second)

	if err != nil {
		logging.GetLogger().Error(err)
		return
	}

	files, _ := ioutil.ReadDir(path)
	for _, f := range files {
		fullpath, name := path+"/"+f.Name(), f.Name()

		if u.isPathExcluded(fullpath) {
			continue
		}

		if _, err := u.Register(fullpath, name); err != nil {
			logging.GetLogger().Errorf("Failed to register namespace %s: %s", fullpath, err)
		}
	}
	logging.GetLogger().Debugf("Probe initialized %s", path)
}

func (u *Probe) start() {
	defer u.wg.Done()

	logging.GetLogger().Debugf("Probe initialized")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	if !atomic.CompareAndSwapInt64(&u.state, common.StoppedState, common.RunningState) {
		return
	}

	for atomic.LoadInt64(&u.state) == common.RunningState {
		select {
		case path := <-u.pending:
			u.wg.Add(1)
			go u.initializeRunPath(path)
		case ev := <-u.watcher.Events:
			if u.isPathExcluded(ev.Name) {
				continue
			}
			if ev.Op&fsnotify.Create == fsnotify.Create {
				if _, err := u.Register(ev.Name, getNetNSName(ev.Name)); err != nil {
					logging.GetLogger().Errorf("Failed to register namespace %s: %s", ev.Name, err)
					continue
				}
			}
			if ev.Op&fsnotify.Remove == fsnotify.Remove {
				u.Unregister(ev.Name)
			}

		case err := <-u.watcher.Errors:
			logging.GetLogger().Errorf("Error while watching network namespace: %s", err)
		case <-ticker.C:
		}
	}
}

// Watch add a path to the inotify watcher
func (u *Probe) Watch(path string) {
	u.pending <- path
}

// Start the probe
func (u *Probe) Start() {
	u.wg.Add(1)
	go u.start()
}

// Stop the probe
func (u *Probe) Stop() {
	if !atomic.CompareAndSwapInt64(&u.state, common.RunningState, common.StoppingState) {
		return
	}
	u.wg.Wait()

	u.nlProbe.Stop()

	atomic.StoreInt64(&u.state, common.StoppedState)
}

func (u *Probe) isPathExcluded(path string) bool {
	u.RLock()
	defer u.RUnlock()

	for _, e := range u.exclude {
		if e == path {
			return true
		}
	}
	return false
}

// Exclude specify path to not process
func (u *Probe) Exclude(paths ...string) {
	u.Lock()
	u.exclude = append(u.exclude, paths...)
	u.Unlock()
}

// NewProbe creates a new network namespace probe
func NewProbe(g *graph.Graph, n *graph.Node, nlProbe *netlink.Probe) (*Probe, error) {
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
		return nil, fmt.Errorf("Unable to create a new Watcher: %s", err)
	}

	nsProbe := &Probe{
		Graph:       g,
		Root:        n,
		nlProbe:     nlProbe,
		pathToNetNS: make(map[string]*NetNs),
		netNsProbes: make(map[string]*netNsProbe),
		rootNs:      rootNs,
		watcher:     watcher,
		pending:     make(chan string, 10),
		state:       common.StoppedState,
	}

	if path := config.GetString("netns.run_path"); path != "" {
		nsProbe.Watch(path)
	}

	return nsProbe, nil
}
