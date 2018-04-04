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

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow/ondemand"
	"github.com/skydive-project/skydive/flow/probes"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

type activeProbe struct {
	graph   *graph.Graph
	node    *graph.Node
	fprobe  probes.FlowProbe
	capture *types.Capture
}

// OnDemandProbeServer describes an ondemand probe server based on websocket
type OnDemandProbeServer struct {
	sync.RWMutex
	graph.DefaultGraphListener
	shttp.DefaultWSSpeakerEventHandler
	Graph            *graph.Graph
	Probes           *probe.ProbeBundle
	WSJSONClientPool *shttp.WSJSONClientPool
	activeProbes     map[graph.Identifier]*activeProbe
}

func (o *OnDemandProbeServer) getProbe(n *graph.Node, capture *types.Capture) (probes.FlowProbe, error) {
	tp, _ := n.GetFieldString("Type")

	probeType, err := common.ProbeTypeForNode(tp, capture.Type)
	if err != nil {
		return nil, err
	}

	probe := o.Probes.GetProbe(probeType)
	if probe == nil {
		return nil, fmt.Errorf("Unable to find probe for this capture type: %s", capture.Type)
	}

	fprobe := probe.(probes.FlowProbe)
	return fprobe, nil
}

func (o *OnDemandProbeServer) registerProbe(n *graph.Node, capture *types.Capture) bool {
	name, _ := n.GetFieldString("Name")
	if name == "" {
		logging.GetLogger().Debugf("Unable to register flow probe, name of node unknown %s", n.ID)
		return false
	}

	logging.GetLogger().Debugf("Attempting to register probe on node %s", name)

	if _, err := n.GetFieldString("Type"); err != nil {
		logging.GetLogger().Infof("Unable to register flow probe type of node unknown %v", n)
		return false
	}

	tid, _ := n.GetFieldString("TID")
	if tid == "" {
		logging.GetLogger().Infof("Unable to register flow probe without node TID %v", n)
		return false
	}

	fprobe, err := o.getProbe(n, capture)
	if fprobe == nil {
		if err != nil {
			logging.GetLogger().Error(err.Error())
		}
		return false
	}
	o.Lock()
	defer o.Unlock()

	if _, active := o.activeProbes[n.ID]; active {
		logging.GetLogger().Debugf("A probe already exists for %s", n.ID)
		return false
	}

	activeProbe := &activeProbe{
		graph:   o.Graph,
		node:    n,
		fprobe:  fprobe,
		capture: capture,
	}

	if err := fprobe.RegisterProbe(n, capture, activeProbe); err != nil {
		logging.GetLogger().Debugf("Failed to register flow probe: %s", err.Error())
		return false
	}

	o.activeProbes[n.ID] = activeProbe

	logging.GetLogger().Debugf("New active probe on: %v(%v)", n, capture)
	return true
}

// unregisterProbe should be executed under graph lock
func (o *OnDemandProbeServer) unregisterProbe(n *graph.Node) bool {
	o.RLock()
	probe, active := o.activeProbes[n.ID]
	o.RUnlock()

	if !active {
		return false
	}

	if err := probe.fprobe.UnregisterProbe(n, probe); err != nil {
		logging.GetLogger().Debugf("Failed to unregister flow probe: %s", err.Error())
	}

	o.Lock()
	delete(o.activeProbes, n.ID)
	o.Unlock()

	return true
}

// OnStarted FlowProbeEventHandler implementation
func (p *activeProbe) OnStarted() {
	p.graph.Lock()
	p.graph.AddMetadata(p.node, "Capture.State", "active")
	p.graph.Unlock()
}

// OnStopped FlowProbeEventHandler implementation
func (p *activeProbe) OnStopped() {
	p.graph.Lock()
	p.graph.DelMetadata(p.node, "Capture")
	p.graph.Unlock()
}

// OnWSJSONMessage websocket message, valid message type are CaptureStart, CaptureStop
func (o *OnDemandProbeServer) OnWSJSONMessage(c shttp.WSSpeaker, msg *shttp.WSJSONMessage) {
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
		if _, err := n.GetFieldString("Capture.ID"); err == nil {
			logging.GetLogger().Debugf("Capture already started on node %s", n.ID)
		} else {
			if ok := o.registerProbe(n, &query.Capture); ok {
				tr := o.Graph.StartMetadataTransaction(n)
				tr.AddMetadata("Capture.ID", query.Capture.UUID)
				if query.Capture.Name != "" {
					tr.AddMetadata("Capture.Name", query.Capture.Name)
				}
				tr.Commit()
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
		if ok := o.unregisterProbe(n); !ok {
			status = http.StatusInternalServerError
		}
	}

	// be sure to unlock before sending message
	o.Graph.Unlock()

	reply := msg.Reply(&query, msg.Type+"Reply", status)
	c.SendMessage(reply)
}

// OnNodeDeleted graph event
func (o *OnDemandProbeServer) OnNodeDeleted(n *graph.Node) {
	if _, err := n.GetFieldString("Capture.ID"); err != nil {
		return
	}

	o.unregisterProbe(n)
}

// Start the probe
func (o *OnDemandProbeServer) Start() error {
	o.Graph.AddEventListener(o)
	o.WSJSONClientPool.AddJSONMessageHandler(o, []string{ondemand.Namespace})

	return nil
}

// Stop the probe
func (o *OnDemandProbeServer) Stop() {
	o.Graph.RemoveEventListener(o)

	o.Graph.Lock()
	defer o.Graph.Unlock()

	for _, p := range o.activeProbes {
		o.unregisterProbe(p.node)
	}
}

// NewOnDemandProbeServer creates a new Ondemand probes server based on graph and websocket
func NewOnDemandProbeServer(fb *probe.ProbeBundle, g *graph.Graph, pool *shttp.WSJSONClientPool) (*OnDemandProbeServer, error) {
	return &OnDemandProbeServer{
		Graph:            g,
		Probes:           fb,
		WSJSONClientPool: pool,
		activeProbes:     make(map[graph.Identifier]*activeProbe),
	}, nil
}
