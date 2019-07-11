// +build linux

/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package common

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/skydive-project/skydive/logging"
	"github.com/vishvananda/netns"
)

// Namespace types
const (
	IPCNamespace     = "ipc"
	MountNamespace   = "mnt"
	NetworkNamespace = "net"
	PIDNamespace     = "pid"
	UserNamespace    = "user"
	TimeNamespace    = "uts"
)

// Namespace describes a network namespace path associated with a device / inode
type Namespace struct {
	netns.NsHandle
	path string
	dev  uint64
	ino  uint64
}

func (ns *Namespace) String() string {
	return fmt.Sprintf("%d,%d", ns.dev, ns.ino)
}

// Path returns the namespace path
func (ns *Namespace) Path() string {
	return ns.path
}

// Ino returns the namespace inode number
func (ns *Namespace) Ino() uint64 {
	return ns.ino
}

// Dev returns the namespace device number
func (ns *Namespace) Dev() uint64 {
	return ns.dev
}

// Equal compares two Namespace objects
func (ns *Namespace) Equal(o *Namespace) bool {
	return (ns.dev == o.dev && ns.ino == o.ino)
}

// GetNamespaceFromPath open a namespace fd from a path
func GetNamespaceFromPath(kind, path string) (*Namespace, error) {
	ns, err := netns.GetFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to get %s root namespace %s", kind, path)
	}

	var stats syscall.Stat_t
	if err = syscall.Fstat(int(ns), &stats); err != nil {
		return nil, fmt.Errorf("Failed to stat %s root namespace %s", kind, path)
	}

	return &Namespace{NsHandle: ns, path: path, dev: stats.Dev, ino: stats.Ino}, nil
}

// GetCurrentNamespace returns the current namespace of the specified kind
func GetCurrentNamespace(kind string) (*Namespace, error) {
	return GetNamespaceFromPath(kind, fmt.Sprintf("/proc/%d/task/%d/ns/%s", os.Getpid(), syscall.Gettid(), kind))
}

// NamespaceContext describes a NameSpace Context switch API
type NamespaceContext struct {
	nsType int
	origNs netns.NsHandle
	newNs  netns.NsHandle
}

// Quit the NameSpace and go back to the original one
func (n *NamespaceContext) Quit() error {
	if n != nil {
		if err := netns.Setns(n.origNs, n.nsType); err != nil {
			return err
		}
		n.newNs.Close()
		n.origNs.Close()
	}
	return nil
}

// Close the NameSpace
func (n *NamespaceContext) Close() {
	if n != nil && n.origNs.IsOpen() {
		n.Quit()
	}

	runtime.UnlockOSThread()
}

func getNsType(kind string) int {
	switch kind {
	case NetworkNamespace:
		return syscall.CLONE_NEWNET
	case IPCNamespace:
		return syscall.CLONE_NEWIPC
	case MountNamespace:
		return syscall.CLONE_NEWNS
	case PIDNamespace:
		return syscall.CLONE_NEWPID
	case UserNamespace:
		return syscall.CLONE_NEWUSER
	case TimeNamespace:
		return syscall.CLONE_NEWUTS
	default:
		return 0
	}
}

// NewNamespaceContext creates a new namespace context from a path
func NewNamespaceContext(kind string, path string) (*NamespaceContext, error) {
	nsType := getNsType(kind)
	if nsType == 0 {
		return nil, fmt.Errorf("Unsupported namespace type: %s", kind)
	}

	runtime.LockOSThread()

	origns, err := GetCurrentNamespace(kind)
	if err != nil {
		return nil, fmt.Errorf("Error while getting current %s ns: %s", kind, err.Error())
	}

	newns, err := GetNamespaceFromPath(kind, path)
	if err != nil {
		origns.Close()
		return nil, fmt.Errorf("Error while opening %s: %s", path, err.Error())
	}

	if err = netns.Setns(newns.NsHandle, nsType); err != nil {
		logging.GetLogger().Errorf("Failed to set namespace %d with type %d", newns.NsHandle, nsType)
		newns.Close()
		origns.Close()
		return nil, fmt.Errorf("Error while switching from root %s ns to %s: %s", kind, path, err.Error())
	}

	return &NamespaceContext{
		nsType: nsType,
		origNs: origns.NsHandle,
		newNs:  newns.NsHandle,
	}, nil
}

// NewNetNsContext creates a new network namespace context from a path
func NewNetNsContext(path string) (*NamespaceContext, error) {
	return NewNamespaceContext(NetworkNamespace, path)
}
