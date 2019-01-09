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
	"strings"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/skydive-project/skydive/common/fs"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
)

// Probe is a SRIO-V probe designed to handle all vfs on server
type Probe struct {
	graph   *graph.Graph
	root    *graph.Node
	cancel  context.CancelFunc
	watcher *fs.SysProcFsWatcher
}

func (h *Probe) periodicChecker(ctx context.Context) {
	allZero := func(s []byte) bool {
		for _, v := range s {
			if v != 0 {
				return false
			}
		}
		return true
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			linkList, err := netlink.LinkList()
			if err != nil {
				logging.GetLogger().Errorf("Unable to get link list : %s", err)
				continue
			}

			h.graph.Lock()

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

					mac := vf.Mac.String()
					vfNode := h.graph.LookupFirstChild(h.root, graph.Metadata{"MAC": mac})
					if vfNode == nil {
						logging.GetLogger().Debugf("Failed to find the device for VF with MAC '%s'", mac)
						continue
					}

					if !h.graph.AreLinked(parentNode, vfNode, graph.Metadata{"RelationType": "SRIO-V"}) {
						h.graph.Link(parentNode, vfNode, graph.Metadata{"RelationType": "SRIO-V"})
					}
				}

				if len(attrs.Vfs) > 0 {
					h.watcher.AddPath(fmt.Sprintf("/sys/class/net/%s/operstate", attrs.Name))
				}
			}

			h.graph.Unlock()
		}
	}
}

// OnChangedFile being triggered when file operstate changed in /sys/ linux file structure
func (h *Probe) OnChangedFile(details fs.Event) {
	re := regexp.MustCompile("/sys/class/net/(.+)/operstate")
	match := re.FindStringSubmatch(details.Name)
	interfaceName := string(match[1])
	state := strings.TrimSpace(strings.ToUpper(string(details.FileInfo.Content)))

	h.graph.Lock()
	node := h.graph.LookupFirstChild(h.root, graph.Metadata{"Name": interfaceName})
	h.graph.AddMetadata(node, "State", state)
	h.graph.Unlock()

	logging.GetLogger().Infof("Updated state for interface: %s", interfaceName)
}

// Start the probe
func (h *Probe) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.watcher.Start()
	go func() {
		logging.GetLogger().Debugf("Start checking SRIO-V nodes")
		h.periodicChecker(ctx)
	}()
}

// Stop the probe
func (h *Probe) Stop() {
	h.cancel()
	h.watcher.Stop()
}

// NewProbe creates an SRIO-V probe
func NewProbe(graph *graph.Graph, root *graph.Node) (*Probe, error) {
	handler := &Probe{
		graph:   graph,
		root:    root,
		watcher: fs.NewSysProcFsWatcher(),
	}
	handler.watcher.OnEvent(fs.ChangedContent, handler.OnChangedFile)
	return handler, nil
}
