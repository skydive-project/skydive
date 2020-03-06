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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/avast/retry-go"
	"github.com/safchain/insanelock"
	fsnotify "gopkg.in/fsnotify/fsnotify.v1"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/service"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	tp "github.com/skydive-project/skydive/topology/probes"
	"github.com/skydive-project/skydive/topology/probes/netlink"
	"github.com/vishvananda/netns"
)

// ProbeHandler describes a netlink probe in a network namespace
type ProbeHandler struct {
	insanelock.RWMutex
	Ctx             tp.Context
	nlHandler       *netlink.ProbeHandler
	pathToNetNS     map[string]*NetNs
	nsNetLinkProbes map[string]*nsNetLinkProbe
	rootNs          *NetNs
	watcher         *fsnotify.Watcher
	pending         chan string
	exclude         []string
	state           service.State
	wg              sync.WaitGroup
}

// NetNs describes a network namespace path associated with a device / inode
type NetNs struct {
	path string
	dev  uint64
	ino  uint64
}

// extends the original struct to add use count number
type nsNetLinkProbe struct {
	*netlink.Probe
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

func (u *ProbeHandler) checkNamespace(path string) error {
	// When a new network namespace has been seen by inotify, the path to
	// the namespace may still be a regular file, not a bind mount to the
	// file in /proc/<pid>/tasks/<tid>/ns/net yet, so we wait a bit for the
	// bind mount to be set up

	return retry.Do(func() error {
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
	}, retry.Delay(10*time.Millisecond))
}

// Register a new network namespace path
func (u *ProbeHandler) Register(path string, name string) (*graph.Node, error) {
	u.Ctx.Logger.Debugf("Register network namespace: %s", path)

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
		u.Ctx.Logger.Debugf("%s is a privileged namespace", path)
		return u.Ctx.RootNode, nil
	}

	u.Lock()
	defer u.Unlock()

	_, ok := u.pathToNetNS[path]
	if !ok {
		u.pathToNetNS[path] = newns
	}

	nsString := newns.String()

	if probe, ok := u.nsNetLinkProbes[nsString]; ok {
		probe.useCount++
		u.Ctx.Logger.Debugf("Increasing counter for namespace %s to %d", nsString, probe.useCount)
		return probe.Ctx.RootNode, nil
	}

	u.Ctx.Logger.Debugf("Network namespace added: %s", nsString)
	metadata := graph.Metadata{
		"Name":   name,
		"Type":   "netns",
		"Path":   path,
		"Inode":  int64(newns.ino),
		"Device": int64(newns.dev),
	}

	u.Ctx.Graph.Lock()
	n, err := u.Ctx.Graph.NewNode(graph.GenID(), metadata)
	if err != nil {
		u.Ctx.Graph.Unlock()
		return nil, err
	}
	topology.AddOwnershipLink(u.Ctx.Graph, u.Ctx.RootNode, n, nil)
	u.Ctx.Graph.Unlock()

	u.Ctx.Logger.Debugf("Registering namespace: %s", nsString)

	var probe *netlink.Probe
	err = retry.Do(func() error {
		var err error

		ctx := tp.Context{
			Logger:   u.Ctx.Logger,
			Config:   u.Ctx.Config,
			Graph:    u.Ctx.Graph,
			RootNode: n,
		}

		probe, err = u.nlHandler.Register(path, ctx)
		if err != nil {
			return fmt.Errorf("Could not register netlink probe within namespace: %s", err)
		}
		return nil
	}, retry.Attempts(100), retry.Delay(10*time.Millisecond), retry.DelayType(retry.FixedDelay))
	if err != nil {
		return nil, err
	}

	u.nsNetLinkProbes[nsString] = &nsNetLinkProbe{Probe: probe, useCount: 1}

	return n, nil
}

// Unregister a network namespace path
func (u *ProbeHandler) Unregister(path string) {
	u.Ctx.Logger.Debugf("Unregister network Namespace: %s", path)

	u.Lock()
	defer u.Unlock()

	ns, ok := u.pathToNetNS[path]
	if !ok {
		return
	}

	nsString := ns.String()
	probe, ok := u.nsNetLinkProbes[nsString]
	if !ok {
		u.Ctx.Logger.Debugf("No existing network namespace found: %s (%s)", nsString)
		return
	}

	if probe.useCount > 1 {
		probe.useCount--
		u.Ctx.Logger.Debugf("Decremented counter for namespace %s to %d", nsString, probe.useCount)
		return
	}

	u.nlHandler.Unregister(path)
	u.Ctx.Logger.Debugf("Network namespace deleted: %s", nsString)

	u.Ctx.Graph.Lock()
	defer u.Ctx.Graph.Unlock()

	for _, child := range u.Ctx.Graph.LookupChildren(probe.Ctx.RootNode, nil, nil) {
		if err := u.Ctx.Graph.DelNode(child); err != nil {
			u.Ctx.Logger.Error(err)
		}
	}
	if err := u.Ctx.Graph.DelNode(probe.Ctx.RootNode); err != nil {
		u.Ctx.Logger.Error(err)
	}

	delete(u.nsNetLinkProbes, nsString)
	delete(u.pathToNetNS, path)
}

func (u *ProbeHandler) initializeRunPath(path string) {
	defer u.wg.Done()

	for u.state.Load() == service.RunningState {
		if _, err := os.Stat(path); err == nil {
			if err = u.watcher.Add(path); err == nil {
				break
			} else {
				u.Ctx.Logger.Errorf("Unable to watch %s: %s", path, err)
			}
		}

		time.Sleep(time.Second)
	}

	files, _ := ioutil.ReadDir(path)
	for _, f := range files {
		fullpath, name := path+"/"+f.Name(), f.Name()

		if u.isPathExcluded(fullpath) {
			continue
		}

		if _, err := u.Register(fullpath, name); err != nil {
			u.Ctx.Logger.Errorf("Failed to register namespace %s: %s", fullpath, err)
		}
	}
	u.Ctx.Logger.Debugf("ProbeHandler initialized %s", path)
}

func (u *ProbeHandler) start() {
	defer u.wg.Done()

	u.Ctx.Logger.Debugf("ProbeHandler initialized")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for u.state.Load() == service.RunningState {
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
					u.Ctx.Logger.Errorf("Failed to register namespace %s: %s", ev.Name, err)
					continue
				}
			}
			if ev.Op&fsnotify.Remove == fsnotify.Remove {
				u.Unregister(ev.Name)
			}

		case err := <-u.watcher.Errors:
			u.Ctx.Logger.Errorf("Error while watching network namespace: %s", err)
		case <-ticker.C:
		}
	}
}

// Watch add a path to the inotify watcher
func (u *ProbeHandler) Watch(path string) {
	u.pending <- path
}

// Start the probe
func (u *ProbeHandler) Start() error {
	if !u.state.CompareAndSwap(service.StoppedState, service.RunningState) {
		return probe.ErrNotStopped
	}

	u.wg.Add(1)
	go u.start()
	return nil
}

// Stop the probe
func (u *ProbeHandler) Stop() {
	if !u.state.CompareAndSwap(service.RunningState, service.StoppingState) {
		return
	}
	u.wg.Wait()

	u.nlHandler.Stop()

	u.state.Store(service.StoppedState)
}

func (u *ProbeHandler) isPathExcluded(path string) bool {
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
func (u *ProbeHandler) Exclude(paths ...string) {
	u.Lock()
	u.exclude = append(u.exclude, paths...)
	u.Unlock()
}

// NewProbe returns a new network namespace probe
func NewProbe(ctx tp.Context, bundle *probe.Bundle) (probe.Handler, error) {
	nlHandler := bundle.GetHandler("netlink")
	if nlHandler == nil {
		return nil, errors.New("unable to find the netlink handler")
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
		return nil, fmt.Errorf("Unable to create a new Watcher: %s", err)
	}

	u := &ProbeHandler{
		Ctx:             ctx,
		nlHandler:       nlHandler.(*netlink.ProbeHandler),
		pathToNetNS:     make(map[string]*NetNs),
		nsNetLinkProbes: make(map[string]*nsNetLinkProbe),
		rootNs:          rootNs,
		watcher:         watcher,
		pending:         make(chan string, 10),
		state:           service.StoppedState,
	}

	if path := ctx.Config.GetString("agent.topology.netns.run_path"); path != "" {
		u.Watch(path)
	}

	return u, nil
}
