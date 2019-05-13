/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package types

import (
	"errors"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology"
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

// ID returns the resource ID
func (b *BasicResource) ID() string {
	return b.UUID
}

// SetID sets the resource ID
func (b *BasicResource) SetID(i string) {
	b.UUID = i
}

// Alert is a set of parameters, the Alert Action will Trigger according to its Expression.
type Alert struct {
	BasicResource `yaml:",inline"`
	Name          string `json:",omitempty" yaml:"Name"`
	Description   string `json:",omitempty" yaml:"Description"`
	Expression    string `json:",omitempty" valid:"nonzero" yaml:"Expression"`
	Action        string `json:",omitempty" valid:"regexp=^(|http://|https://|file://).*$" yaml:"Action"`
	Trigger       string `json:",omitempty" valid:"regexp=^(graph|duration:.+|)$" yaml:"Trigger"`
	CreateTime    time.Time
}

// NewAlert creates a New empty Alert, only CreateTime is set.
func NewAlert() *Alert {
	return &Alert{
		CreateTime: time.Now().UTC(),
	}
}

// Capture describes a capture API
type Capture struct {
	BasicResource   `yaml:",inline"`
	GremlinQuery    string           `json:"GremlinQuery,omitempty" valid:"isGremlinExpr" yaml:"GremlinQuery"`
	BPFFilter       string           `json:"BPFFilter,omitempty" valid:"isBPFFilter" yaml:"BPFFilter"`
	Name            string           `json:"Name,omitempty" yaml:"Name"`
	Description     string           `json:"Description,omitempty" yaml:"Description"`
	Type            string           `json:"Type,omitempty" valid:"isValidCaptureType" yaml:"Type"`
	Count           int              `json:"Count" yaml:"Count"`
	PCAPSocket      string           `json:"PCAPSocket,omitempty" yaml:"PCAPSocket"`
	Port            int              `json:"Port,omitempty" yaml:"Port"`
	SamplingRate    uint32           `json:"SamplingRate" yaml:"SamplingRate"`
	PollingInterval uint32           `json:"PollingInterval" yaml:"PollingInterval"`
	RawPacketLimit  int              `json:"RawPacketLimit,omitempty" valid:"isValidRawPacketLimit" yaml:"RawPacketLimit"`
	HeaderSize      int              `json:"HeaderSize,omitempty" valid:"isValidCaptureHeaderSize" yaml:"HeaderSize"`
	ExtraTCPMetric  bool             `json:"ExtraTCPMetric" yaml:"ExtraTCPMetric"`
	IPDefrag        bool             `json:"IPDefrag" yaml:"IPDefrag"`
	ReassembleTCP   bool             `json:"ReassembleTCP" yaml:"ReassembleTCP"`
	LayerKeyMode    string           `json:"LayerKeyMode,omitempty" valid:"isValidLayerKeyMode" yaml:"LayerKeyMode"`
	ExtraLayers     flow.ExtraLayers `json:"ExtraLayers,omitempty" yaml:"ExtraLayers"`
	Target          string           `json:"Target,omitempty" valid:"isValidAddress" yaml:"Target"`
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
	BasicResource `yaml:",inline"`
	Name          string         `yaml:"Name"`
	Description   string         `yaml:"Description"`
	Src           string         `valid:"isGremlinExpr" yaml:"Src"`
	Dst           string         `valid:"isGremlinExpr" yaml:"Dst"`
	Metadata      graph.Metadata `yaml:"Metadata"`
}

// Validate verifies the edge rule does not create invalid edges
func (e *EdgeRule) Validate() error {
	n1 := graph.CreateNode(graph.GenID(), nil, graph.TimeUTC(), "", common.UnknownService)
	n2 := graph.CreateNode(graph.GenID(), nil, graph.TimeUTC(), "", common.UnknownService)
	edge := graph.CreateEdge(graph.GenID(), n1, n2, e.Metadata, graph.TimeUTC(), "", common.UnknownService)
	return schemaValidator.ValidateEdge(edge)
}

// NodeRule describes a node rule
type NodeRule struct {
	BasicResource `yaml:",inline"`
	Name          string         `yaml:"Name"`
	Description   string         `yaml:"Description"`
	Metadata      graph.Metadata `yaml:"Metadata"`
	Action        string         `valid:"regexp=^(create|update)$" yaml:"Action"`
	Query         string         `valid:"isGremlinOrEmpty" yaml:"Query"`
}

// Validate verifies the node rule does not create invalid node or change
// important attributes of an existing node
func (n *NodeRule) Validate() error {
	switch n.Action {
	case "create":
		// TODO: we should modify the JSON schema so that we can validate only the metadata
		node := graph.CreateNode(graph.GenID(), n.Metadata, graph.TimeUTC(), "", common.UnknownService)
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
	BasicResource    `yaml:",inline"`
	Src              string `yaml:"Src"`
	Dst              string `yaml:"Dst"`
	SrcIP            string `valid:"isIPOrCIDR" yaml:"SrcIP"`
	DstIP            string `valid:"isIPOrCIDR" yaml:"DstIP"`
	SrcMAC           string `valid:"isMAC" yaml:"SrcMAC"`
	DstMAC           string `valid:"isMAC" yaml:"DstMAC"`
	SrcPort          uint16 `yaml:"SrcPort"`
	DstPort          uint16 `yaml:"DstPort"`
	Type             string `yaml:"Type"`
	Payload          string `yaml:"Payload"`
	TrackingID       string
	ICMPID           uint16 `yaml:"ICMPID"`
	Count            uint64 `yaml:"Count"`
	Interval         uint64 `yaml:"Interval"`
	Increment        bool   `yaml:"Increment"`
	IncrementPayload int64  `yaml:"IncrementPayload"`
	StartTime        time.Time
	Pcap             []byte `yaml:"Pcap"`
	TTL              uint8  `yaml:"TTL"`
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
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr" yaml:"GremlinQuery"`
}

// WorkflowChoice describes one value within a choice
type WorkflowChoice struct {
	Value       string `yaml:"Value"`
	Description string `yaml:"Description"`
}

// WorkflowParam describes a workflow parameter
type WorkflowParam struct {
	Name        string           `yaml:"Name"`
	Description string           `yaml:"Description"`
	Type        string           `yaml:"Type"`
	Default     interface{}      `yaml:"Default"`
	Values      []WorkflowChoice `yaml:"Values"`
}

// Workflow describes a workflow
type Workflow struct {
	BasicResource `yaml:",inline"`
	Name          string          `yaml:"Name" valid:"nonzero"`
	Description   string          `yaml:"Description"`
	Parameters    []WorkflowParam `yaml:"Parameters"`
	Source        string          `valid:"isValidWorkflow" yaml:"Source"`
}

// WorkflowCall describes workflow call
type WorkflowCall struct {
	Params []interface{}
}

func init() {
	var err error
	if schemaValidator, err = topology.NewSchemaValidator(); err != nil {
		panic(err)
	}
}
