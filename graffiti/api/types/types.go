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

package types

import (
	"time"

	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/graph"
)

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

// Edge object
// easyjson:json
// swagger:model
type Edge graph.Edge

// GetID returns the edge ID
func (e *Edge) GetID() string {
	return string(e.ID)
}

// SetID sets the node ID
func (e *Edge) SetID(i string) {
	e.ID = graph.Identifier(i)
}

// GetName returns the edge resource name
func (e *Edge) GetName() string {
	return "Edge"
}

// Validate integrity of the resource
func (e *Edge) Validate() error {
	return nil
}

// Node object
// swagger:model
type Node graph.Node

// UnmarshalJSON decodes types.Node using the custom graph.Node unmarshal which
// uses MetadataDecoders
func (n *Node) UnmarshalJSON(data []byte) error {
	gNode := graph.Node(*n)
	err := gNode.UnmarshalJSON(data)
	*n = Node(gNode)
	return err
}

// GetID returns the node ID
func (n *Node) GetID() string {
	return string(n.ID)
}

// SetID sets the resource ID
func (n *Node) SetID(i string) {
	n.ID = graph.Identifier(i)
}

// GetName returns the node resource name
func (n *Node) GetName() string {
	return "Node"
}

// Validate integrity of the resource
func (n *Node) Validate() error {
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
