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

package server

import (
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/ondemand"
	"github.com/skydive-project/skydive/ondemand/server"
	"github.com/skydive-project/skydive/probe"
	ws "github.com/skydive-project/skydive/websocket"
)

type activeProbe struct {
	graph   *graph.Graph
	n       *graph.Node
	capture *types.Capture
	handler probes.FlowProbeHandler
	probe   probes.Probe
}

func (p *activeProbe) OnStarted(metadata *probes.CaptureMetadata) {
	p.graph.Lock()
	metadata.ID = p.capture.UUID
	metadata.Description = p.capture.Description
	metadata.Name = p.capture.Name
	metadata.BPFFilter = p.capture.BPFFilter
	metadata.Type = p.capture.Type
	metadata.State = "active"
	if p.graph.UpdateMetadata(p.n, "Captures", func(obj interface{}) bool {
		captures := obj.(*probes.Captures)
		*captures = append(*captures, metadata)
		return true
	}) == common.ErrFieldNotFound {
		p.graph.AddMetadata(p.n, "Captures", &probes.Captures{metadata})
	}
	p.graph.Unlock()
}

func (p *activeProbe) OnStopped() {
	p.graph.Lock()
	p.graph.UpdateMetadata(p.n, "Captures", func(obj interface{}) bool {
		captures := obj.(*probes.Captures)
		for i, capture := range *captures {
			if capture.ID == p.capture.UUID {
				if len(*captures) <= 1 {
					p.graph.DelMetadata(p.n, "Captures")
					return false
				}
				*captures = append((*captures)[:i], (*captures)[i+1:]...)
				return true
			}
		}
		return false
	})
	p.graph.Unlock()
}

func (p *activeProbe) OnError(err error) {
	p.graph.Lock()
	p.graph.UpdateMetadata(p.n, "Captures", func(obj interface{}) bool {
		captures := obj.(*probes.Captures)
		for _, capture := range *captures {
			if capture.ID == p.capture.UUID {
				capture.State = "error"
				capture.Error = err.Error()
				return true
			}
		}
		return false
	})
	p.graph.Unlock()
}

type onDemandFlowProbeServer struct {
	graph         *graph.Graph
	probeHandlers *probe.Bundle
}

func (o *onDemandFlowProbeServer) getProbeHandler(n *graph.Node, resource types.Resource) (probes.FlowProbeHandler, error) {
	capture := resource.(*types.Capture)
	tp, _ := n.GetFieldString("Type")
	probeType, err := common.ProbeTypeForNode(tp, capture.Type)
	if err != nil {
		return nil, err
	}

	probe := o.probeHandlers.GetProbe(probeType)
	if probe == nil {
		return nil, fmt.Errorf("Unable to find probe for this capture type: %s", capture.Type)
	}

	probeHandler := probe.(probes.FlowProbeHandler)
	return probeHandler, nil
}

func (o *onDemandFlowProbeServer) CreateTask(n *graph.Node, resource types.Resource) (ondemand.Task, error) {
	handler, err := o.getProbeHandler(n, resource)
	if err != nil {
		return nil, err
	}

	active := &activeProbe{graph: o.graph, n: n, capture: resource.(*types.Capture), handler: handler}
	if active.probe, err = handler.RegisterProbe(n, resource.(*types.Capture), active); err != nil {
		go active.OnError(err)
		return nil, err
	}

	return active, nil
}

func (o *onDemandFlowProbeServer) RemoveTask(n *graph.Node, resource types.Resource, task ondemand.Task) error {
	activeProbe := task.(*activeProbe)
	return activeProbe.handler.UnregisterProbe(n, activeProbe, activeProbe.probe)
}

func (o *onDemandFlowProbeServer) ResourceName() string {
	return "Capture"
}

func (o *onDemandFlowProbeServer) DecodeMessage(msg json.RawMessage) (types.Resource, error) {
	var capture types.Capture
	if err := json.Unmarshal(msg, &capture); err != nil {
		return nil, fmt.Errorf("Unable to decode %s: %s", o.ResourceName(), err)
	}
	return &capture, nil
}

// NewOnDemandFlowProbeServer creates a new ondemand probes server based on graph and websocket
func NewOnDemandFlowProbeServer(fb *probe.Bundle, g *graph.Graph, pool *ws.StructClientPool) (*server.OnDemandServer, error) {
	return server.NewOnDemandServer(g, pool, &onDemandFlowProbeServer{
		graph:         g,
		probeHandlers: fb,
	})
}
