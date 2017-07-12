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

package api

import (
	"fmt"

	"github.com/nu7hatch/gouuid"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

// Capture describes a capture API
type Capture struct {
	UUID         string
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
	BPFFilter    string `json:"BPFFilter,omitempty" valid:"isBPFFilter"`
	Name         string `json:"Name,omitempty"`
	Description  string `json:"Description,omitempty"`
	Type         string `json:"Type,omitempty"`
	Count        int    `json:"Count,omitempty"`
	PCAPSocket   string `json:"PCAPSocket,omitempty"`
	Port         int    `json:"Port,omitempty"`
}

// CaptureResourceHandler describes a capture ressouce handler
type CaptureResourceHandler struct {
	ResourceHandler
}

// CaptureAPIHandler based on BasicAPIHandler
type CaptureAPIHandler struct {
	BasicAPIHandler
	Graph *graph.Graph
}

// NewCapture creates a new capture
func NewCapture(query string, bpfFilter string) *Capture {
	id, _ := uuid.NewV4()

	return &Capture{
		UUID:         id.String(),
		GremlinQuery: query,
		BPFFilter:    bpfFilter,
	}
}

// Name returns "capture"
func (c *CaptureResourceHandler) Name() string {
	return "capture"
}

// New creates a new capture resource
func (c *CaptureResourceHandler) New() Resource {
	id, _ := uuid.NewV4()

	return &Capture{
		UUID: id.String(),
	}
}

// Decorate populate the capture resource
func (c *CaptureAPIHandler) Decorate(resource Resource) {
	capture := resource.(*Capture)

	count := 0
	pcapSocket := ""

	c.Graph.RLock()
	defer c.Graph.RUnlock()

	res, err := topology.ExecuteGremlinQuery(c.Graph, capture.GremlinQuery)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin error: %s", err.Error())
		return
	}

	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			n := value.(*graph.Node)
			if tp, _ := n.GetFieldString("Type"); tp != "" && common.IsCaptureAllowed(tp) {
				count++
			}
			if p, _ := n.GetFieldString("PCAPSocket"); p != "" {
				pcapSocket = p
			}
		case []*graph.Node:
			count += len(value.([]*graph.Node))
		default:
			count = 0
		}
	}

	capture.Count = count
	capture.PCAPSocket = pcapSocket
}

// ID returns the capture Identifier
func (c *Capture) ID() string {
	return c.UUID
}

// SetID set a new identifier for this capture
func (c *Capture) SetID(i string) {
	c.UUID = i
}

// Create tests that resource GremlinQuery does not exists already
func (c *CaptureAPIHandler) Create(r Resource) error {
	capture := r.(*Capture)
	resources := c.BasicAPIHandler.Index()
	for _, resource := range resources {
		if resource.(*Capture).GremlinQuery == capture.GremlinQuery {
			return fmt.Errorf("Duplicate capture, uuid=%s", resource.(*Capture).UUID)
		}
	}

	return c.BasicAPIHandler.Create(r)
}

// RegisterCaptureAPI registers an new resource, capture
func RegisterCaptureAPI(apiServer *Server, g *graph.Graph) (*CaptureAPIHandler, error) {
	captureAPIHandler := &CaptureAPIHandler{
		BasicAPIHandler: BasicAPIHandler{
			ResourceHandler: &CaptureResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
		Graph: g,
	}
	if err := apiServer.RegisterAPIHandler(captureAPIHandler); err != nil {
		return nil, err
	}
	return captureAPIHandler, nil
}
