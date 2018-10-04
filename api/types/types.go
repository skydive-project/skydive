/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package types

import (
	"errors"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/graph"
)

var schemaValidator *topology.SchemaValidator

// Resource used as interface resources for each API
type Resource interface {
	ID() string
	SetID(string)
}

// BasicResource is a resource with a unique identifier
type BasicResource struct {
	UUID string `yaml:"UUID"`
}

// ID returns the alert ID
func (b *BasicResource) ID() string {
	return b.UUID
}

// SetID set ID
func (b *BasicResource) SetID(i string) {
	b.UUID = i
}

// Alert is a set of parameters, the Alert Action will Trigger according to its Expression.
type Alert struct {
	BasicResource
	Name        string `json:",omitempty"`
	Description string `json:",omitempty"`
	Expression  string `json:",omitempty" valid:"nonzero"`
	Action      string `json:",omitempty" valid:"regexp=^(|http://|https://|file://).*$"`
	Trigger     string `json:",omitempty" valid:"regexp=^(graph|duration:.+|)$"`
	CreateTime  time.Time
}

// NewAlert creates a New empty Alert, only UUID and CreateTime are set.
func NewAlert() *Alert {
	return &Alert{
		CreateTime: time.Now().UTC(),
	}
}

// Capture describes a capture API
type Capture struct {
	BasicResource
	GremlinQuery   string           `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
	BPFFilter      string           `json:"BPFFilter,omitempty" valid:"isBPFFilter"`
	Name           string           `json:"Name,omitempty"`
	Description    string           `json:"Description,omitempty"`
	Type           string           `json:"Type,omitempty"`
	Count          int              `json:"Count"`
	PCAPSocket     string           `json:"PCAPSocket,omitempty"`
	Port           int              `json:"Port,omitempty"`
	RawPacketLimit int              `json:"RawPacketLimit,omitempty" valid:"isValidRawPacketLimit"`
	HeaderSize     int              `json:"HeaderSize,omitempty" valid:"isValidCaptureHeaderSize"`
	ExtraTCPMetric bool             `json:"ExtraTCPMetric"`
	IPDefrag       bool             `json:"IPDefrag"`
	ReassembleTCP  bool             `json:"ReassembleTCP"`
	LayerKeyMode   string           `json:"LayerKeyMode,omitempty" valid:"isValidLayerKeyMode"`
	ExtraLayers    flow.ExtraLayers `json:"ExtraLayers,omitempty"`
}

// NewCapture creates a new capture
func NewCapture(query string, bpfFilter string) *Capture {
	return &Capture{
		GremlinQuery: query,
		BPFFilter:    bpfFilter,
	}
}

// EdgeRule describes a edge rule
type EdgeRule struct {
	UUID        string
	Name        string
	Description string
	Src         string `valid:"isGremlinExpr"`
	Dst         string `valid:"isGremlinExpr"`
	Metadata    graph.Metadata
}

// ID returns the edge rule ID
func (e *EdgeRule) ID() string {
	return e.UUID
}

// SetID set ID
func (e *EdgeRule) SetID(id string) {
	e.UUID = id
}

// Validate verifies the nodedgee rule does not create invalid edges
func (e *EdgeRule) Validate() error {
	n1 := graph.CreateNode(graph.GenID(), nil, time.Now(), "", common.UnknownService)
	n2 := graph.CreateNode(graph.GenID(), nil, time.Now(), "", common.UnknownService)
	edge := graph.CreateEdge(graph.GenID(), n1, n2, e.Metadata, time.Now(), "", common.UnknownService)
	return schemaValidator.ValidateEdge(edge)
}

// NodeRule describes a node rule
type NodeRule struct {
	UUID        string
	Name        string
	Description string
	Metadata    graph.Metadata
	Action      string `valid:"regexp=^(create|update)$"`
	Query       string `valid:"isGremlinOrEmpty"`
}

// ID returns the node rule ID
func (n *NodeRule) ID() string {
	return n.UUID
}

// SetID set ID
func (n *NodeRule) SetID(id string) {
	n.UUID = id
}

// Validate verifies the node rule does not create invalid node or change
// important attributes of an existing node
func (n *NodeRule) Validate() error {
	switch n.Action {
	case "create":
		// TODO: we should modify the JSON schema so that we can validate only the metadata
		node := graph.CreateNode(graph.GenID(), n.Metadata, time.Now(), "", common.UnknownService)
		return schemaValidator.ValidateNode(node)
	case "update":
		if n.Metadata["Type"] != nil || n.Metadata["Name"] != nil {
			return errors.New("Name and Type fields can not be changed")
		}
	}
	return nil
}

// PacketInjection packet injector API parameters
type PacketInjection struct {
	BasicResource
	Src        string
	Dst        string
	SrcIP      string
	DstIP      string
	SrcMAC     string
	DstMAC     string
	SrcPort    int64
	DstPort    int64
	Type       string
	Payload    string
	TrackingID string
	ICMPID     int64
	Count      int64
	Interval   int64
	Increment  bool
	StartTime  time.Time
}

// Validate verifies the packet injection type is supported
func (pi *PacketInjection) Validate() error {
	allowedTypes := map[string]bool{"icmp4": true, "icmp6": true, "tcp4": true, "tcp6": true, "udp4": true, "udp6": true}
	if _, ok := allowedTypes[pi.Type]; !ok {
		return errors.New("given type is not supported")
	}
	return nil
}

// TopologyParam topology API parameter
type TopologyParam struct {
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr"`
}

// WorkflowChoice describes one value within a choice
type WorkflowChoice struct {
	Value       string `yaml:"value"`
	Description string `yaml:"description"`
}

// WorkflowParam describes a workflow parameter
type WorkflowParam struct {
	Name        string
	Description string
	Type        string
	Default     interface{}
	Values      []WorkflowChoice
}

// Workflow describes a workflow
type Workflow struct {
	BasicResource `yaml:",inline"`
	Name          string          `yaml:"name" valid:"nonzero"`
	Description   string          `yaml:"description"`
	Parameters    []WorkflowParam `yaml:"parameters"`
	Source        string          `valid:"isValidWorkflow" yaml:"source"`
}

func init() {
	var err error
	if schemaValidator, err = topology.NewSchemaValidator(); err != nil {
		panic(err)
	}
}
