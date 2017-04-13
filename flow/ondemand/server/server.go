/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/skydive-project/skydive/api"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/ondemand"
	"github.com/skydive-project/skydive/flow/probes"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

type OnDemandProbeServer struct {
	sync.RWMutex
	graph.DefaultGraphListener
	shttp.DefaultWSClientEventHandler
	Graph             *graph.Graph
	Probes            *probes.FlowProbeBundle
	WSAsyncClientPool *shttp.WSAsyncClientPool
	fta               *flow.TableAllocator
	activeProbes      map[graph.Identifier]*flow.Table
	captures          map[graph.Identifier]*api.Capture
}

func (o *OnDemandProbeServer) isActive(n *graph.Node) bool {
	o.RLock()
	defer o.RUnlock()
	_, active := o.activeProbes[n.ID]

	return active
}

func (o *OnDemandProbeServer) getProbe(n *graph.Node, capture *api.Capture) (*probes.FlowProbe, error) {
	tp, _ := n.GetFieldString("Type")

	capType := ""
	if capture.Type != "" {
		types := common.CaptureTypes[tp].Allowed
		for _, t := range types {
			if t == capture.Type {
				capType = t
				break
			}
		}
		if capType == "" {
			return nil, fmt.Errorf("Capture type %v not allowed on this node: %v", capture, n)
		}
	} else {
		// no capture type defined for this type of node, ex: ovsport
		c, ok := common.CaptureTypes[tp]
		if !ok {
			return nil, nil
		}
		capType = c.Default
	}
	probe := o.Probes.GetProbe(capType)
	if probe == nil {
		return nil, fmt.Errorf("Unable to find probe for this capture type: %v", capType)
	}

	fprobe := probe.(*probes.FlowProbe)
	return fprobe, nil
}

func (o *OnDemandProbeServer) registerProbe(n *graph.Node, capture *api.Capture) bool {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		logging.GetLogger().Debugf("Unable to register flow probe, name of node unknown %s", n.ID)
		return false
	}

	logging.GetLogger().Debugf("Attempting to register probe on node %s", name)

	if o.isActive(n) {
		logging.GetLogger().Debugf("A probe already exists for %s", n.ID)
		return false
	}

	if _, err := n.GetFieldString("Type"); err != nil {
		logging.GetLogger().Infof("Unable to register flow probe type of node unknown %v", n)
		return false
	}

	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		logging.GetLogger().Infof("Unable to register flow probe without node TID %v", n)
		return false
	}

	o.Lock()
	defer o.Unlock()

	fprobe, err := o.getProbe(n, capture)
	if fprobe == nil {
		if err != nil {
			logging.GetLogger().Error(err.Error())
		}
		return false
	}

	ft := o.fta.Alloc(fprobe.AsyncFlowPipeline)
	ft.SetNodeTID(tid)

	if err := fprobe.RegisterProbe(n, capture, ft); err != nil {
		logging.GetLogger().Debugf("Failed to register flow probe: %s", err.Error())
		o.fta.Release(ft)
		return false
	}

	o.activeProbes[n.ID] = ft
	o.captures[n.ID] = capture

	logging.GetLogger().Debugf("New active probe on: %v(%v)", n, capture)
	return true
}

func (o *OnDemandProbeServer) unregisterProbe(n *graph.Node) bool {
	if !o.isActive(n) {
		return false
	}

	o.Lock()
	c := o.captures[n.ID]
	o.Unlock()
	fprobe, err := o.getProbe(n, c)
	if fprobe == nil {
		if err != nil {
			logging.GetLogger().Error(err.Error())
		}
		return false
	}

	if err := fprobe.UnregisterProbe(n); err != nil {
		logging.GetLogger().Debugf("Failed to unregister flow probe: %s", err.Error())
	}

	o.Lock()
	o.fta.Release(o.activeProbes[n.ID])
	delete(o.activeProbes, n.ID)
	delete(o.captures, n.ID)
	o.Unlock()

	return true
}

func (o *OnDemandProbeServer) OnMessage(c *shttp.WSAsyncClient, msg shttp.WSMessage) {
	var query ondemand.CaptureQuery
	if err := json.Unmarshal([]byte(*msg.Obj), &query); err != nil {
		logging.GetLogger().Errorf("Unable to decode capture %v", msg)
		return
	}

	status := http.StatusBadRequest

	o.Graph.Lock()

	switch msg.Type {
	case "CaptureStart":
		n := o.Graph.GetNode(graph.Identifier(query.NodeID))
		if n == nil {
			logging.GetLogger().Errorf("Unknown node %s for new capture", query.NodeID)
			status = http.StatusNotFound
			break
		}

		status = http.StatusOK
		if _, err := n.GetFieldString("Capture/ID"); err == nil {
			logging.GetLogger().Debugf("Capture already started on node %s", n.ID)
		} else {
			if ok := o.registerProbe(n, &query.Capture); ok {
				t := o.Graph.StartMetadataTransaction(n)
				t.AddMetadata("Capture/ID", query.Capture.UUID)
				t.Commit()
			} else {
				status = http.StatusInternalServerError
			}
		}

	case "CaptureStop":
		n := o.Graph.GetNode(graph.Identifier(query.NodeID))
		if n == nil {
			logging.GetLogger().Errorf("Unknown node %s for new capture", query.NodeID)
			status = http.StatusNotFound
			break
		}

		status = http.StatusOK
		if ok := o.unregisterProbe(n); ok {
			metadata := n.Metadata()
			delete(metadata, "Capture/ID")
			delete(metadata, "Capture/PacketsReceived")
			delete(metadata, "Capture/PacketsDropped")
			delete(metadata, "Capture/PacketsIfDropped")
			o.Graph.SetMetadata(n, metadata)
		} else {
			status = http.StatusInternalServerError
		}
	}

	// be sure to unlock before sending message
	o.Graph.Unlock()

	reply := msg.Reply(&query, msg.Type+"Reply", status)
	c.SendWSMessage(reply)
}

func (o *OnDemandProbeServer) OnNodeDeleted(n *graph.Node) {
	if _, err := n.GetFieldString("Capture/ID"); err != nil {
		return
	}

	o.unregisterProbe(n)
}

func (o *OnDemandProbeServer) Start() error {
	o.Graph.AddEventListener(o)
	o.WSAsyncClientPool.AddEventHandler(o, []string{ondemand.Namespace})

	return nil
}

func (o *OnDemandProbeServer) Stop() {
	o.Graph.RemoveEventListener(o)
}

func NewOnDemandProbeServer(fb *probes.FlowProbeBundle, g *graph.Graph, wspool *shttp.WSAsyncClientPool) (*OnDemandProbeServer, error) {
	return &OnDemandProbeServer{
		Graph:             g,
		Probes:            fb,
		WSAsyncClientPool: wspool,
		fta:               fb.FlowTableAllocator,
		activeProbes:      make(map[graph.Identifier]*flow.Table),
		captures:          make(map[graph.Identifier]*api.Capture),
	}, nil
}
