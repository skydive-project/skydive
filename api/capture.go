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

type Capture struct {
	UUID         string
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
	BPFFilter    string `json:"BPFFilter,omitempty"`
	Name         string `json:"Name,omitempty"`
	Description  string `json:"Description,omitempty"`
	Type         string `json:"Type,omitempty"`
	Count        int    `json:"Count,omitempty"`
	PCAPSocket   string `json:"PCAPSocket,omitempty"`
}

type CaptureResourceHandler struct {
}

type CaptureApiHandler struct {
	BasicApiHandler
	Graph *graph.Graph
}

func NewCapture(query string, bpfFilter string) *Capture {
	id, _ := uuid.NewV4()

	return &Capture{
		UUID:         id.String(),
		GremlinQuery: query,
		BPFFilter:    bpfFilter,
	}
}

func (c *CaptureResourceHandler) New() ApiResource {
	id, _ := uuid.NewV4()

	return &Capture{
		UUID: id.String(),
	}
}

func (c *CaptureApiHandler) Decorate(resource ApiResource) {
	capture := resource.(*Capture)

	count := 0
	pcapSocket := ""

	res, err := topology.ExecuteGremlinQuery(c.Graph, capture.GremlinQuery)
	if err != nil {
		logging.GetLogger().Errorf("Gremlin error: %s", err.Error())
		return
	}

	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			n := value.(*graph.Node)
			if t, ok := n.Metadata()["Type"]; ok && common.IsCaptureAllowed(t.(string)) {
				count++
			}
			if p, ok := n.Metadata()["PCAPSocket"]; ok {
				pcapSocket = p.(string)
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

func (c *CaptureResourceHandler) Name() string {
	return "capture"
}

func (c *Capture) ID() string {
	return c.UUID
}

func (c *Capture) SetID(i string) {
	c.UUID = i
}

// Create tests that resource GremlinQuery does not exists already
func (c *CaptureApiHandler) Create(r ApiResource) error {
	capture := r.(*Capture)
	resources := c.BasicApiHandler.Index()
	for _, resource := range resources {
		if resource.(*Capture).GremlinQuery == capture.GremlinQuery {
			return fmt.Errorf("Duplicate capture, uuid=%s", resource.(*Capture).UUID)
		}
	}

	return c.BasicApiHandler.Create(r)
}
