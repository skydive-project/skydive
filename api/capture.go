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
	"strings"

	etcd "github.com/coreos/etcd/client"
	"github.com/nu7hatch/gouuid"
	"golang.org/x/net/context"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

type Capture struct {
	UUID         string
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
	BPFFilter    string `json:"BPFFilter,omitempty"`
	Name         string `json:"Name,omitempty"`
	Description  string `json:"Description,omitempty"`
	Type         string `json:"Type,omitempty"`
	Count        int    `json:"Count,omitempty"`
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
		UUID:  id.String(),
		Count: 0,
	}
}

func (c *CaptureApiHandler) getCaptureCount(r ApiResource) ApiResource {
	capture := r.(*Capture)
	tr := traversal.NewGremlinTraversalParser(c.Graph)
	ts, err := tr.Parse(strings.NewReader(capture.GremlinQuery))
	if err != nil {
		logging.GetLogger().Errorf("Gremlin expression error: %s", err.Error())
		return r
	}

	res, err := ts.Exec()
	if err != nil {
		logging.GetLogger().Errorf("Gremlin execution error: %s", err.Error())
		return r
	}

	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			n := value.(*graph.Node)
			if t, ok := n.Metadata()["Type"]; ok && common.IsCaptureAllowed(t.(string)) {
				capture.Count = capture.Count + 1
			}
		case []*graph.Node:
			capture.Count = capture.Count + len(value.([]*graph.Node))
		default:
			capture.Count = 0
		}
	}
	return capture
}

func (c *CaptureApiHandler) Index() map[string]ApiResource {
	etcdPath := fmt.Sprintf("/%s/", c.ResourceHandler.Name())

	resp, err := c.EtcdKeyAPI.Get(context.Background(), etcdPath, &etcd.GetOptions{Recursive: true})
	resources := make(map[string]ApiResource)

	if err == nil {
		c.collectNodes(resources, resp.Node.Nodes)
	}

	if c.ResourceHandler.Name() == "capture" {
		mr := make(map[string]ApiResource)
		for _, resource := range resources {
			mr[resource.ID()] = c.getCaptureCount(resource)
		}
		return mr
	}
	return resources
}

// List returns the default list without any Count
// basically use by ondemand
func (c *CaptureApiHandler) List() map[string]ApiResource {
	return c.BasicApiHandler.Index()
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
	resources := c.BasicApiHandler.Index()
	for _, resource := range resources {
		if resource.(*Capture).GremlinQuery == r.(*Capture).GremlinQuery {
			return fmt.Errorf("Duplicate capture, uuid=%s", resource.(*Capture).UUID)
		}
	}
	return c.BasicApiHandler.Create(r)
}
