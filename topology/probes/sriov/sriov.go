// +build linux

/*
 * Copyright (C) 2018 Samsung, Inc.
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

package sriov

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/skydive-project/skydive/common/fs"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

// Probe is a Sriov probe designed to handle all vfs on server
type Probe struct {
	graph   *graph.Graph
	root    *graph.Node
	cancel  context.CancelFunc // cancel function
	watcher *fs.SysProcFsWatcher
}

// Manager is a const to be used as a string to describe who handles sriov nodes
const Manager = "vfio"

// GenerateNameForNodeFromPhysInterfaceNameAndVFID - generates a name for vf based on physical device name and id of of this vf
func GenerateNameForNodeFromPhysInterfaceNameAndVFID(physName string, id string) string {
	return fmt.Sprintf("%s-vf-%s", physName, id)
}

func allZero(s []byte) bool {
	for _, v := range s {
		if v != 0 {
			return false
		}
	}
	return true
}

func (h *Probe) getAllSriovNodes() []*graph.Node {
	filter := filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", Manager),
	)
	m := graph.NewElementFilter(filter)
	return h.graph.GetNodes(m)
}

func (h *Probe) periodicChecker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			linkList, err := netlink.LinkList()
			if err != nil {
				logging.GetLogger().Errorf("unable to get link list : %s", err)
			}
			h.graph.Lock()
			var addedNodes = make(map[string]bool)
			for _, link := range linkList {
				attrs := link.Attrs()
				parentNode := h.graph.LookupFirstChild(h.root, graph.Metadata{"Name": attrs.Name, "IfIndex": int64(attrs.Index)})
				if parentNode == nil {
					continue
				}
				for _, vf := range attrs.Vfs {
					if allZero(vf.Mac) {
						continue
					}
					name := GenerateNameForNodeFromPhysInterfaceNameAndVFID(attrs.Name, strconv.Itoa(vf.ID))
					logging.GetLogger().Debugf("try to process sriov nodes and vf : %s. MAC: %s, %b", attrs.Name, vf.Mac, vf.Mac)
					h.addVfNode(name, parentNode, &vf)
					addedNodes[name] = true
				}
				if len(attrs.Vfs) > 0 {
					h.watcher.AddPath(fmt.Sprintf("/sys/class/net/%s/operstate", attrs.Name))
				}
			}
			for _, node := range h.getAllSriovNodes() {
				nodeName, _ := node.GetFieldString("Name")
				if _, found := addedNodes[nodeName]; found {
					continue
				}
				h.graph.DelNode(node)
			}
			h.graph.Unlock()
		}
	}
}

func (h *Probe) addVfNode(name string, parentNode *graph.Node, vf *netlink.VfInfo) {
	node := h.graph.LookupFirstChild(parentNode, graph.Metadata{"Name": name})
	if node != nil {
		return
	}
	metadata := graph.Metadata{
		"Type":    Manager,
		"Name":    name,
		"Manager": Manager,
	}

	node = h.graph.NewNode(graph.GenID(), metadata)
	topology.AddOwnershipLink(h.graph, parentNode, node, nil)
	logging.GetLogger().Debugf("Finalized registration of VF for physical interface: VF name: %s", name)
}

// OnChangedFile being triggered when file operstate changed in /sys/ linux file structure
func (h *Probe) OnChangedFile(details fs.Event) {
	re := regexp.MustCompile("/sys/class/net/(.+)/operstate")
	match := re.FindStringSubmatch(details.Name)
	interfaceName := string(match[1])
	state := strings.TrimSpace(strings.ToUpper(string(details.FileInfo.Content)))
	h.graph.Lock()
	defer h.graph.Unlock()
	node := h.graph.LookupFirstChild(h.root, graph.Metadata{"Name": interfaceName})
	h.graph.AddMetadata(node, "State", state)
	logging.GetLogger().Infof("Updated state for interface: %s", interfaceName)
}

// Start - starts the sriov probe
func (h *Probe) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.watcher.Start()
	go func() {
		logging.GetLogger().Debugf("Start checking sriov nodes")
		h.periodicChecker(ctx)
	}()
}

// Stop stops the probe
func (h *Probe) Stop() {
	h.cancel()
	h.watcher.Stop()
}

// NewProbe creates an Sriov probe
func NewProbe(graph *graph.Graph, root *graph.Node) (*Probe, error) {
	handler := &Probe{
		graph:   graph,
		root:    root,
		watcher: fs.NewSysProcFsWatcher(),
	}
	handler.watcher.OnEvent(fs.ChangedContent, handler.OnChangedFile)
	return handler, nil
}
