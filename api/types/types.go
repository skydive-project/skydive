//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

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

// Package types defines the API resource types
//
// swagger:meta
package types

import (
	"errors"
	"time"

	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/schema"
)

// SchemaValidator validates resources against JSON schemas
var SchemaValidator *schema.Validator

// Alert object
//
// Alerts provide a way to be notified when a Gremlin expression
// is evaluated to true.
//
// easyjson:json
// swagger:model Alert
type Alert struct {
	// swagger:allOf
	rest.BasicResource `yaml:",inline"`
	// Alert name
	Name string `json:",omitempty" yaml:"Name"`
	// Alert description
	Description string `json:",omitempty" yaml:"Description"`
	// Gremlin or JavaScript expression evaluated to trigger the alarm
	Expression string `json:",omitempty" valid:"nonzero" yaml:"Expression"`
	// Action to execute when the alert is triggered.
	// Can be either an empty string, or a URL (use 'file://' for local scripts)
	Action string `json:",omitempty" valid:"regexp=^(|http://|https://|file://).*$" yaml:"Action"`
	// Event that triggers the alert evaluation
	Trigger    string `json:",omitempty" valid:"regexp=^(graph|duration:.+|)$" yaml:"Trigger"`
	CreateTime time.Time
}

// GetName returns the resource name
func (a *Alert) GetName() string {
	return "Alert"
}

// NewAlert creates a New empty Alert, only CreateTime is set.
func NewAlert() *Alert {
	return &Alert{
		CreateTime: time.Now().UTC(),
	}
}

// Capture object
//
// Captures provide a way to capture network traffic on the nodes
// matching a Gremlin expression.
//
// easyjson:json
// swagger:model Capture
type Capture struct {
	// swagger:allOf
	rest.BasicResource `yaml:",inline"`
	// Gremlin Query
	// required: true
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr" yaml:"GremlinQuery"`
	// BPF filter
	BPFFilter string `json:"BPFFilter,omitempty" valid:"isBPFFilter" yaml:"BPFFilter"`
	// Capture name
	Name string `json:"Name,omitempty" yaml:"Name"`
	// Capture description
	Description string `json:"Description,omitempty" yaml:"Description"`
	// Capture type. Can be afpacket, pcap, ebpf, sflow, pcapsocket, ovsmirror, dpdk, ovssflow or ovsnetflow
	Type string `json:"Type,omitempty" valid:"isValidCaptureType" yaml:"Type"`
	// Number of active captures
	// swagger:ignore
	Count int `json:"Count" yaml:"Count"`
	// SFlow port
	Port int `json:"Port,omitempty" yaml:"Port"`
	// Sampling rate for SFlow flows. 0: no flow samples
	SamplingRate uint32 `json:"SamplingRate" yaml:"SamplingRate"`
	// Polling interval for SFlow counters, 0: no counter samples
	PollingInterval uint32 `json:"PollingInterval" yaml:"PollingInterval"`
	// Maximum number of raw packets captured, 0: no packet, -1: unlimited
	RawPacketLimit int `json:"RawPacketLimit,omitempty" valid:"isValidRawPacketLimit" yaml:"RawPacketLimit"`
	// Packet header size to consider
	HeaderSize int `json:"HeaderSize,omitempty" valid:"isValidCaptureHeaderSize" yaml:"HeaderSize"`
	// Add additional TCP metrics to flows
	ExtraTCPMetric bool `json:"ExtraTCPMetric" yaml:"ExtraTCPMetric"`
	// Defragment IPv4 packets
	IPDefrag bool `json:"IPDefrag" yaml:"IPDefrag"`
	// Reassemble TCP packets
	ReassembleTCP bool `json:"ReassembleTCP" yaml:"ReassembleTCP"`
	// First layer used by flow key calculation, L2 or L3
	LayerKeyMode string `json:"LayerKeyMode,omitempty" valid:"isValidLayerKeyMode" yaml:"LayerKeyMode"`
	// List of extra layers to be added to the flow, available: DNS|DHCPv4|VRRP
	ExtraLayers int `json:"ExtraLayers,omitempty" yaml:"ExtraLayers"`
	// sFlow/NetFlow target, if empty the agent will be used
	Target string `json:"Target,omitempty" valid:"isValidAddress" yaml:"Target"`
	// target type (netflowv5, erspanv1), ignored in case of sFlow/NetFlow capture
	TargetType string `json:"TargetType,omitempty" yaml:"TargetType"`
}

// GetName returns the resource name
func (c *Capture) GetName() string {
	return "Capture"
}

// NewCapture creates a new capture
func NewCapture(query string, bpfFilter string) *Capture {
	return &Capture{
		GremlinQuery: query,
		BPFFilter:    bpfFilter,
	}
}

// EdgeRule object
//
// Edge rules allow the dynamic creation of links between nodes of the graph.
//
// easyjson:json
// swagger:model
type EdgeRule struct {
	// swagger:allOf
	rest.BasicResource `yaml:",inline"`
	// Edge rule name
	Name string `yaml:"Name"`
	// Edge rule description
	Description string `yaml:"Description"`
	// Gremlin expression of the edges source nodes
	Src string `valid:"isGremlinExpr" yaml:"Src"`
	// Gremlin expression of the edges destination nodes
	Dst string `valid:"isGremlinExpr" yaml:"Dst"`
	// Metadata of the edges to create
	Metadata graph.Metadata `yaml:"Metadata"`
}

// GetName returns the resource name
func (e *EdgeRule) GetName() string {
	return "EdgeRule"
}

// Validate verifies the edge rule does not create invalid edges
func (e *EdgeRule) Validate() error {
	n1 := graph.CreateNode(graph.GenID(), nil, graph.TimeUTC(), "", "")
	n2 := graph.CreateNode(graph.GenID(), nil, graph.TimeUTC(), "", "")
	edge := graph.CreateEdge(graph.GenID(), n1, n2, e.Metadata, graph.TimeUTC(), "", "")
	return SchemaValidator.Validate("edge", edge)
}

// NodeRule object
//
// Node rules allow the dynamic creation of nodes in the graph.
//
// easyjson:json
// swagger:model
type NodeRule struct {
	// swagger:allOf
	rest.BasicResource `yaml:",inline"`
	// Node rule name
	Name string `yaml:"Name"`
	// Node rule description
	Description string `yaml:"Description"`
	// Metadata of the nodes to create
	Metadata graph.Metadata `yaml:"Metadata"`
	// 'create' to create nodes, 'update' to updates nodes
	Action string `valid:"regexp=^(create|update)$" yaml:"Action"`
	// Gremlin expression of the nodes to update
	Query string `valid:"isGremlinOrEmpty" yaml:"Query"`
}

// GetName returns the resource name
func (n *NodeRule) GetName() string {
	return "NodeRule"
}

// Validate verifies the node rule does not create invalid node or change
// important attributes of an existing node
func (n *NodeRule) Validate() error {
	switch n.Action {
	case "create":
		// TODO: we should modify the JSON schema so that we can validate only the metadata
		node := graph.CreateNode(graph.GenID(), n.Metadata, graph.TimeUTC(), "", "")
		return SchemaValidator.Validate("node", node)
	case "update":
		if n.Metadata["Type"] != nil || n.Metadata["Name"] != nil {
			return errors.New("Name and Type fields can not be changed")
		}
	}
	return nil
}

const (
	// PIModeUniqPerNode use a unique packet identifier per source node
	PIModeUniqPerNode = "unique"
	// PIModeRandom use random packet identifier for each packet
	PIModeRandom = "random"

	// PITypeICMP4 icmpv4 packet
	PITypeICMP4 = "icmp4"
	// PITypeICMP6 icmpv6 packet
	PITypeICMP6 = "icmp6"
	// PITypeTCP4 ipv4 + tcp packet
	PITypeTCP4 = "tcp4"
	// PITypeTCP6 ipv6 + tcp packet
	PITypeTCP6 = "tcp6"
	// PITypeUDP4 ipv4 + udp packet
	PITypeUDP4 = "udp4"
	// PITypeUDP6 ipv6 + udp packet
	PITypeUDP6 = "udp6"
)

// PacketInjection packet injector API parameters
// easyjson:json
// swagger:model
type PacketInjection struct {
	// swagger:allOf
	rest.BasicResource `yaml:",inline"`
	Src                string `yaml:"Src"`
	Dst                string `yaml:"Dst"`
	SrcIP              string `valid:"isIPOrCIDR" yaml:"SrcIP"`
	DstIP              string `valid:"isIPOrCIDR" yaml:"DstIP"`
	SrcMAC             string `valid:"isMAC" yaml:"SrcMAC"`
	DstMAC             string `valid:"isMAC" yaml:"DstMAC"`
	SrcPort            uint16 `yaml:"SrcPort"`
	DstPort            uint16 `yaml:"DstPort"`
	Type               string `yaml:"Type"`
	Payload            string `yaml:"Payload"`
	ICMPID             uint16 `yaml:"ICMPID"`
	Count              uint64 `yaml:"Count"`
	Interval           uint64 `yaml:"Interval"`
	Mode               string `yaml:"Mode"`
	IncrementPayload   int64  `yaml:"IncrementPayload"`
	StartTime          time.Time
	Pcap               []byte `yaml:"Pcap"`
	TTL                uint8  `yaml:"TTL"`
}

// GetName returns the resource name
func (pi *PacketInjection) GetName() string {
	return "PacketInjection"
}

// Validate verifies the packet injection type is supported
func (pi *PacketInjection) Validate() error {
	allowedTypes := map[string]bool{
		PITypeICMP4: true,
		PITypeICMP6: true,
		PITypeTCP4:  true,
		PITypeTCP6:  true,
		PITypeUDP4:  true,
		PITypeUDP6:  true,
	}
	if _, ok := allowedTypes[pi.Type]; !ok {
		return errors.New("given type is not supported")
	}

	if pi.Mode != "" && pi.Mode != PIModeUniqPerNode && pi.Mode != PIModeRandom {
		return errors.New("given mode is not supported")
	}
	return nil
}

// TopologyParams topology query parameters
// easyjson:json
// swagger:model
type TopologyParams struct {
	GremlinQuery string `json:"GremlinQuery,omitempty" valid:"isGremlinExpr" yaml:"GremlinQuery"`
}

// WorkflowChoice describes one value within a choice
// easyjson:json
// swagger:model
type WorkflowChoice struct {
	Value       string `yaml:"Value"`
	Description string `yaml:"Description"`
}

// WorkflowParam describes a workflow parameter
// easyjson:json
// swagger:model
type WorkflowParam struct {
	Name        string           `yaml:"Name"`
	Description string           `yaml:"Description"`
	Type        string           `yaml:"Type"`
	Default     interface{}      `yaml:"Default"`
	Values      []WorkflowChoice `yaml:"Values"`
}

// Workflow object
//
// Workflow allows to automate actions using JavaScript.
//
// easyjson:json
// swagger:model
type Workflow struct {
	// swagger:allOf
	rest.BasicResource `yaml:",inline"`
	// Workflow name
	Name string `yaml:"Name" valid:"nonzero"`
	// Workflow title
	Title string `yaml:"Title"`
	// Workflow abstract
	Abstract string `yaml:"Abstract"`
	// Workflow description
	Description string `yaml:"Description"`
	// Workflow parameters
	Parameters []WorkflowParam `yaml:"Parameters"`
	Source     string          `valid:"isValidWorkflow" yaml:"Source"`
}

// WorkflowCall describes workflow call
// swagger:model
type WorkflowCall struct {
	Params []interface{}
}

func init() {
	SchemaValidator = schema.NewValidator()
}
